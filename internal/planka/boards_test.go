package planka

// Test Plan for internal/planka/boards.go
//
// BoardDetail (Classification: I/O boundary — GET /api/boards/{id})
//   [x] Happy: returns board + populated included block
//   [x] Unhappy: 404 → propagates IsNotFound error
//
// GetBoards (Classification: I/O boundary — GET /api/projects; SWALLOW errors)
//   [x] Happy: filters included.Boards to those matching projectID
//   [x] Happy/empty-included: response has no included block → []Board{}
//   [x] Unhappy/swallow: API error → returns []Board{}, nil (SWALLOW contract)
//
// CreateBoard / UpdateBoard / DeleteBoard — thin CRUD wrappers covered by tools/projectboard_test.go
//   Skipped: happy/error paths covered there via newToolClient round-trips.

import (
	"context"
	"testing"
)

func TestBoardDetailReturnsItemAndIncluded(t *testing.T) {
	m := &mockPlanka{
		tokens: []string{"tok"},
		handler: func(token string, r recordedRequest) (int, string) {
			if r.Method == "GET" && r.Path == "/api/boards/5" {
				return 200, `{"item":{"id":"5","name":"MyBoard","projectId":"P1"},"included":{"lists":[{"id":"L1","boardId":"5","name":"To Do","position":65535,"createdAt":"2024-01-01T00:00:00Z"}]}}`
			}
			return 404, `{"message":"unexpected"}`
		},
	}
	c, _ := newTestClient(t, m)

	board, included, err := c.BoardDetail(context.Background(), "5")
	if err != nil {
		t.Fatalf("BoardDetail: %v", err)
	}
	if board == nil || board.ID != "5" || board.Name != "MyBoard" {
		t.Errorf("board = %+v", board)
	}
	if included == nil {
		t.Fatal("included = nil, want non-nil")
	}
	if len(included.Lists) != 1 || included.Lists[0].ID != "L1" {
		t.Errorf("included.Lists = %+v, want [{L1}]", included.Lists)
	}
}

func TestBoardDetailPropagatesError(t *testing.T) {
	m := &mockPlanka{
		tokens: []string{"tok"},
		handler: func(token string, r recordedRequest) (int, string) {
			return 404, `{"message":"board not found"}`
		},
	}
	c, _ := newTestClient(t, m)

	board, included, err := c.BoardDetail(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	if !IsNotFound(err) {
		t.Errorf("err = %v, want IsNotFound", err)
	}
	if board != nil || included != nil {
		t.Errorf("expected nil board and included on error")
	}
}

func TestGetBoardsFiltersIncludedByProjectID(t *testing.T) {
	m := &mockPlanka{
		tokens: []string{"tok"},
		handler: func(token string, r recordedRequest) (int, string) {
			if r.Path == "/api/projects" {
				return 200, `{"items":[],"included":{"boards":[
					{"id":"B1","projectId":"P1","name":"Board1","position":1,"createdAt":"2024-01-01T00:00:00Z"},
					{"id":"B2","projectId":"P2","name":"Board2","position":2,"createdAt":"2024-01-01T00:00:00Z"},
					{"id":"B3","projectId":"P1","name":"Board3","position":3,"createdAt":"2024-01-01T00:00:00Z"}
				]}}`
			}
			return 404, `{"message":"unexpected"}`
		},
	}
	c, _ := newTestClient(t, m)

	boards, err := c.GetBoards(context.Background(), "P1")
	if err != nil {
		t.Fatalf("GetBoards: %v", err)
	}
	if len(boards) != 2 {
		t.Fatalf("len(boards) = %d, want 2 (B1 and B3 belong to P1)", len(boards))
	}
	for _, b := range boards {
		if b.ProjectID != "P1" {
			t.Errorf("board %q has projectId %q, want P1", b.ID, b.ProjectID)
		}
	}
}

func TestGetBoardsSwallowsAPIError(t *testing.T) {
	// SWALLOW contract: a request failure must return []Board{}, nil (not an error).
	m := &mockPlanka{
		tokens: []string{"tok"},
		handler: func(token string, r recordedRequest) (int, string) {
			return 500, `{"message":"server error"}`
		},
	}
	c, _ := newTestClient(t, m)

	boards, err := c.GetBoards(context.Background(), "P1")
	if err != nil {
		t.Fatalf("GetBoards should swallow errors, got: %v", err)
	}
	if len(boards) != 0 {
		t.Errorf("len(boards) = %d, want 0", len(boards))
	}
}

func TestGetBoardsEmptyIncludedReturnsEmpty(t *testing.T) {
	// Response with no included block → []Board{}.
	m := &mockPlanka{
		tokens: []string{"tok"},
		handler: func(token string, r recordedRequest) (int, string) {
			return 200, `{"items":[]}`
		},
	}
	c, _ := newTestClient(t, m)

	boards, err := c.GetBoards(context.Background(), "P1")
	if err != nil {
		t.Fatalf("GetBoards: %v", err)
	}
	if len(boards) != 0 {
		t.Errorf("len(boards) = %d, want 0 (no included block)", len(boards))
	}
}

func TestGetBoardsProjectIDNotInIncluded(t *testing.T) {
	// Boards exist in included, but none belong to the requested projectID.
	m := &mockPlanka{
		tokens: []string{"tok"},
		handler: func(token string, r recordedRequest) (int, string) {
			return 200, `{"items":[],"included":{"boards":[
				{"id":"B1","projectId":"P2","name":"Board1","position":1,"createdAt":"2024-01-01T00:00:00Z"}
			]}}`
		},
	}
	c, _ := newTestClient(t, m)

	boards, err := c.GetBoards(context.Background(), "P1")
	if err != nil {
		t.Fatalf("GetBoards: %v", err)
	}
	if len(boards) != 0 {
		t.Errorf("len(boards) = %d, want 0 (no boards for P1)", len(boards))
	}
}
