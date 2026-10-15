package tools

import (
	"context"
	"strings"
	"testing"
)

// These lock the path-traversal hardening at Layer B. The schema golden tests
// also require the current TS plankaId pattern at Layer A for these fields.

func TestLabelRemoveRejectsNonNumericLabelID(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchLabel(context.Background(), c, labelArgs{
		Action: "remove_from_card", CardID: sp("9"), LabelId: sp("1/../../users"),
	})
	if err == nil || !strings.Contains(err.Error(), "numeric Planka ID") {
		t.Fatalf("labelId path-traversal must be rejected at Layer B, got %v", err)
	}
}

func TestCardDetailsRejectsNonNumericCardID(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchCard(context.Background(), c, cardArgs{
		Action: "get_details", CardID: sp("1/../../users"),
	})
	if err == nil || !strings.Contains(err.Error(), "numeric Planka ID") {
		t.Fatalf("cardId path-traversal must be rejected at Layer B, got %v", err)
	}
}

func TestTaskBatchRejectsNonNumericCardIDBeforeAnyRequest(t *testing.T) {
	for _, id := range []string{"abc", "1/task-lists", "1/../../users", "", "١"} {
		t.Run(id, func(t *testing.T) {
			m := &mockPlanka{}
			c := newToolClient(t, m)
			tasks := []taskBatchItem{{CardID: "42", Name: "valid first task"}, {CardID: id, Name: "invalid second task"}}
			_, err := dispatchTask(context.Background(), c, taskArgs{Action: "batch_create", Tasks: &tasks})
			if err == nil || !strings.Contains(err.Error(), "tasks[1].cardId") {
				t.Fatalf("batch must reject indexed invalid cardId, got %v", err)
			}
			if got := len(m.reqs()); got != 0 {
				t.Fatalf("invalid batch made %d requests before rejection", got)
			}
		})
	}
}
