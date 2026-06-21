package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/cameronsjo/kanban-mcp/internal/planka"
)

// minimalCardJSON returns a card item envelope with no stopwatch.
func minimalCardJSON(id string) string {
	return `{"item":{"id":"` + id + `","listId":"7","name":"Card","position":1,"createdAt":"2024-01-01T00:00:00Z"}}`
}

func TestStopwatchStartSendsStopwatchBody(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		switch {
		case r.Method == "GET" && r.Path == "/api/cards/42":
			return 200, minimalCardJSON("42")
		case r.Method == "PATCH" && r.Path == "/api/cards/42":
			return 200, minimalCardJSON("42")
		}
		return 404, `{"message":"unexpected"}`
	}}
	c := newToolClient(t, m)
	res, err := dispatchStopwatch(context.Background(), c, stopwatchArgs{Action: "start", ID: "42"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	reqs := m.reqs()
	patchReq := reqs[len(reqs)-1]
	if patchReq.Method != "PATCH" || patchReq.Path != "/api/cards/42" {
		t.Errorf("start hit %s %s, want PATCH /api/cards/42", patchReq.Method, patchReq.Path)
	}
	if !strings.Contains(patchReq.Body, `"stopwatch"`) {
		t.Errorf("start body %q missing 'stopwatch'", patchReq.Body)
	}
	if !strings.Contains(patchReq.Body, `"startedAt"`) {
		t.Errorf("start body %q missing 'startedAt'", patchReq.Body)
	}
	if !strings.Contains(patchReq.Body, `"total"`) {
		t.Errorf("start body %q missing 'total'", patchReq.Body)
	}
}

func TestStopwatchStopNotRunningReturnsCardUnchanged(t *testing.T) {
	callCount := 0
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		callCount++
		if r.Method == "GET" && r.Path == "/api/cards/42" {
			// No stopwatch → not running.
			return 200, minimalCardJSON("42")
		}
		return 404, `{"message":"unexpected"}`
	}}
	c := newToolClient(t, m)
	res, err := dispatchStopwatch(context.Background(), c, stopwatchArgs{Action: "stop", ID: "42"})
	if err != nil {
		t.Fatalf("stop (not running): %v", err)
	}
	card, ok := res.(*planka.Card)
	if !ok || card.ID != "42" {
		t.Errorf("got %+v, want card id=42", res)
	}
	// PATCH must NOT have been called.
	for _, r := range m.reqs() {
		if r.Method == "PATCH" {
			t.Errorf("stop (not running) should not PATCH, but got PATCH %s", r.Path)
		}
	}
}

func TestStopwatchStopRunningAccumulatesTotal(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		switch {
		case r.Method == "GET" && r.Path == "/api/cards/42":
			// Stopwatch is running; total already 60s.
			return 200, `{"item":{"id":"42","listId":"7","name":"Card","position":1,"createdAt":"2024-01-01T00:00:00Z","stopwatch":{"startedAt":"2020-01-01T00:00:00Z","total":60}}}`
		case r.Method == "PATCH" && r.Path == "/api/cards/42":
			return 200, minimalCardJSON("42")
		}
		return 404, `{"message":"unexpected"}`
	}}
	c := newToolClient(t, m)
	res, err := dispatchStopwatch(context.Background(), c, stopwatchArgs{Action: "stop", ID: "42"})
	if err != nil {
		t.Fatalf("stop (running): %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	reqs := m.reqs()
	patchReq := reqs[len(reqs)-1]
	if patchReq.Method != "PATCH" {
		t.Errorf("stop (running) should PATCH, got %s", patchReq.Method)
	}
	if !strings.Contains(patchReq.Body, `"stopwatch"`) {
		t.Errorf("stop body %q missing 'stopwatch'", patchReq.Body)
	}
}

func TestStopwatchGetNoStopwatch(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		if r.Method == "GET" && r.Path == "/api/cards/42" {
			return 200, minimalCardJSON("42")
		}
		return 404, `{"message":"unexpected"}`
	}}
	c := newToolClient(t, m)
	res, err := dispatchStopwatch(context.Background(), c, stopwatchArgs{Action: "get", ID: "42"})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	m2, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", res)
	}
	if m2["isRunning"] != false {
		t.Errorf("isRunning = %v, want false", m2["isRunning"])
	}
	if m2["total"] != float64(0) {
		t.Errorf("total = %v, want 0", m2["total"])
	}
	if m2["formattedTotal"] != "0s" {
		t.Errorf("formattedTotal = %v, want '0s'", m2["formattedTotal"])
	}
}

func TestStopwatchReset(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		if r.Method == "PATCH" && r.Path == "/api/cards/42" {
			return 200, minimalCardJSON("42")
		}
		return 404, `{"message":"unexpected"}`
	}}
	c := newToolClient(t, m)
	res, err := dispatchStopwatch(context.Background(), c, stopwatchArgs{Action: "reset", ID: "42"})
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	reqs := m.reqs()
	patchReq := reqs[0]
	if patchReq.Method != "PATCH" || patchReq.Path != "/api/cards/42" {
		t.Errorf("reset hit %s %s, want PATCH /api/cards/42", patchReq.Method, patchReq.Path)
	}
	// Body must contain "stopwatch":null.
	if !strings.Contains(patchReq.Body, `"stopwatch":null`) {
		t.Errorf("reset body %q missing '\"stopwatch\":null'", patchReq.Body)
	}
}

func TestStopwatchRejectsNonNumericID(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchStopwatch(context.Background(), c, stopwatchArgs{Action: "start", ID: "abc"})
	if err == nil || !strings.Contains(err.Error(), "numeric Planka ID") {
		t.Fatalf("err = %v, want numeric-ID rejection", err)
	}
}

func TestStopwatchUnknownActionErrors(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchStopwatch(context.Background(), c, stopwatchArgs{Action: "frobnicate", ID: "42"})
	if err == nil || !strings.Contains(err.Error(), "unknown action") {
		t.Fatalf("err = %v, want unknown-action error", err)
	}
}

// formatDuration is an unexported planka helper; its unit test lives in
// internal/planka/stopwatch_test.go (TestFormatDuration).
