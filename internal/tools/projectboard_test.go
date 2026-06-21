package tools

import (
	"context"
	"strings"
	"testing"
)

// TestGetProjectsRequiresPageAndPerPage mirrors the index.ts guard:
// "page and perPage are required for get_projects action".
func TestGetProjectsRequiresPageAndPerPage(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})

	_, err := dispatchProjectBoard(context.Background(), c, projectBoardArgs{
		Action: "get_projects", Page: fp(1), // missing PerPage
	})
	if err == nil || !strings.Contains(err.Error(), "perPage") {
		t.Fatalf("err = %v, want perPage-required error", err)
	}

	_, err = dispatchProjectBoard(context.Background(), c, projectBoardArgs{
		Action: "get_projects", PerPage: fp(30), // missing Page
	})
	if err == nil || !strings.Contains(err.Error(), "page") {
		t.Fatalf("err = %v, want page-required error", err)
	}
}

// TestGetProjectsHappyPath verifies method, path, and query string.
func TestGetProjectsHappyPath(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		if r.Method == "GET" && strings.Contains(r.RawURI, "/api/projects") {
			return 200, `{"items":[{"id":"1","name":"Proj","createdAt":"2024-01-01"}]}`
		}
		return 404, `{}`
	}}
	c := newToolClient(t, m)

	res, err := dispatchProjectBoard(context.Background(), c, projectBoardArgs{
		Action: "get_projects", Page: fp(1), PerPage: fp(30),
	})
	if err != nil {
		t.Fatalf("get_projects: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	reqs := m.reqs()
	if len(reqs) == 0 {
		t.Fatal("no requests captured")
	}
	got := reqs[0]
	if got.Method != "GET" {
		t.Errorf("method = %q, want GET", got.Method)
	}
	if !strings.Contains(got.RawURI, "page=1") {
		t.Errorf("path %q missing page=1", got.RawURI)
	}
	if !strings.Contains(got.RawURI, "per_page=30") {
		t.Errorf("path %q missing per_page=30", got.RawURI)
	}
}

// TestGetProjectRequiresNumericID verifies Layer-B requireID rejects non-numeric.
func TestGetProjectRequiresNumericID(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchProjectBoard(context.Background(), c, projectBoardArgs{
		Action: "get_project", ID: sp("not-a-number"),
	})
	if err == nil || !strings.Contains(err.Error(), "numeric Planka ID") {
		t.Fatalf("err = %v, want numeric-ID rejection", err)
	}
}

// TestGetBoardsHappyPath verifies that GetBoards calls GET /api/projects and
// filters boards by projectId.
func TestGetBoardsHappyPath(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		if r.Method == "GET" && r.Path == "/api/projects" {
			return 200, `{"items":[],"included":{"boards":[{"id":"10","projectId":"42","name":"Board","position":65535,"createdAt":"2024-01-01"}]}}`
		}
		return 404, `{}`
	}}
	c := newToolClient(t, m)

	res, err := dispatchProjectBoard(context.Background(), c, projectBoardArgs{
		Action: "get_boards", ProjectID: sp("42"),
	})
	if err != nil {
		t.Fatalf("get_boards: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	reqs := m.reqs()
	if len(reqs) == 0 || reqs[0].Path != "/api/projects" {
		t.Errorf("expected GET /api/projects, got %+v", reqs)
	}
}

// TestGetBoardsSwallowsError mirrors boards.ts getBoards: request errors yield [].
func TestGetBoardsSwallowsError(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		return 500, `{"message":"boom"}`
	}}
	c := newToolClient(t, m)

	res, err := dispatchProjectBoard(context.Background(), c, projectBoardArgs{
		Action: "get_boards", ProjectID: sp("42"),
	})
	if err != nil {
		t.Fatalf("get_boards should swallow error, got: %v", err)
	}
	_ = res // empty slice
}

// TestCreateBoardHappyPath verifies POST path, request body, and that best-effort
// side effects (admin membership, lists, labels) do not cause the action to fail.
func TestCreateBoardHappyPath(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		switch {
		case r.Method == "GET" && r.Path == "/api/users":
			// AdminUserID lookup — return empty list so it's skipped cleanly.
			return 200, `{"items":[]}`
		case r.Method == "POST" && r.Path == "/api/projects/42/boards":
			return 200, `{"item":{"id":"10","projectId":"42","name":"My Board","position":65535,"createdAt":"2024-01-01"}}`
		default:
			// All best-effort calls (membership, list, label creation) — succeed silently.
			return 200, `{"item":{}}`
		}
	}}
	c := newToolClient(t, m)

	res, err := dispatchProjectBoard(context.Background(), c, projectBoardArgs{
		Action:    "create_board",
		ProjectID: sp("42"),
		Name:      sp("My Board"),
		Position:  fp(65535),
	})
	if err != nil {
		t.Fatalf("create_board: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	// Find the board creation request.
	var boardReq *capturedRequest
	for _, r := range m.reqs() {
		if r.Method == "POST" && r.Path == "/api/projects/42/boards" {
			rc := r
			boardReq = &rc
			break
		}
	}
	if boardReq == nil {
		t.Fatal("did not find POST /api/projects/42/boards")
	}
	for _, want := range []string{`"name":"My Board"`, `"position":65535`} {
		if !strings.Contains(boardReq.Body, want) {
			t.Errorf("create_board body %q missing %q", boardReq.Body, want)
		}
	}
}

// TestCreateBoardMissingPositionErrors verifies the position-required guard.
func TestCreateBoardMissingPositionErrors(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchProjectBoard(context.Background(), c, projectBoardArgs{
		Action: "create_board", ProjectID: sp("42"), Name: sp("Board"),
	})
	if err == nil || !strings.Contains(err.Error(), "position") {
		t.Fatalf("err = %v, want position-required error", err)
	}
}

// TestGetBoardHappyPath verifies GET /api/boards/{id}.
func TestGetBoardHappyPath(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		if r.Method == "GET" && r.Path == "/api/boards/10" {
			return 200, `{"item":{"id":"10","projectId":"42","name":"Board","position":65535,"createdAt":"2024-01-01"}}`
		}
		return 404, `{}`
	}}
	c := newToolClient(t, m)

	res, err := dispatchProjectBoard(context.Background(), c, projectBoardArgs{
		Action: "get_board", ID: sp("10"),
	})
	if err != nil {
		t.Fatalf("get_board: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	got := m.reqs()[0]
	if got.Method != "GET" || got.Path != "/api/boards/10" {
		t.Errorf("want GET /api/boards/10, got %s %s", got.Method, got.Path)
	}
}

// TestUpdateBoardHappyPath verifies PATCH /api/boards/{id} with name+position,
// and that type is included when set.
func TestUpdateBoardHappyPath(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		if r.Method == "PATCH" && r.Path == "/api/boards/10" {
			return 200, `{"item":{"id":"10","projectId":"42","name":"Renamed","position":131070,"createdAt":"2024-01-01"}}`
		}
		return 404, `{}`
	}}
	c := newToolClient(t, m)

	res, err := dispatchProjectBoard(context.Background(), c, projectBoardArgs{
		Action:   "update_board",
		ID:       sp("10"),
		Name:     sp("Renamed"),
		Position: fp(131070),
		Type:     sp("kanban"),
	})
	if err != nil {
		t.Fatalf("update_board: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	got := m.reqs()[0]
	if got.Method != "PATCH" || got.Path != "/api/boards/10" {
		t.Errorf("want PATCH /api/boards/10, got %s %s", got.Method, got.Path)
	}
	for _, want := range []string{`"name":"Renamed"`, `"position":131070`, `"type":"kanban"`} {
		if !strings.Contains(got.Body, want) {
			t.Errorf("update_board body %q missing %q", got.Body, want)
		}
	}
}

// TestUpdateBoardRequiresNameAndPosition verifies both guards.
func TestUpdateBoardRequiresNameAndPosition(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})

	_, err := dispatchProjectBoard(context.Background(), c, projectBoardArgs{
		Action: "update_board", ID: sp("10"), Position: fp(1), // missing name
	})
	if err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("err = %v, want name-required error", err)
	}

	_, err = dispatchProjectBoard(context.Background(), c, projectBoardArgs{
		Action: "update_board", ID: sp("10"), Name: sp("X"), // missing position
	})
	if err == nil || !strings.Contains(err.Error(), "position") {
		t.Fatalf("err = %v, want position-required error", err)
	}
}

// TestDeleteBoardHappyPath verifies DELETE /api/boards/{id} and success shape.
func TestDeleteBoardHappyPath(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		return 200, `{}`
	}}
	c := newToolClient(t, m)

	res, err := dispatchProjectBoard(context.Background(), c, projectBoardArgs{
		Action: "delete_board", ID: sp("10"),
	})
	if err != nil {
		t.Fatalf("delete_board: %v", err)
	}
	got := m.reqs()[0]
	if got.Method != "DELETE" || got.Path != "/api/boards/10" {
		t.Errorf("want DELETE /api/boards/10, got %s %s", got.Method, got.Path)
	}
	_ = res
}

// TestGetBoardSummaryRequiresBoardID verifies requireString("boardId") fires.
func TestGetBoardSummaryRequiresBoardID(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchProjectBoard(context.Background(), c, projectBoardArgs{
		Action: "get_board_summary",
		// boardId absent
	})
	if err == nil || !strings.Contains(err.Error(), "boardId") {
		t.Fatalf("err = %v, want boardId-required error", err)
	}
}

// TestGetBoardSummaryBoardIdHardenedAtLayerB locks the security fix: boardId
// carries NO schema pattern (Layer A — the golden test's noPattern list keeps
// zod parity), but the handler rejects a non-numeric boardId (Layer B requireID)
// so it cannot traverse into a privileged Planka path (e.g. boardSummary →
// GET /api/boards/<boardId>).
func TestGetBoardSummaryBoardIdHardenedAtLayerB(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchProjectBoard(context.Background(), c, projectBoardArgs{
		Action:  "get_board_summary",
		BoardID: sp("1/../../users"),
	})
	if err == nil || !strings.Contains(err.Error(), "numeric Planka ID") {
		t.Fatalf("boardId path-traversal must be rejected at Layer B, got: %v", err)
	}
}

// TestProjectBoardUnknownAction verifies the default branch.
func TestProjectBoardUnknownAction(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchProjectBoard(context.Background(), c, projectBoardArgs{
		Action: "frobnicate",
	})
	if err == nil || !strings.Contains(err.Error(), "unknown action") {
		t.Fatalf("err = %v, want unknown-action error", err)
	}
}
