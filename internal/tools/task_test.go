package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/cameronsjo/kanban-mcp/internal/planka"
)

// cardWithTaskListJSON returns a card response that includes one task-list so
// ensureTaskListID short-circuits without issuing a POST.
func cardWithTaskListJSON(taskListID string) string {
	return `{"item":{"id":"42","listId":"1","name":"Card"},"included":{"taskLists":[{"id":"` + taskListID + `","cardId":"42","name":"Tasks","position":65535}]}}`
}

// cardWithTasksJSON returns a card response including a single task.
func cardWithTasksJSON(taskID string) string {
	return `{"item":{"id":"42","listId":"1","name":"Card"},"included":{"tasks":[{"id":"` + taskID + `","name":"My Task","isCompleted":false,"position":65535}]}}`
}

func TestTaskGetAllSwallowsErrorToEmpty(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		return 500, `{"message":"boom"}`
	}}
	c := newToolClient(t, m)
	res, err := dispatchTask(context.Background(), c, taskArgs{Action: "get_all", CardID: sp("42")})
	if err != nil {
		t.Fatalf("get_all should swallow errors, got: %v", err)
	}
	tasks, ok := res.([]planka.Task)
	if !ok {
		t.Fatalf("result type = %T, want []planka.Task", res)
	}
	if len(tasks) != 0 {
		t.Errorf("expected empty slice on error, got %d tasks", len(tasks))
	}
}

func TestTaskCreateHappyPath(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		// First call: GET card to find task-list.
		if r.Method == "GET" && strings.Contains(r.Path, "/api/cards/42") {
			return 200, cardWithTaskListJSON("7")
		}
		// Second call: POST task under task-list 7.
		if r.Method == "POST" && r.Path == "/api/task-lists/7/tasks" {
			return 200, `{"item":{"id":"99","name":"My Task","isCompleted":false,"position":65535}}`
		}
		return 404, `{"message":"unexpected ` + r.Method + " " + r.Path + `"}`
	}}
	c := newToolClient(t, m)

	res, err := dispatchTask(context.Background(), c, taskArgs{
		Action: "create", CardID: sp("42"), Name: sp("My Task"),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	reqs := m.reqs()
	// Find the POST to task-lists.
	var postBody string
	for _, r := range reqs {
		if r.Method == "POST" {
			postBody = r.Body
			break
		}
	}
	for _, want := range []string{`"name":"My Task"`, `"position":65535`} {
		if !strings.Contains(postBody, want) {
			t.Errorf("create body %q missing %q", postBody, want)
		}
	}
}

func TestTaskCreateMissingCardIDErrors(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchTask(context.Background(), c, taskArgs{
		Action: "create", Name: sp("X"),
	})
	if err == nil || !strings.Contains(err.Error(), "cardId") {
		t.Fatalf("err = %v, want cardId required error", err)
	}
}

func TestTaskCreateMissingNameErrors(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchTask(context.Background(), c, taskArgs{
		Action: "create", CardID: sp("42"),
	})
	if err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("err = %v, want name required error", err)
	}
}

func TestTaskGetOneRequiresCardID(t *testing.T) {
	// get_one passes "" as cardId — GetTask must error immediately.
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchTask(context.Background(), c, taskArgs{
		Action: "get_one", ID: sp("55"),
	})
	if err == nil || !strings.Contains(err.Error(), "Card ID is required") {
		t.Fatalf("err = %v, want Card ID required error", err)
	}
}

func TestTaskGetOneRejectsNonNumericID(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchTask(context.Background(), c, taskArgs{
		Action: "get_one", ID: sp("../../../etc/passwd"),
	})
	if err == nil || !strings.Contains(err.Error(), "numeric Planka ID") {
		t.Fatalf("err = %v, want numeric-ID rejection", err)
	}
}

func TestTaskUpdateHappyPath(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		if r.Method == "PATCH" && r.Path == "/api/tasks/55" {
			return 200, `{"item":{"id":"55","name":"Updated","isCompleted":false,"position":100}}`
		}
		return 404, `{"message":"not found"}`
	}}
	c := newToolClient(t, m)

	res, err := dispatchTask(context.Background(), c, taskArgs{
		Action: "update", ID: sp("55"), Name: sp("Updated"), Position: fp(100),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	req := m.reqs()[0]
	if req.Method != "PATCH" || req.Path != "/api/tasks/55" {
		t.Errorf("update hit %s %s, want PATCH /api/tasks/55", req.Method, req.Path)
	}
	for _, want := range []string{`"name":"Updated"`, `"position":100`} {
		if !strings.Contains(req.Body, want) {
			t.Errorf("update body %q missing %q", req.Body, want)
		}
	}
}

func TestTaskCompleteTaskHappyPath(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		if r.Method == "PATCH" && r.Path == "/api/tasks/55" {
			return 200, `{"item":{"id":"55","name":"Task","isCompleted":true,"position":65535}}`
		}
		return 404, `{"message":"not found"}`
	}}
	c := newToolClient(t, m)

	_, err := dispatchTask(context.Background(), c, taskArgs{
		Action: "complete_task", ID: sp("55"),
	})
	if err != nil {
		t.Fatalf("complete_task: %v", err)
	}
	req := m.reqs()[0]
	if !strings.Contains(req.Body, `"isCompleted":true`) {
		t.Errorf("complete_task body %q missing isCompleted:true", req.Body)
	}
}

func TestTaskDeleteHappyPath(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		return 200, `{}`
	}}
	c := newToolClient(t, m)

	res, err := dispatchTask(context.Background(), c, taskArgs{
		Action: "delete", ID: sp("55"),
	})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	req := m.reqs()[0]
	if req.Method != "DELETE" || req.Path != "/api/tasks/55" {
		t.Errorf("delete hit %s %s, want DELETE /api/tasks/55", req.Method, req.Path)
	}
	success, ok := res.(map[string]bool)
	if !ok || !success["success"] {
		t.Errorf("delete result = %v, want {success:true}", res)
	}
}

func TestTaskBatchCreateHappyPath(t *testing.T) {
	callCount := 0
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		// Every GET /api/cards/... returns a task-list.
		if r.Method == "GET" && strings.HasPrefix(r.Path, "/api/cards/") {
			return 200, cardWithTaskListJSON("7")
		}
		if r.Method == "POST" && r.Path == "/api/task-lists/7/tasks" {
			callCount++
			return 200, `{"item":{"id":"` + string(rune('0'+callCount)) + `","name":"task","isCompleted":false,"position":65535}}`
		}
		return 404, `{"message":"not found"}`
	}}
	c := newToolClient(t, m)

	items := []taskBatchItem{
		{CardID: "42", Name: "Task A"},
		{CardID: "42", Name: "Task B"},
	}
	res, err := dispatchTask(context.Background(), c, taskArgs{
		Action: "batch_create", Tasks: &items,
	})
	if err != nil {
		t.Fatalf("batch_create: %v", err)
	}
	br, ok := res.(planka.BatchResult)
	if !ok {
		t.Fatalf("result type = %T, want planka.BatchResult", res)
	}
	if len(br.Results) != 2 {
		t.Errorf("got %d results, want 2", len(br.Results))
	}
	if len(br.Successes) != 2 {
		t.Errorf("got %d successes, want 2", len(br.Successes))
	}
	if len(br.Failures) != 0 {
		t.Errorf("got %d failures, want 0", len(br.Failures))
	}
}

func TestTaskBatchCreateEmptyErrors(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	empty := []taskBatchItem{}
	_, err := dispatchTask(context.Background(), c, taskArgs{
		Action: "batch_create", Tasks: &empty,
	})
	if err == nil || !strings.Contains(err.Error(), "tasks array is required") {
		t.Fatalf("err = %v, want tasks-required error", err)
	}
}

func TestTaskBatchCreateNilTasksErrors(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchTask(context.Background(), c, taskArgs{
		Action: "batch_create", // Tasks is nil
	})
	if err == nil || !strings.Contains(err.Error(), "tasks array is required") {
		t.Fatalf("err = %v, want tasks-required error", err)
	}
}

func TestTaskUnknownActionErrors(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchTask(context.Background(), c, taskArgs{Action: "frobnicate"})
	if err == nil || !strings.Contains(err.Error(), "unknown action") {
		t.Fatalf("err = %v, want unknown-action error", err)
	}
}

// TestTaskEnsureTaskListIDCreatesWhenAbsent verifies the v2.1 quirk: when a card
// has no task-lists, CreateTask POSTs to /api/cards/{id}/task-lists first.
func TestTaskEnsureTaskListIDCreatesWhenAbsent(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		if r.Method == "GET" && r.Path == "/api/cards/42" {
			// No included.taskLists — triggers auto-create.
			return 200, `{"item":{"id":"42","listId":"1","name":"Card"}}`
		}
		if r.Method == "POST" && r.Path == "/api/cards/42/task-lists" {
			return 200, `{"item":{"id":"99","name":"Tasks","position":65535}}`
		}
		if r.Method == "POST" && r.Path == "/api/task-lists/99/tasks" {
			return 200, `{"item":{"id":"1","name":"Auto","isCompleted":false,"position":65535}}`
		}
		return 404, `{"message":"unexpected ` + r.Method + " " + r.Path + `"}`
	}}
	c := newToolClient(t, m)

	_, err := dispatchTask(context.Background(), c, taskArgs{
		Action: "create", CardID: sp("42"), Name: sp("Auto"),
	})
	if err != nil {
		t.Fatalf("create with auto task-list: %v", err)
	}
	// Confirm the task-list POST fired.
	var sawTaskListPost bool
	for _, r := range m.reqs() {
		if r.Method == "POST" && r.Path == "/api/cards/42/task-lists" {
			sawTaskListPost = true
			if !strings.Contains(r.Body, `"name":"Tasks"`) {
				t.Errorf("task-list POST body %q, want Tasks name", r.Body)
			}
		}
	}
	if !sawTaskListPost {
		t.Error("expected POST to /api/cards/42/task-lists but did not see it")
	}
}
