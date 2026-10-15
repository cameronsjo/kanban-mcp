package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/cameronsjo/kanban-mcp/internal/planka"
)

// cardJSON returns a minimal JSON card item envelope.
func cardJSON(id, listID, name string) string {
	return `{"item":{"id":"` + id + `","listId":"` + listID + `","name":"` + name + `","position":65535,"createdAt":"2024-01-01T00:00:00Z"}}`
}

func TestCardGetAllSendsToCorrectBoard(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		switch {
		case r.Method == "GET" && r.Path == "/api/projects":
			return 200, `{"items":[],"included":{"boards":[{"id":"10","projectId":"1","name":"B","position":1,"createdAt":"2024-01-01T00:00:00Z"}]}}`
		case r.Method == "GET" && r.Path == "/api/boards/10":
			return 200, `{"item":{"id":"10","projectId":"1","name":"B","position":1,"createdAt":"2024-01-01T00:00:00Z"},"included":{"cards":[{"id":"42","listId":"7","name":"MyCard","position":1,"createdAt":"2024-01-01T00:00:00Z"}]}}`
		}
		return 404, `{"message":"unexpected"}`
	}}
	c := newToolClient(t, m)

	res, err := dispatchCard(context.Background(), c, cardArgs{Action: "get_all", ListId: sp("7")})
	if err != nil {
		t.Fatalf("get_all: %v", err)
	}
	cards, ok := res.([]planka.Card)
	if !ok {
		t.Fatalf("result type = %T, want []planka.Card", res)
	}
	if len(cards) != 1 || cards[0].ID != "42" {
		t.Errorf("got %+v, want card id=42", cards)
	}
}

func TestCardGetAllSwallowsErrorToEmpty(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		return 500, `{"message":"boom"}`
	}}
	c := newToolClient(t, m)
	res, err := dispatchCard(context.Background(), c, cardArgs{Action: "get_all", ListId: sp("7")})
	if err != nil {
		t.Fatalf("get_all should swallow to empty, got err %v", err)
	}
	cards, ok := res.([]planka.Card)
	if !ok {
		t.Fatalf("result type = %T, want []planka.Card", res)
	}
	if len(cards) != 0 {
		t.Errorf("expected empty on swallowed error, got %d", len(cards))
	}
}

func TestCardGetAllMissingListIdErrors(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchCard(context.Background(), c, cardArgs{Action: "get_all"})
	if err == nil || !strings.Contains(err.Error(), "listId") {
		t.Fatalf("err = %v, want listId-required error", err)
	}
}

func TestCardCreate(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		if r.Method == "POST" && r.Path == "/api/lists/20/cards" {
			return 200, cardJSON("55", "20", "New Card")
		}
		return 404, `{"message":"unexpected"}`
	}}
	c := newToolClient(t, m)

	res, err := dispatchCard(context.Background(), c, cardArgs{
		Action: "create", ListId: sp("20"), Name: sp("New Card"),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	body := m.reqs()[0].Body
	for _, want := range []string{`"name":"New Card"`, `"description":""`, `"position":0`, `"type":"project"`} {
		if !strings.Contains(body, want) {
			t.Errorf("create body %q missing %q", body, want)
		}
	}
}

func TestCardCreateMissingNameErrors(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchCard(context.Background(), c, cardArgs{Action: "create", ListId: sp("20")})
	if err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("err = %v, want name-required error", err)
	}
}

func TestCardGetOne(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		if r.Method == "GET" && r.Path == "/api/cards/42" {
			return 200, cardJSON("42", "7", "My Card")
		}
		return 404, `{"message":"unexpected"}`
	}}
	c := newToolClient(t, m)
	res, err := dispatchCard(context.Background(), c, cardArgs{Action: "get_one", ID: sp("42")})
	if err != nil {
		t.Fatalf("get_one: %v", err)
	}
	card, ok := res.(*planka.Card)
	if !ok || card.ID != "42" {
		t.Errorf("got %+v, want card id=42", res)
	}
}

func TestCardGetOneRejectsNonNumericID(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchCard(context.Background(), c, cardArgs{Action: "get_one", ID: sp("1/../../users")})
	if err == nil || !strings.Contains(err.Error(), "numeric Planka ID") {
		t.Fatalf("err = %v, want numeric-ID rejection", err)
	}
}

func TestCardUpdate(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		if r.Method == "PATCH" && r.Path == "/api/cards/42" {
			return 200, cardJSON("42", "7", "Updated")
		}
		return 404, `{"message":"unexpected"}`
	}}
	c := newToolClient(t, m)
	res, err := dispatchCard(context.Background(), c, cardArgs{
		Action: "update", ID: sp("42"), Name: sp("Updated"), Position: fp(100),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	body := m.reqs()[0].Body
	if !strings.Contains(body, `"name":"Updated"`) {
		t.Errorf("update body %q missing name", body)
	}
	if !strings.Contains(body, `"position":100`) {
		t.Errorf("update body %q missing position", body)
	}
}

func TestCardMove(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		if r.Method == "PATCH" && r.Path == "/api/cards/42" {
			return 200, cardJSON("42", "99", "Card")
		}
		return 404, `{"message":"unexpected"}`
	}}
	c := newToolClient(t, m)
	res, err := dispatchCard(context.Background(), c, cardArgs{
		Action: "move", ID: sp("42"), ListId: sp("99"), Position: fp(65535),
	})
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	body := m.reqs()[0].Body
	for _, want := range []string{`"listId":"99"`, `"position":65535`} {
		if !strings.Contains(body, want) {
			t.Errorf("move body %q missing %q", body, want)
		}
	}
}

func TestCardMoveMissingPositionErrors(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchCard(context.Background(), c, cardArgs{Action: "move", ID: sp("42"), ListId: sp("99")})
	if err == nil || !strings.Contains(err.Error(), "position") {
		t.Fatalf("err = %v, want position-required error", err)
	}
}

func TestCardDuplicate(t *testing.T) {
	callCount := 0
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		callCount++
		switch {
		case r.Method == "GET" && r.Path == "/api/cards/42":
			return 200, `{"item":{"id":"42","listId":"7","name":"Original","position":1,"createdAt":"2024-01-01T00:00:00Z"}}`
		case r.Method == "POST" && r.Path == "/api/lists/7/cards":
			return 200, cardJSON("43", "7", "Copy of Original")
		}
		return 404, `{"message":"unexpected"}`
	}}
	c := newToolClient(t, m)
	res, err := dispatchCard(context.Background(), c, cardArgs{
		Action: "duplicate", ID: sp("42"), Position: fp(200),
	})
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	reqs := m.reqs()
	postReq := reqs[len(reqs)-1]
	if !strings.Contains(postReq.Body, `"Copy of Original"`) {
		t.Errorf("duplicate body %q missing 'Copy of Original'", postReq.Body)
	}
}

func TestCardDuplicateMissingPositionErrors(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchCard(context.Background(), c, cardArgs{Action: "duplicate", ID: sp("42")})
	if err == nil || !strings.Contains(err.Error(), "position") {
		t.Fatalf("err = %v, want position-required error", err)
	}
}

func TestCardDelete(t *testing.T) {
	m := &mockPlanka{respond: func(r capturedRequest) (int, string) {
		return 200, `{}`
	}}
	c := newToolClient(t, m)
	res, err := dispatchCard(context.Background(), c, cardArgs{Action: "delete", ID: sp("42")})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	got := m.reqs()[0]
	if got.Method != "DELETE" || got.Path != "/api/cards/42" {
		t.Errorf("delete hit %s %s, want DELETE /api/cards/42", got.Method, got.Path)
	}
	result, ok := res.(map[string]bool)
	if !ok || !result["success"] {
		t.Errorf("delete result = %v, want {success:true}", res)
	}
}

func TestCardGetDetailsRequiresCardId(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchCard(context.Background(), c, cardArgs{Action: "get_details"})
	if err == nil || !strings.Contains(err.Error(), "cardId") {
		t.Fatalf("err = %v, want cardId-required error", err)
	}
}

func TestCardUnknownActionErrors(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchCard(context.Background(), c, cardArgs{Action: "frobnicate"})
	if err == nil || !strings.Contains(err.Error(), "unknown action") {
		t.Fatalf("err = %v, want unknown-action error", err)
	}
}

// TestCardCreateWithTasksRequiresListIdAndName verifies create_with_tasks enforces its required fields.
func TestCardCreateWithTasksRequiresListIdAndName(t *testing.T) {
	c := newToolClient(t, &mockPlanka{})
	_, err := dispatchCard(context.Background(), c, cardArgs{Action: "create_with_tasks", ListId: sp("20")})
	if err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("err = %v, want name-required error", err)
	}
}
