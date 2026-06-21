package tools

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/cameronsjo/kanban-mcp/internal/planka"
)

// This file holds the composite tools (board_summary, card_details,
// create_card_with_tasks), porting tools/board-summary.ts, tools/card-details.ts
// and tools/create-card-with-tasks.ts. Independent fetches are parallelized with
// errgroup; a 404 on comments is treated as "none" (errors.As / IsNotFound).
//
// Output shapes mirror the TS by embedding planka.Card / planka.List so their
// fields flatten to the top level the way the TS object-spread (`...card`) did.

// CreateCardWithTasksParams ports the create_card_with_tasks input.
type CreateCardWithTasksParams struct {
	ListID      string
	Name        string
	Description *string
	Tasks       []string
	Comment     *string
	Position    *float64
}

// --- board_summary -----------------------------------------------------------

type taskSummary struct {
	Items                []planka.Task `json:"items"`
	Total                int           `json:"total"`
	Completed            int           `json:"completed"`
	CompletionPercentage int           `json:"completionPercentage"`
}

type summaryCard struct {
	planka.Card
	// Pointers so "requested but empty" renders ({}/[]) while "not requested"
	// is omitted — matching the TS `includeX ? value : undefined`.
	Tasks    *taskSummary      `json:"tasks,omitempty"`
	Comments *[]planka.Comment `json:"comments,omitempty"`
}

type summaryList struct {
	planka.List
	Cards     []summaryCard `json:"cards"`
	CardCount int           `json:"cardCount"`
}

func boardSummary(ctx context.Context, client *planka.Client, boardID string, includeTaskDetails, includeComments bool) (any, error) {
	board, err := client.GetBoard(ctx, boardID)
	if err != nil {
		return nil, err
	}
	if board == nil {
		return nil, fmt.Errorf("Board with ID %s not found", boardID)
	}

	lists, err := client.GetLists(ctx, boardID)
	if err != nil {
		return nil, err
	}

	summaryLists := make([]summaryList, len(lists))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(8)
	for i := range lists {
		i := i
		lst := lists[i]
		g.Go(func() error {
			cards, err := client.GetCards(gctx, lst.ID)
			if err != nil {
				return err
			}
			sc := make([]summaryCard, len(cards))
			for j, card := range cards {
				entry := summaryCard{Card: card}
				if includeTaskDetails {
					tasks, err := client.GetTasks(gctx, card.ID)
					if err != nil {
						return err
					}
					completed := countCompletedTasks(tasks)
					entry.Tasks = &taskSummary{
						Items:                tasks,
						Total:                len(tasks),
						Completed:            completed,
						CompletionPercentage: percent(completed, len(tasks)),
					}
				}
				if includeComments {
					comments, err := commentsOrEmpty(gctx, client, card.ID)
					if err != nil {
						return err
					}
					entry.Comments = &comments
				}
				sc[j] = entry
			}
			summaryLists[i] = summaryList{List: lst, Cards: sc, CardCount: len(sc)}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	labels, err := client.GetLabels(ctx, boardID)
	if err != nil {
		return nil, err
	}

	totalCards := 0
	for _, l := range summaryLists {
		totalCards += l.CardCount
	}
	backlog := findListByName(summaryLists, "backlog")
	inProgress := findListByName(summaryLists, "in progress")
	testing := findListByName(summaryLists, "testing")
	done := findListByName(summaryLists, "done")

	urgentCount := countCardsWithLabelNamed(summaryLists, labels, "urgent")
	bugCount := countCardsWithLabelNamed(summaryLists, labels, "bug")

	doneCount := cardCountOf(done)
	backlogCount := cardCountOf(backlog)
	inProgressCount := cardCountOf(inProgress)
	testingCount := cardCountOf(testing)

	return map[string]any{
		"board":  board,
		"lists":  summaryLists,
		"labels": labels,
		"stats": map[string]any{
			"totalCards":           totalCards,
			"backlogCount":         backlogCount,
			"inProgressCount":      inProgressCount,
			"testingCount":         testingCount,
			"doneCount":            doneCount,
			"urgentCount":          urgentCount,
			"bugCount":             bugCount,
			"completionPercentage": percent(doneCount, totalCards),
		},
		"workflowState": map[string]any{
			"hasCardsInBacklog":    backlogCount > 0,
			"hasCardsInProgress":   inProgressCount > 0,
			"hasCardsInTesting":    testingCount > 0,
			"nextActionSuggestion": nextActionSuggestion(backlogCount, inProgressCount, testingCount),
		},
	}, nil
}

func findListByName(lists []summaryList, name string) *summaryList {
	for i := range lists {
		if strings.ToLower(lists[i].List.Name) == name {
			return &lists[i]
		}
	}
	return nil
}

func cardCountOf(l *summaryList) int {
	if l == nil {
		return 0
	}
	return l.CardCount
}

func countCardsWithLabelNamed(lists []summaryList, labels []planka.Label, labelName string) int {
	matching := map[string]bool{}
	for _, lb := range labels {
		if strings.ToLower(lb.Name) == labelName {
			matching[lb.ID] = true
		}
	}
	if len(matching) == 0 {
		return 0
	}
	count := 0
	for _, l := range lists {
		for _, c := range l.Cards {
			for _, lid := range c.Card.LabelIDs {
				if matching[lid] {
					count++
					break
				}
			}
		}
	}
	return count
}

func nextActionSuggestion(backlog, inProgress, testing int) string {
	switch {
	case testing > 0:
		return "Review cards in Testing that need feedback"
	case inProgress > 0:
		return "Continue working on cards in In Progress"
	case backlog > 0:
		return "Start working on a card from Backlog"
	default:
		return "All tasks complete! Create new cards or projects"
	}
}

// --- card_details ------------------------------------------------------------

func cardDetails(ctx context.Context, client *planka.Client, cardID string) (any, error) {
	card, err := client.GetCard(ctx, cardID)
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, fmt.Errorf("Card with ID %s not found", cardID)
	}

	var tasks []planka.Task
	var comments []planka.Comment
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		t, err := client.GetTasks(gctx, card.ID)
		tasks = t
		return err
	})
	g.Go(func() error {
		c, err := commentsOrEmpty(gctx, client, card.ID)
		comments = c
		return err
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Locate the owning board by walking projects → boards → lists.
	boardID, err := findBoardIDForList(ctx, client, card.ListID)
	if err != nil {
		return nil, err
	}
	if boardID == "" {
		return nil, fmt.Errorf("Could not determine board ID for card %s", cardID)
	}
	labels, err := client.GetLabels(ctx, boardID)
	if err != nil {
		return nil, err
	}

	completed := countCompletedTasks(tasks)
	// Sort comments newest first.
	sort.SliceStable(comments, func(i, j int) bool {
		return parseTime(comments[i].CreatedAt).After(parseTime(comments[j].CreatedAt))
	})

	latestText := ""
	if len(comments) > 0 {
		latestText = comments[0].Text
	}
	hasRecentHumanFeedback := len(comments) > 0 && latestText != "" &&
		!strings.Contains(latestText, "Implemented feature") &&
		!strings.Contains(latestText, "Awaiting human review")

	completionPct := percent(completed, len(tasks))
	return map[string]any{
		"card":      card,
		"taskItems": tasks,
		"taskStats": map[string]any{
			"total":                len(tasks),
			"completed":            completed,
			"completionPercentage": completionPct,
		},
		"comments": comments,
		"labels":   labels,
		"analysis": map[string]any{
			"hasRecentHumanFeedback": hasRecentHumanFeedback,
			"isComplete":             completionPct == 100,
			"needsAttention":         hasRecentHumanFeedback || completed == 0,
		},
	}, nil
}

// findBoardIDForList walks projects → boards → lists and returns the board whose
// lists include listID, or "" if none match.
func findBoardIDForList(ctx context.Context, client *planka.Client, listID string) (string, error) {
	page, err := client.GetProjects(ctx, 1, 100)
	if err != nil {
		return "", err
	}
	for _, project := range page.Items {
		boards, err := client.GetBoards(ctx, project.ID)
		if err != nil {
			return "", err
		}
		for _, board := range boards {
			lists, err := client.GetLists(ctx, board.ID)
			if err != nil {
				return "", err
			}
			for _, l := range lists {
				if l.ID == listID {
					return board.ID, nil
				}
			}
		}
	}
	return "", nil
}

// --- create_card_with_tasks --------------------------------------------------

func createCardWithTasks(ctx context.Context, client *planka.Client, params CreateCardWithTasksParams) (any, error) {
	position := 65535.0
	if params.Position != nil {
		position = *params.Position
	}
	description := ""
	if params.Description != nil {
		description = *params.Description
	}

	// Best-effort, no transaction: the card is created first, then tasks, then
	// the comment. A failure mid-way leaves partial state and surfaces the
	// error (matching create-card-with-tasks.ts, which re-throws).
	card, err := client.CreateCard(ctx, planka.CreateCardOptions{
		ListID:      params.ListID,
		Name:        params.Name,
		Description: description,
		Position:    position,
	})
	if err != nil {
		return nil, err
	}

	createdTasks := []planka.Task{}
	for i, name := range params.Tasks {
		task, err := client.CreateTask(ctx, card.ID, name, float64(65535*(i+1)))
		if err != nil {
			return nil, err
		}
		createdTasks = append(createdTasks, *task)
	}

	var createdComment *planka.Comment
	if params.Comment != nil && *params.Comment != "" {
		c, err := client.CreateComment(ctx, card.ID, *params.Comment)
		if err != nil {
			return nil, err
		}
		createdComment = c
	}

	return map[string]any{
		"card":    card,
		"tasks":   createdTasks,
		"comment": createdComment,
	}, nil
}

// --- shared helpers ----------------------------------------------------------

func countCompletedTasks(tasks []planka.Task) int {
	n := 0
	for _, t := range tasks {
		if t.IsCompleted {
			n++
		}
	}
	return n
}

// percent returns round(completed/total*100), or 0 when total is 0.
func percent(completed, total int) int {
	if total <= 0 {
		return 0
	}
	return int(math.Round(float64(completed) / float64(total) * 100))
}

// commentsOrEmpty fetches a card's comments, treating a 404 as "no comments".
func commentsOrEmpty(ctx context.Context, client *planka.Client, cardID string) ([]planka.Comment, error) {
	comments, err := client.GetComments(ctx, cardID)
	if err != nil {
		if planka.IsNotFound(err) {
			return []planka.Comment{}, nil
		}
		return nil, err
	}
	return comments, nil
}

// parseTime parses an RFC3339 timestamp, returning the zero time on failure so
// unparseable/missing timestamps sort last.
func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
