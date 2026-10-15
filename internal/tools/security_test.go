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
