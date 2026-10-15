package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/cameronsjo/kanban-mcp/internal/planka"
)

// boardWithMembershipsJSON returns a board detail response with one membership.
func boardWithMembershipsJSON(membershipID string) string {
	return `{"item":{"id":"10","projectId":"1","name":"Board","position":65535},"included":{"boardMemberships":[{"id":"` + membershipID + `","boardId":"10","userId":"5","role":"editor","canComment":null,"createdAt":"2024-01-01T00:00:00Z"}]}}`
}

func TestMembershipGetAllHappyPath(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		if r.Method == "GET" && r.Path == "/api/boards/10" {
			return 200, boardWithMembershipsJSON("99")
		}
		return 404, `{"message":"not found"}`
	}}
	c := newToolClient(t, m)

	res, err := dispatchMembership(context.Background(), c, membershipArgs{
		Action: "get_all", BoardID: sp("10"),
	})
	if err != nil {
		t.Fatalf("get_all: %v", err)
	}
	memberships, ok := res.([]planka.BoardMembership)
	if !ok {
		t.Fatalf("result type = %T, want []planka.BoardMembership", res)
	}
	if len(memberships) != 1 || memberships[0].ID != "99" {
		t.Errorf("got memberships = %v, want [{ID:99}]", memberships)
	}
}

func TestMembershipGetAllPropagatesError(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		return 500, `{"message":"internal error"}`
	}}
	c := newToolClient(t, m)

	_, err := dispatchMembership(context.Background(), c, membershipArgs{
		Action: "get_all", BoardID: sp("10"),
	})
	if err == nil {
		t.Fatal("get_all should propagate errors, got nil")
	}
}

func TestMembershipGetAllMissingBoardIDErrors(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchMembership(context.Background(), c, membershipArgs{
		Action: "get_all",
	})
	if err == nil || !strings.Contains(err.Error(), "boardId") {
		t.Fatalf("err = %v, want boardId required error", err)
	}
}

func TestMembershipCreateHappyPath(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		if r.Method == "POST" && r.Path == "/api/boards/10/board-memberships" {
			return 200, `{"item":{"id":"99","boardId":"10","userId":"5","role":"editor","canComment":null,"createdAt":"2024-01-01T00:00:00Z"}}`
		}
		return 404, `{"message":"not found"}`
	}}
	c := newToolClient(t, m)

	res, err := dispatchMembership(context.Background(), c, membershipArgs{
		Action: "create", BoardID: sp("10"), UserID: sp("5"), Role: sp("editor"),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	req := m.reqs()[0]
	if req.Method != "POST" || req.Path != "/api/boards/10/board-memberships" {
		t.Errorf("create hit %s %s, want POST /api/boards/10/board-memberships", req.Method, req.Path)
	}
	for _, want := range []string{`"userId":"5"`, `"role":"editor"`} {
		if !strings.Contains(req.Body, want) {
			t.Errorf("create body %q missing %q", req.Body, want)
		}
	}
}

func TestMembershipCreateMissingRoleErrors(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchMembership(context.Background(), c, membershipArgs{
		Action: "create", BoardID: sp("10"), UserID: sp("5"),
	})
	if err == nil || !strings.Contains(err.Error(), "role") {
		t.Fatalf("err = %v, want role required error", err)
	}
}

func TestMembershipCreateRejectsNonNumericBoardID(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchMembership(context.Background(), c, membershipArgs{
		Action: "create", BoardID: sp("bad/id"), UserID: sp("5"), Role: sp("editor"),
	})
	if err == nil || !strings.Contains(err.Error(), "numeric Planka ID") {
		t.Fatalf("err = %v, want numeric-ID rejection for boardId", err)
	}
}

func TestMembershipGetOneHappyPath(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		if r.Method == "GET" && r.Path == "/api/board-memberships/99" {
			return 200, `{"item":{"id":"99","boardId":"10","userId":"5","role":"viewer","canComment":null,"createdAt":"2024-01-01T00:00:00Z"}}`
		}
		return 404, `{"message":"not found"}`
	}}
	c := newToolClient(t, m)

	res, err := dispatchMembership(context.Background(), c, membershipArgs{
		Action: "get_one", ID: sp("99"),
	})
	if err != nil {
		t.Fatalf("get_one: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	req := m.reqs()[0]
	if req.Method != "GET" || req.Path != "/api/board-memberships/99" {
		t.Errorf("get_one hit %s %s, want GET /api/board-memberships/99", req.Method, req.Path)
	}
}

func TestMembershipUpdateHappyPath(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		if r.Method == "PATCH" && r.Path == "/api/board-memberships/99" {
			return 200, `{"item":{"id":"99","boardId":"10","userId":"5","role":"viewer","canComment":true,"createdAt":"2024-01-01T00:00:00Z"}}`
		}
		return 404, `{"message":"not found"}`
	}}
	c := newToolClient(t, m)

	canComment := true
	res, err := dispatchMembership(context.Background(), c, membershipArgs{
		Action: "update", ID: sp("99"), Role: sp("viewer"), CanComment: &canComment,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	req := m.reqs()[0]
	if req.Method != "PATCH" || req.Path != "/api/board-memberships/99" {
		t.Errorf("update hit %s %s, want PATCH /api/board-memberships/99", req.Method, req.Path)
	}
	for _, want := range []string{`"role":"viewer"`, `"canComment":true`} {
		if !strings.Contains(req.Body, want) {
			t.Errorf("update body %q missing %q", req.Body, want)
		}
	}
}

func TestMembershipUpdateOnlyIDRequired(t *testing.T) {
	// update requires only id; role/canComment are optional — an empty body is fine.
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		return 200, `{"item":{"id":"99","boardId":"10","userId":"5","role":"editor","canComment":null,"createdAt":"2024-01-01T00:00:00Z"}}`
	}}
	c := newToolClient(t, m)

	_, err := dispatchMembership(context.Background(), c, membershipArgs{
		Action: "update", ID: sp("99"),
	})
	if err != nil {
		t.Fatalf("update with no optional fields: %v", err)
	}
}

func TestMembershipDeleteHappyPath(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		return 200, `{}`
	}}
	c := newToolClient(t, m)

	res, err := dispatchMembership(context.Background(), c, membershipArgs{
		Action: "delete", ID: sp("99"),
	})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	req := m.reqs()[0]
	if req.Method != "DELETE" || req.Path != "/api/board-memberships/99" {
		t.Errorf("delete hit %s %s, want DELETE /api/board-memberships/99", req.Method, req.Path)
	}
	success, ok := res.(map[string]bool)
	if !ok || !success["success"] {
		t.Errorf("delete result = %v, want {success:true}", res)
	}
}

func TestMembershipDeleteRejectsNonNumericID(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchMembership(context.Background(), c, membershipArgs{
		Action: "delete", ID: sp("1/../../users"),
	})
	if err == nil || !strings.Contains(err.Error(), "numeric Planka ID") {
		t.Fatalf("err = %v, want numeric-ID rejection", err)
	}
}

func TestMembershipUnknownActionErrors(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchMembership(context.Background(), c, membershipArgs{Action: "frobnicate"})
	if err == nil || !strings.Contains(err.Error(), "unknown action") {
		t.Fatalf("err = %v, want unknown-action error", err)
	}
}
