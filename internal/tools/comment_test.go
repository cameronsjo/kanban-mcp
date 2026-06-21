package tools

import (
	"context"
	"strings"
	"testing"
)

func TestCommentDispatch(t *testing.T) {
	commentJSON := `{"item":{"id":"5","text":"hello","cardId":"42","createdAt":"2024-01-01T00:00:00Z"}}`
	commentsJSON := `{"items":[{"id":"5","text":"hello","cardId":"42","createdAt":"2024-01-01T00:00:00Z"}]}`
	// Board-walk responses for get_one: projects list → board → card comments.
	projectsJSON := `{"item":{},"included":{"boards":[{"id":"10","projectId":"1","name":"Board","position":1,"createdAt":"2024-01-01T00:00:00Z"}]}}`
	boardJSON := `{"item":{"id":"10","projectId":"1","name":"Board","position":1,"createdAt":"2024-01-01T00:00:00Z"},"included":{"cards":[{"id":"42","listId":"20","name":"Card","position":1,"createdAt":"2024-01-01T00:00:00Z"}]}}`
	successJSON := `{"item":{}}`

	tests := []struct {
		name       string
		args       commentArgs
		wantMethod string
		wantPath   string
		wantBody   string
		wantErr    string
		respond    func(capturedRequest) (int, string)
	}{
		// Happy-path: get_all
		{
			name:       "get_all happy",
			args:       commentArgs{Action: "get_all", CardID: sp("42")},
			wantMethod: "GET",
			wantPath:   "/api/cards/42/comments",
			respond:    func(_ capturedRequest) (int, string) { return 200, commentsJSON },
		},
		// Happy-path: create
		{
			name:       "create happy",
			args:       commentArgs{Action: "create", CardID: sp("42"), Text: sp("hello")},
			wantMethod: "POST",
			wantPath:   "/api/cards/42/comments",
			wantBody:   `"text":"hello"`,
			respond:    func(_ capturedRequest) (int, string) { return 200, commentJSON },
		},
		// Happy-path: get_one (board-walk; final request is the card comments endpoint)
		{
			name: "get_one happy",
			args: commentArgs{Action: "get_one", ID: sp("5")},
			respond: func(req capturedRequest) (int, string) {
				switch {
				case req.Path == "/api/projects":
					return 200, projectsJSON
				case req.Path == "/api/boards/10":
					return 200, boardJSON
				case strings.HasPrefix(req.Path, "/api/cards/") && strings.HasSuffix(req.Path, "/comments"):
					return 200, commentsJSON
				default:
					return 404, `{"error":"not found"}`
				}
			},
			wantPath: "/api/cards/42/comments",
		},
		// Happy-path: update
		{
			name:       "update happy",
			args:       commentArgs{Action: "update", ID: sp("5"), Text: sp("updated")},
			wantMethod: "PATCH",
			wantPath:   "/api/comments/5",
			wantBody:   `"text":"updated"`,
			respond:    func(_ capturedRequest) (int, string) { return 200, commentJSON },
		},
		// Happy-path: delete
		{
			name:       "delete happy",
			args:       commentArgs{Action: "delete", ID: sp("5")},
			wantMethod: "DELETE",
			wantPath:   "/api/comments/5",
			respond:    func(_ capturedRequest) (int, string) { return 200, successJSON },
		},
		// Validation: missing cardId for get_all
		{
			name:    "get_all missing cardId",
			args:    commentArgs{Action: "get_all"},
			wantErr: "cardId is required",
		},
		// Validation: non-numeric id rejected
		{
			name:    "get_all non-numeric cardId",
			args:    commentArgs{Action: "get_all", CardID: sp("abc")},
			wantErr: "cardId must be a numeric Planka ID",
		},
		// Validation: create missing text
		{
			name:    "create missing text",
			args:    commentArgs{Action: "create", CardID: sp("42")},
			wantErr: "text is required",
		},
		// Validation: update missing text
		{
			name:    "update missing text",
			args:    commentArgs{Action: "update", ID: sp("5")},
			wantErr: "text is required",
		},
		// Validation: missing id for delete
		{
			name:    "delete missing id",
			args:    commentArgs{Action: "delete"},
			wantErr: "id is required",
		},
		// Unknown action
		{
			name:    "unknown action",
			args:    commentArgs{Action: "wibble"},
			wantErr: "unknown action: wibble",
		},
		// Quirk: get_one board-walk returns "Comment not found" when comment absent
		{
			name: "get_one not found",
			args: commentArgs{Action: "get_one", ID: sp("999")},
			respond: func(req capturedRequest) (int, string) {
				switch {
				case req.Path == "/api/projects":
					return 200, projectsJSON
				case req.Path == "/api/boards/10":
					return 200, boardJSON
				case strings.HasPrefix(req.Path, "/api/cards/") && strings.HasSuffix(req.Path, "/comments"):
					return 200, commentsJSON // comment id "5" not "999"
				default:
					return 404, `{"error":"not found"}`
				}
			},
			wantErr: "Comment not found: 999",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &mockPlanka{respond: tc.respond}
			client := newToolClient(t, m)

			_, err := dispatchComment(context.Background(), client, tc.args)

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
			// For multi-request operations (board-walk), check the final request.
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
