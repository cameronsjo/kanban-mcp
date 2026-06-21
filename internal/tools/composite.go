package tools

import (
	"context"
	"fmt"

	"github.com/cameronsjo/kanban-mcp/internal/planka"
)

// This file holds the composite tools (board_summary, card_details,
// create_card_with_tasks). The real implementations land in step 4; the
// signatures below are the contract the project_board and card managers call,
// stubbed so the package builds during the resource fan-out.

// CreateCardWithTasksParams ports the create_card_with_tasks input.
type CreateCardWithTasksParams struct {
	ListID      string
	Name        string
	Description *string
	Tasks       []string
	Comment     *string
	Position    *float64
}

// boardSummary aggregates a board's lists, cards, tasks, labels, stats and
// workflow state (ports tools/board-summary.ts). Called by the project_board
// manager's get_board_summary action.
func boardSummary(ctx context.Context, client *planka.Client, boardID string, includeTaskDetails, includeComments bool) (any, error) {
	return nil, fmt.Errorf("board_summary not yet implemented")
}

// cardDetails aggregates a card's tasks, comments, labels and analysis (ports
// tools/card-details.ts). Called by the card manager's get_details action.
func cardDetails(ctx context.Context, client *planka.Client, cardID string) (any, error) {
	return nil, fmt.Errorf("card_details not yet implemented")
}

// createCardWithTasks creates a card plus its tasks and optional comment in one
// best-effort operation (ports tools/create-card-with-tasks.ts). Called by the
// card manager's create_with_tasks action.
func createCardWithTasks(ctx context.Context, client *planka.Client, params CreateCardWithTasksParams) (any, error) {
	return nil, fmt.Errorf("create_card_with_tasks not yet implemented")
}
