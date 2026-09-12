package planka

// Test Plan for internal/planka/tasks.go
//
// ensureTaskListID (Classification: I/O boundary — GET /api/cards/{id}, POST when none found)
//   [x] Happy/existing: included.taskLists present → returns first ID, no POST
//   [x] Happy/auto-create: no taskLists in included → POSTs /api/cards/{id}/task-lists
//   [x] Unhappy: GET /api/cards/{id} returns 404 → propagates error
//   [x] Unhappy: POST /api/cards/{id}/task-lists returns 500 → propagates error
//
// CreateTask, GetTasks, GetTask, UpdateTask, DeleteTask, BatchCreateTasks
//   Covered by tools/task_test.go via newToolClient round-trips — skipped here.

import (
	"context"
	"testing"
)

func TestEnsureTaskListIDReturnsFirstExistingTaskList(t *testing.T) {
	// When the card already has task lists, ensureTaskListID returns the first
	// one's ID and does NOT create a new one.
	m := &mockPlanka{
		tokens: []string{"tok"},
		handler: func(token string, r recordedRequest) (int, string) {
			if r.Method == "GET" && r.Path == "/api/cards/C1" {
				return 200, `{"item":{"id":"C1"},"included":{"taskLists":[{"id":"TL1"},{"id":"TL2"}]}}`
			}
			// Any POST here is a bug.
			t.Errorf("unexpected request: %s %s", r.Method, r.Path)
			return 404, `{"message":"unexpected"}`
		},
	}
	c, _ := newTestClient(t, m)

	id, err := c.ensureTaskListID(context.Background(), "C1")
	if err != nil {
		t.Fatalf("ensureTaskListID: %v", err)
	}
	if id != "TL1" {
		t.Errorf("id = %q, want TL1 (first task list)", id)
	}
	for _, req := range m.requests {
		if req.Method == "POST" {
			t.Errorf("unexpected POST to %s — should not create a new task list when one exists", req.Path)
		}
	}
}

func TestEnsureTaskListIDAutoCreatesWhenNoTaskLists(t *testing.T) {
	// When the card has no task lists, ensureTaskListID POSTs a new one and
	// returns its ID.
	m := &mockPlanka{
		tokens: []string{"tok"},
		handler: func(token string, r recordedRequest) (int, string) {
			switch {
			case r.Method == "GET" && r.Path == "/api/cards/C2":
				// Empty included block — triggers auto-create.
				return 200, `{"item":{"id":"C2"},"included":{}}`
			case r.Method == "POST" && r.Path == "/api/cards/C2/task-lists":
				return 200, `{"item":{"id":"NEW-TL","name":"Tasks","position":65535}}`
			}
			t.Errorf("unexpected: %s %s", r.Method, r.Path)
			return 404, `{"message":"unexpected"}`
		},
	}
	c, _ := newTestClient(t, m)

	id, err := c.ensureTaskListID(context.Background(), "C2")
	if err != nil {
		t.Fatalf("ensureTaskListID: %v", err)
	}
	if id != "NEW-TL" {
		t.Errorf("id = %q, want NEW-TL (the auto-created task list)", id)
	}
	// Verify a POST was actually made.
	postCount := 0
	for _, req := range m.requests {
		if req.Method == "POST" && req.Path == "/api/cards/C2/task-lists" {
			postCount++
		}
	}
	if postCount != 1 {
		t.Errorf("POST /api/cards/C2/task-lists count = %d, want 1", postCount)
	}
}

func TestEnsureTaskListIDNilIncludedAutoCreates(t *testing.T) {
	// A response with a nil included field also falls into the auto-create path.
	m := &mockPlanka{
		tokens: []string{"tok"},
		handler: func(token string, r recordedRequest) (int, string) {
			switch {
			case r.Method == "GET" && r.Path == "/api/cards/C3":
				return 200, `{"item":{"id":"C3"}}`
			case r.Method == "POST" && r.Path == "/api/cards/C3/task-lists":
				return 200, `{"item":{"id":"TL-NIL","name":"Tasks"}}`
			}
			t.Errorf("unexpected: %s %s", r.Method, r.Path)
			return 404, `{"message":"unexpected"}`
		},
	}
	c, _ := newTestClient(t, m)

	id, err := c.ensureTaskListID(context.Background(), "C3")
	if err != nil {
		t.Fatalf("ensureTaskListID: %v", err)
	}
	if id != "TL-NIL" {
		t.Errorf("id = %q, want TL-NIL", id)
	}
}

func TestEnsureTaskListIDPropagatesGetError(t *testing.T) {
	// A 404 on GET /api/cards/{id} propagates — does not swallow.
	m := &mockPlanka{
		tokens: []string{"tok"},
		handler: func(token string, r recordedRequest) (int, string) {
			return 404, `{"message":"card not found"}`
		},
	}
	c, _ := newTestClient(t, m)

	_, err := c.ensureTaskListID(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error from 404, got nil")
	}
	if !IsNotFound(err) {
		t.Errorf("err = %v, want IsNotFound", err)
	}
}

func TestEnsureTaskListIDPropagatesPostError(t *testing.T) {
	// A failure when creating the task list propagates.
	m := &mockPlanka{
		tokens: []string{"tok"},
		handler: func(token string, r recordedRequest) (int, string) {
			if r.Method == "GET" {
				return 200, `{"item":{"id":"C4"},"included":{}}`
			}
			// POST fails.
			return 500, `{"message":"failed to create task list"}`
		},
	}
	c, _ := newTestClient(t, m)

	_, err := c.ensureTaskListID(context.Background(), "C4")
	if err == nil {
		t.Fatal("expected error from POST 500, got nil")
	}
}
