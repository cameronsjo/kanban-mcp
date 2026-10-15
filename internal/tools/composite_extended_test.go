package tools

// Test Plan for internal/tools/composite.go — functions not yet covered by composite_test.go
//
// cardDetails (Classification: I/O boundary — multi-resource fetch then board-walk)
//   [x] Happy/board-walk: card found, findBoardIDForList walks projects→boards→lists
//   [x] Unhappy/board-not-found: no board has the card's list → error "Could not determine board ID..."
//   [x] Output shape: result contains card, taskStats, comments, labels, analysis keys
//
// findBoardIDForList (Classification: I/O boundary — tested indirectly via cardDetails)
//   [x] Happy: projects.Included.Boards → GetLists → match → returns board ID
//   [x] Edge/no-boards: GetProjects returns empty included → returns "" → cardDetails errors
//
// boardSummary with includeTaskDetails=true
//   [x] tasks block per card with total/completed/completionPercentage
//
// boardSummary with includeComments=true
//   [x] comments slice present on each summaryCard

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestCardDetailsBoardWalkFindsBoard exercises the full cardDetails path:
// GetCard → parallel (GetTasks, GetComments) → findBoardIDForList
// (GetProjects → GetLists per board) → GetLabels.
func TestCardDetailsBoardWalkFindsBoard(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		switch {
		case r.Method == "GET" && r.Path == "/api/cards/C1":
			// Serves both GetCard and GetTasks (same endpoint).
			return 200, `{"item":{"id":"C1","listId":"L1","name":"My Card"},"included":{"tasks":[{"id":"T1","name":"task","isCompleted":false,"position":1,"createdAt":"2024-01-01T00:00:00Z"}]}}`
		case r.Method == "GET" && r.Path == "/api/cards/C1/comments":
			return 200, `{"items":[]}`
		case r.Method == "GET" && r.Path == "/api/projects":
			// findBoardIDForList calls GetProjects which hits /api/projects?...
			// The mock receives the decoded path without query string.
			return 200, `{"items":[],"included":{"boards":[{"id":"B1","projectId":"P1","name":"Board","position":1,"createdAt":"2024-01-01T00:00:00Z"}]}}`
		case r.Method == "GET" && r.Path == "/api/boards/B1":
			// Serves both GetLists (findBoardIDForList) and GetLabels.
			return 200, `{"item":{"id":"B1","name":"Board","projectId":"P1","position":1,"createdAt":"2024-01-01T00:00:00Z"},"included":{"lists":[{"id":"L1","boardId":"B1","name":"To Do","position":1,"createdAt":"2024-01-01T00:00:00Z"}],"labels":[{"id":"LB1","boardId":"B1","name":"Bug","color":"red","createdAt":"2024-01-01T00:00:00Z"}]}}`
		}
		return 404, `{"message":"unexpected ` + r.Method + " " + r.Path + `"}`
	}}
	c := newToolClient(t, m)

	result, err := cardDetails(context.Background(), c, "C1")
	if err != nil {
		t.Fatalf("cardDetails: %v", err)
	}
	out, _ := json.Marshal(result)
	s := string(out)
	for _, want := range []string{
		`"id":"C1"`,
		`"labels":`,
		`"id":"LB1"`,
		`"taskStats":`,
		`"total":1`,
		`"completed":0`,
		`"completionPercentage":0`,
		`"analysis":`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("result missing %q in: %s", want, s)
		}
	}
}

// TestCardDetailsBoardNotFoundErrors verifies that when findBoardIDForList
// cannot locate a board containing the card's list, cardDetails returns the
// "Could not determine board ID" error.
func TestCardDetailsBoardNotFoundErrors(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		switch {
		case r.Method == "GET" && r.Path == "/api/cards/C1":
			return 200, `{"item":{"id":"C1","listId":"UNKNOWN-LIST"}}`
		case r.Method == "GET" && r.Path == "/api/cards/C1/comments":
			return 200, `{"items":[]}`
		case r.Method == "GET" && r.Path == "/api/projects":
			// No boards in the included block → findBoardIDForList returns "".
			return 200, `{"items":[],"included":{"boards":[]}}`
		}
		return 404, `{"message":"unexpected"}`
	}}
	c := newToolClient(t, m)

	_, err := cardDetails(context.Background(), c, "C1")
	if err == nil {
		t.Fatal("expected error when board cannot be found for card")
	}
	if !strings.Contains(err.Error(), "Could not determine board ID") {
		t.Errorf("err = %q, want 'Could not determine board ID...'", err.Error())
	}
}

// TestCardDetailsAnalysisHasRecentHumanFeedback verifies the "hasRecentHumanFeedback"
// heuristic: a comment that is neither the bot phrases → true.
func TestCardDetailsAnalysisHasRecentHumanFeedback(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		switch {
		case r.Method == "GET" && r.Path == "/api/cards/C1":
			return 200, `{"item":{"id":"C1","listId":"L1"},"included":{}}`
		case r.Method == "GET" && r.Path == "/api/cards/C1/comments":
			return 200, `{"items":[{"id":"CM1","text":"Please fix the regression","createdAt":"2024-06-01T12:00:00Z"}]}`
		case r.Method == "GET" && r.Path == "/api/projects":
			return 200, `{"items":[],"included":{"boards":[{"id":"B1","projectId":"P1","name":"B","position":1,"createdAt":"2024-01-01T00:00:00Z"}]}}`
		case r.Method == "GET" && r.Path == "/api/boards/B1":
			return 200, `{"item":{"id":"B1","projectId":"P1","name":"B","position":1,"createdAt":"2024-01-01T00:00:00Z"},"included":{"lists":[{"id":"L1","boardId":"B1","name":"In Progress","position":1,"createdAt":"2024-01-01T00:00:00Z"}],"labels":[]}}`
		}
		return 404, `{"message":"unexpected"}`
	}}
	c := newToolClient(t, m)

	result, err := cardDetails(context.Background(), c, "C1")
	if err != nil {
		t.Fatalf("cardDetails: %v", err)
	}
	out, _ := json.Marshal(result)
	s := string(out)
	if !strings.Contains(s, `"hasRecentHumanFeedback":true`) {
		t.Errorf("expected hasRecentHumanFeedback:true in: %s", s)
	}
}

// TestBoardSummaryIncludeTaskDetailsComputesCompletion verifies that when
// includeTaskDetails=true, each card in the summary carries a tasks block with
// correct completionPercentage.
func TestBoardSummaryIncludeTaskDetailsComputesCompletion(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		switch {
		case r.Method == "GET" && r.Path == "/api/boards/7":
			return 200, `{"item":{"id":"7","name":"Board","projectId":"P1","position":1,"createdAt":"2024-01-01T00:00:00Z"},"included":{"lists":[{"id":"L1","boardId":"7","name":"In Progress","position":1,"createdAt":"2024-01-01T00:00:00Z"}],"labels":[]}}`
		case r.Method == "GET" && r.Path == "/api/projects":
			// GetCards board-walk.
			return 200, `{"items":[],"included":{"boards":[{"id":"B1","projectId":"P1","name":"B","position":1,"createdAt":"2024-01-01T00:00:00Z"}]}}`
		case r.Method == "GET" && r.Path == "/api/boards/B1":
			// GetCards fetches board detail to enumerate cards.
			return 200, `{"item":{"id":"B1","projectId":"P1","name":"B","position":1,"createdAt":"2024-01-01T00:00:00Z"},"included":{"cards":[{"id":"C1","listId":"L1","name":"Card","position":1,"createdAt":"2024-01-01T00:00:00Z"}]}}`
		case r.Method == "GET" && r.Path == "/api/cards/C1":
			// GetTasks reads the card's included.tasks.
			return 200, `{"item":{"id":"C1","listId":"L1","name":"Card","position":1,"createdAt":"2024-01-01T00:00:00Z"},"included":{"tasks":[{"id":"T1","isCompleted":true,"name":"a","position":1,"createdAt":"2024-01-01T00:00:00Z"},{"id":"T2","isCompleted":false,"name":"b","position":2,"createdAt":"2024-01-01T00:00:00Z"}]}}`
		}
		return 404, `{"message":"unexpected ` + r.Method + " " + r.Path + `"}`
	}}
	c := newToolClient(t, m)

	result, err := boardSummary(context.Background(), c, "7", true, false)
	if err != nil {
		t.Fatalf("boardSummary with task details: %v", err)
	}
	out, _ := json.Marshal(result)
	s := string(out)
	for _, want := range []string{
		`"tasks":`,
		`"total":2`,
		`"completed":1`,
		`"completionPercentage":50`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("result missing %q in: %s", want, s)
		}
	}
}

// TestBoardSummaryIncludeCommentsAttachesCommentsPerCard verifies that when
// includeComments=true the summary carries a comments slice on each card.
func TestBoardSummaryIncludeCommentsAttachesCommentsPerCard(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		switch {
		case r.Method == "GET" && r.Path == "/api/boards/7":
			return 200, `{"item":{"id":"7","name":"B","projectId":"P1","position":1,"createdAt":"2024-01-01T00:00:00Z"},"included":{"lists":[{"id":"L1","boardId":"7","name":"Backlog","position":1,"createdAt":"2024-01-01T00:00:00Z"}],"labels":[]}}`
		case r.Method == "GET" && r.Path == "/api/projects":
			return 200, `{"items":[],"included":{"boards":[{"id":"B1","projectId":"P1","name":"B","position":1,"createdAt":"2024-01-01T00:00:00Z"}]}}`
		case r.Method == "GET" && r.Path == "/api/boards/B1":
			return 200, `{"item":{"id":"B1","projectId":"P1","name":"B","position":1,"createdAt":"2024-01-01T00:00:00Z"},"included":{"cards":[{"id":"C1","listId":"L1","name":"Card","position":1,"createdAt":"2024-01-01T00:00:00Z"}]}}`
		case r.Method == "GET" && r.Path == "/api/cards/C1/comments":
			return 200, `{"items":[{"id":"CM1","text":"hello","createdAt":"2024-01-01T00:00:00Z"}]}`
		}
		return 404, `{"message":"unexpected ` + r.Method + " " + r.Path + `"}`
	}}
	c := newToolClient(t, m)

	result, err := boardSummary(context.Background(), c, "7", false, true)
	if err != nil {
		t.Fatalf("boardSummary with comments: %v", err)
	}
	out, _ := json.Marshal(result)
	s := string(out)
	if !strings.Contains(s, `"comments":`) {
		t.Errorf("result missing comments field in: %s", s)
	}
	if !strings.Contains(s, `"id":"CM1"`) {
		t.Errorf("result missing comment CM1 in: %s", s)
	}
}

// TestBoardSummaryIncludeComments404TreatedAsEmpty verifies that a 404 from
// GetComments is treated as "no comments" by commentsOrEmpty rather than an error.
func TestBoardSummaryIncludeComments404TreatedAsEmpty(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		switch {
		case r.Method == "GET" && r.Path == "/api/boards/7":
			return 200, `{"item":{"id":"7","name":"B","projectId":"P1","position":1,"createdAt":"2024-01-01T00:00:00Z"},"included":{"lists":[{"id":"L1","boardId":"7","name":"Backlog","position":1,"createdAt":"2024-01-01T00:00:00Z"}],"labels":[]}}`
		case r.Method == "GET" && r.Path == "/api/projects":
			return 200, `{"items":[],"included":{"boards":[{"id":"B1","projectId":"P1","name":"B","position":1,"createdAt":"2024-01-01T00:00:00Z"}]}}`
		case r.Method == "GET" && r.Path == "/api/boards/B1":
			return 200, `{"item":{"id":"B1","projectId":"P1","name":"B","position":1,"createdAt":"2024-01-01T00:00:00Z"},"included":{"cards":[{"id":"C1","listId":"L1","name":"Card","position":1,"createdAt":"2024-01-01T00:00:00Z"}]}}`
		case r.Method == "GET" && r.Path == "/api/cards/C1/comments":
			// 404 must be treated as "no comments" per commentsOrEmpty.
			return 404, `{"message":"not found"}`
		}
		return 404, `{"message":"unexpected"}`
	}}
	c := newToolClient(t, m)

	result, err := boardSummary(context.Background(), c, "7", false, true)
	if err != nil {
		t.Fatalf("boardSummary with 404 comments: %v", err)
	}
	out, _ := json.Marshal(result)
	s := string(out)
	// comments should be present as an empty array, not missing.
	if !strings.Contains(s, `"comments":[]`) {
		t.Errorf("expected 'comments':[] (404 treated as empty) in: %s", s)
	}
}
