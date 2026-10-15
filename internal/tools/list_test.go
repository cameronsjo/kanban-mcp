package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/cameronsjo/kanban-mcp/internal/planka"
)

func TestListCreateSendsTypeAndPosition(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		if r.Method == "POST" && r.Path == "/api/boards/10/lists" {
			return 200, `{"item":{"id":"99","boardId":"10","name":"Todo","position":65535}}`
		}
		return 404, `{"message":"unexpected ` + r.Method + " " + r.Path + `"}`
	}}
	c := newToolClient(t, m)

	res, err := dispatchList(context.Background(), c, listArgs{
		Action: "create", BoardID: sp("10"), Name: sp("Todo"), Position: fp(65535),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	body := m.reqs()[0].Body
	for _, want := range []string{`"name":"Todo"`, `"position":65535`, `"type":"active"`} {
		if !strings.Contains(body, want) {
			t.Errorf("create body %q missing %q", body, want)
		}
	}
}

func TestListCreateMissingPositionErrors(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchList(context.Background(), c, listArgs{
		Action: "create", BoardID: sp("10"), Name: sp("Todo"), // no Position
	})
	if err == nil || !strings.Contains(err.Error(), "position") {
		t.Fatalf("err = %v, want a position-required error", err)
	}
}

func TestListGetOneRejectsNonNumericID(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchList(context.Background(), c, listArgs{
		Action: "get_one", ID: sp("1/../../users"),
	})
	if err == nil || !strings.Contains(err.Error(), "numeric Planka ID") {
		t.Fatalf("err = %v, want numeric-ID rejection (Layer B)", err)
	}
}

func TestListDeleteSuccessShape(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		return 200, `{}`
	}}
	c := newToolClient(t, m)
	res, err := dispatchList(context.Background(), c, listArgs{Action: "delete", ID: sp("5")})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := m.reqs()[0]; got.Method != "DELETE" || got.Path != "/api/lists/5" {
		t.Errorf("delete hit %s %s, want DELETE /api/lists/5", got.Method, got.Path)
	}
	_ = res
}

func TestListUnknownActionErrors(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchList(context.Background(), c, listArgs{Action: "frobnicate"})
	if err == nil || !strings.Contains(err.Error(), "unknown action") {
		t.Fatalf("err = %v, want unknown-action error", err)
	}
}

func TestListGetAllSwallowsErrorToEmpty(t *testing.T) {
	// Mirrors getLists in lists.ts: a request error yields [] (best-effort), not
	// a surfaced failure.
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		return 500, `{"message":"boom"}`
	}}
	c := newToolClient(t, m)
	res, err := dispatchList(context.Background(), c, listArgs{Action: "get_all", BoardID: sp("10")})
	if err != nil {
		t.Fatalf("get_all should swallow to empty, got err %v", err)
	}
	lists, ok := res.([]planka.List)
	if !ok {
		t.Fatalf("result type = %T, want []planka.List", res)
	}
	if len(lists) != 0 {
		t.Errorf("expected empty list on swallowed error, got %d", len(lists))
	}
}
