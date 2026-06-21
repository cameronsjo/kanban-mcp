package tools

import (
	"context"
	"strings"
	"testing"
)

func TestLabelDispatch(t *testing.T) {
	labelJSON := `{"item":{"id":"1","boardId":"10","name":"P0","color":"berry-red","position":65535,"createdAt":"2024-01-01T00:00:00Z"}}`
	labelsJSON := `{"item":{"id":"10","boardId":"99","name":"board","position":1,"createdAt":"2024-01-01T00:00:00Z"},"included":{"labels":[{"id":"1","boardId":"10","name":"P0","color":"berry-red","position":65535,"createdAt":"2024-01-01T00:00:00Z"}]}}`
	successJSON := `{"item":{}}`

	tests := []struct {
		name       string
		args       labelArgs
		wantMethod string
		wantPath   string
		wantBody   string
		wantErr    string
		respond    func(capturedRequest) (int, string)
	}{
		// Happy-path: get_all
		{
			name:       "get_all happy",
			args:       labelArgs{Action: "get_all", BoardID: sp("10")},
			wantMethod: "GET",
			wantPath:   "/api/boards/10",
			respond:    func(_ capturedRequest) (int, string) { return 200, labelsJSON },
		},
		// Happy-path: create
		{
			name:       "create happy",
			args:       labelArgs{Action: "create", BoardID: sp("10"), Name: sp("Bug"), Color: sp("coral-green"), Position: fp(65535)},
			wantMethod: "POST",
			wantPath:   "/api/boards/10/labels",
			wantBody:   `"name":"Bug"`,
			respond:    func(_ capturedRequest) (int, string) { return 200, labelJSON },
		},
		// Happy-path: update
		{
			name:       "update happy",
			args:       labelArgs{Action: "update", ID: sp("1"), Name: sp("P0"), Color: sp("berry-red"), Position: fp(65535)},
			wantMethod: "PATCH",
			wantPath:   "/api/labels/1",
			wantBody:   `"color":"berry-red"`,
			respond:    func(_ capturedRequest) (int, string) { return 200, labelJSON },
		},
		// Happy-path: delete
		{
			name:       "delete happy",
			args:       labelArgs{Action: "delete", ID: sp("1")},
			wantMethod: "DELETE",
			wantPath:   "/api/labels/1",
			respond:    func(_ capturedRequest) (int, string) { return 200, successJSON },
		},
		// Happy-path: add_to_card
		{
			name:       "add_to_card happy",
			args:       labelArgs{Action: "add_to_card", CardID: sp("42"), LabelId: sp("7")},
			wantMethod: "POST",
			wantPath:   "/api/cards/42/card-labels",
			wantBody:   `"labelId":"7"`,
			respond:    func(_ capturedRequest) (int, string) { return 200, successJSON },
		},
		// Happy-path: remove_from_card (also tests colon in raw URI)
		{
			name:       "remove_from_card happy",
			args:       labelArgs{Action: "remove_from_card", CardID: sp("42"), LabelId: sp("7")},
			wantMethod: "DELETE",
			wantPath:   "/api/cards/42/card-labels/labelId:7",
			respond:    func(_ capturedRequest) (int, string) { return 200, successJSON },
		},
		// Validation: missing boardId for get_all
		{
			name:    "get_all missing boardId",
			args:    labelArgs{Action: "get_all"},
			wantErr: "boardId is required",
		},
		// Validation: non-numeric boardId rejected (Layer B requireID)
		{
			name:    "get_all non-numeric boardId",
			args:    labelArgs{Action: "get_all", BoardID: sp("not-a-number")},
			wantErr: "boardId must be a numeric Planka ID",
		},
		// Validation: missing id for delete
		{
			name:    "delete missing id",
			args:    labelArgs{Action: "delete"},
			wantErr: "id is required",
		},
		// Validation: create missing position
		{
			name:    "create missing position",
			args:    labelArgs{Action: "create", BoardID: sp("10"), Name: sp("Bug"), Color: sp("coral-green")},
			wantErr: "boardId, name, color, and position are required",
		},
		// Validation: add_to_card missing labelId
		{
			name:    "add_to_card missing labelId",
			args:    labelArgs{Action: "add_to_card", CardID: sp("42")},
			wantErr: "labelId is required",
		},
		// Unknown action
		{
			name:    "unknown action",
			args:    labelArgs{Action: "frobnicate"},
			wantErr: "unknown action: frobnicate",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &mockPlanka{respond: tc.respond}
			client := newToolClient(t, m)

			_, err := dispatchLabel(context.Background(), client, tc.args)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error %q, got %q", tc.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			reqs := m.reqs()
			if len(reqs) == 0 {
				t.Fatal("no requests captured")
			}
			req := reqs[len(reqs)-1]

			if tc.wantMethod != "" && req.Method != tc.wantMethod {
				t.Errorf("method: want %s, got %s", tc.wantMethod, req.Method)
			}
			if tc.wantPath != "" && req.Path != tc.wantPath {
				t.Errorf("path: want %s, got %s", tc.wantPath, req.Path)
			}
			if tc.wantBody != "" && !strings.Contains(req.Body, tc.wantBody) {
				t.Errorf("body: want substring %q in %q", tc.wantBody, req.Body)
			}
		})
	}
}

// TestLabelRemoveColonNotEscaped asserts that RemoveLabelFromCard sends the
// literal "labelId:" colon in the raw request URI (not percent-encoded as %3A).
// Planka's path-param format requires the colon verbatim.
func TestLabelRemoveColonNotEscaped(t *testing.T) {
	m := &mockPlanka{respond: func(_ capturedRequest) (int, string) {
		return 200, `{"item":{}}`
	}}
	client := newToolClient(t, m)

	args := labelArgs{Action: "remove_from_card", CardID: sp("42"), LabelId: sp("7")}
	_, err := dispatchLabel(context.Background(), client, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reqs := m.reqs()
	if len(reqs) == 0 {
		t.Fatal("no requests captured")
	}
	raw := reqs[len(reqs)-1].RawURI
	if strings.Contains(raw, "%3A") {
		t.Errorf("colon was percent-encoded in raw URI %q; want literal 'labelId:'", raw)
	}
	if !strings.Contains(raw, "labelId:7") {
		t.Errorf("raw URI %q does not contain literal 'labelId:7'", raw)
	}
}
