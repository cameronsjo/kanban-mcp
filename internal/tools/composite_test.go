package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestCreateCardWithTasksSequence(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		switch {
		case r.Method == "POST" && r.Path == "/api/lists/55/cards":
			return 200, `{"item":{"id":"C1","listId":"55","name":"Feature"}}`
		case r.Method == "GET" && r.Path == "/api/cards/C1":
			// ensureTaskListID reuses an existing task list.
			return 200, `{"item":{"id":"C1","listId":"55"},"included":{"taskLists":[{"id":"TL1"}]}}`
		case r.Method == "POST" && r.Path == "/api/task-lists/TL1/tasks":
			return 200, `{"item":{"id":"T","name":"t","isCompleted":false}}`
		case r.Method == "POST" && r.Path == "/api/cards/C1/comments":
			return 200, `{"item":{"id":"CM1","text":"done"}}`
		}
		return 404, `{"message":"unexpected ` + r.Method + " " + r.Path + `"}`
	}}
	c := newToolClient(t, m)

	res, err := createCardWithTasks(context.Background(), c, CreateCardWithTasksParams{
		ListID: "55", Name: "Feature", Tasks: []string{"a", "b"}, Comment: sp("done"),
	})
	if err != nil {
		t.Fatalf("createCardWithTasks: %v", err)
	}
	out, _ := json.Marshal(res)
	s := string(out)
	for _, want := range []string{`"card":`, `"id":"C1"`, `"comment":`, `"id":"CM1"`} {
		if !strings.Contains(s, want) {
			t.Errorf("result %s missing %q", s, want)
		}
	}
	// The card create body carries the v2.1 required type and default position.
	cardBody := m.reqs()[0].Body
	for _, want := range []string{`"name":"Feature"`, `"type":"project"`, `"position":65535`} {
		if !strings.Contains(cardBody, want) {
			t.Errorf("card create body %q missing %q", cardBody, want)
		}
	}
	// Two tasks created (each at 65535*(i+1)).
	taskPosts := 0
	for _, rq := range m.reqs() {
		if rq.Method == "POST" && rq.Path == "/api/task-lists/TL1/tasks" {
			taskPosts++
		}
	}
	if taskPosts != 2 {
		t.Fatalf("task POSTs = %d, want 2", taskPosts)
	}
}

func TestCreateCardWithTasksPropagatesTaskFailure(t *testing.T) {
	// A mid-sequence failure surfaces the error (best-effort, no rollback).
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		switch {
		case r.Method == "POST" && r.Path == "/api/lists/55/cards":
			return 200, `{"item":{"id":"C1","listId":"55"}}`
		case r.Method == "GET" && r.Path == "/api/cards/C1":
			return 200, `{"item":{"id":"C1"},"included":{"taskLists":[{"id":"TL1"}]}}`
		case r.Method == "POST" && r.Path == "/api/task-lists/TL1/tasks":
			return 422, `{"message":"bad task"}`
		}
		return 404, `{"message":"unexpected"}`
	}}
	c := newToolClient(t, m)
	_, err := createCardWithTasks(context.Background(), c, CreateCardWithTasksParams{
		ListID: "55", Name: "X", Tasks: []string{"a"},
	})
	if err == nil {
		t.Fatal("expected the task failure to propagate")
	}
}

func TestBoardSummaryAssemblesStats(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		switch {
		case r.Method == "GET" && r.Path == "/api/boards/7":
			// board detail carries lists + labels in included.
			return 200, `{"item":{"id":"7","name":"B"},"included":{"lists":[{"id":"L1","boardId":"7","name":"Backlog"},{"id":"L2","boardId":"7","name":"Done"}],"labels":[]}}`
		case r.Method == "GET" && r.Path == "/api/projects":
			// GetCards board-walk: no boards → no cards.
			return 200, `{"items":[],"included":{}}`
		}
		return 404, `{"message":"unexpected ` + r.Method + " " + r.Path + `"}`
	}}
	c := newToolClient(t, m)

	res, err := boardSummary(context.Background(), c, "7", false, false)
	if err != nil {
		t.Fatalf("boardSummary: %v", err)
	}
	out, _ := json.Marshal(res)
	s := string(out)
	for _, want := range []string{
		`"totalCards":0`,
		`"backlogCount":0`,
		`"doneCount":0`,
		`"nextActionSuggestion":"All tasks complete! Create new cards or projects"`,
		`"cardCount":0`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("summary %s missing %q", s, want)
		}
	}
}
