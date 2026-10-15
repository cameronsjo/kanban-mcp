package planka

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
)

func TestAddLabelToCardAcceptsEmptySuccessResponse(t *testing.T) {
	m := &mockPlanka{
		tokens: []string{"tok"},
		handler: func(token string, r recordedRequest) (int, string) {
			if r.Method != "POST" || r.Path != "/api/cards/C1/card-labels" {
				t.Fatalf("request = %s %s, want POST /api/cards/C1/card-labels", r.Method, r.Path)
			}
			return 204, ""
		},
	}
	c, _ := newTestClient(t, m)

	got, err := c.AddLabelToCard(context.Background(), "C1", "L1")
	if err != nil {
		t.Fatalf("AddLabelToCard: %v", err)
	}
	if !got["success"] {
		t.Fatalf("result = %#v, want success=true", got)
	}
}

func TestGetCommentsAcceptsBareArrayResponse(t *testing.T) {
	m := &mockPlanka{
		tokens: []string{"tok"},
		handler: func(token string, r recordedRequest) (int, string) {
			if r.Method != "GET" || r.Path != "/api/cards/C1/comments" {
				t.Fatalf("request = %s %s, want GET /api/cards/C1/comments", r.Method, r.Path)
			}
			return 200, `[{"id":"CMT1","cardId":"C1","text":"hello"}]`
		},
	}
	c, _ := newTestClient(t, m)

	comments, err := c.GetComments(context.Background(), "C1")
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	if len(comments) != 1 || comments[0].ID != "CMT1" || comments[0].Text != "hello" {
		t.Fatalf("comments = %#v, want one CMT1 comment", comments)
	}
}

func TestGetCommentsNormalizesEmptyResponsesToEmptyArray(t *testing.T) {
	for _, body := range []string{`{}`, `{"items":null}`, `[]`} {
		t.Run(body, func(t *testing.T) {
			m := &mockPlanka{
				tokens: []string{"tok"},
				handler: func(token string, r recordedRequest) (int, string) {
					return 200, body
				},
			}
			c, _ := newTestClient(t, m)

			comments, err := c.GetComments(context.Background(), "C1")
			if err != nil {
				t.Fatalf("GetComments: %v", err)
			}
			wire, err := json.Marshal(comments)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}
			if string(wire) != "[]" {
				t.Fatalf("comments JSON = %s, want []", wire)
			}
		})
	}
}

func TestGetCommentPropagatesBoardAndCommentsErrors(t *testing.T) {
	cases := []struct {
		name     string
		failPath string
	}{
		{name: "board", failPath: "/api/boards/B1"},
		{name: "comments", failPath: "/api/cards/C1/comments"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &mockPlanka{
				tokens: []string{"tok"},
				handler: func(token string, r recordedRequest) (int, string) {
					switch r.Path {
					case "/api/projects":
						return 200, `{"items":[],"included":{"boards":[{"id":"B1"}]}}`
					case "/api/boards/B1":
						if r.Path == tc.failPath {
							return 500, `{"message":"board lookup failed"}`
						}
						return 200, `{"item":{"id":"B1"},"included":{"cards":[{"id":"C1"}]}}`
					case "/api/cards/C1/comments":
						if r.Path == tc.failPath {
							return 500, `{"message":"comments lookup failed"}`
						}
					}
					t.Fatalf("unexpected request: %s %s", r.Method, r.Path)
					return 404, `{"message":"unexpected"}`
				},
			}
			c, _ := newTestClient(t, m)

			_, err := c.GetComment(context.Background(), "missing")
			if err == nil {
				t.Fatal("GetComment returned nil error")
			}
			apiErr, ok := AsAPIError(err)
			if !ok || apiErr.Status != 500 {
				t.Fatalf("err = %v, want APIError status 500", err)
			}
		})
	}
}

func TestGetCardStopwatchStoppedIncludesNullStartedAt(t *testing.T) {
	m := &mockPlanka{
		tokens: []string{"tok"},
		handler: func(token string, r recordedRequest) (int, string) {
			if r.Method != "GET" || r.Path != "/api/cards/C1" {
				t.Fatalf("request = %s %s, want GET /api/cards/C1", r.Method, r.Path)
			}
			return 200, `{"item":{"id":"C1","stopwatch":{"startedAt":null,"total":60}}}`
		},
	}
	c, _ := newTestClient(t, m)

	result, err := c.GetCardStopwatch(context.Background(), "C1")
	if err != nil {
		t.Fatalf("GetCardStopwatch: %v", err)
	}
	got, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	if got["isRunning"] != false || got["total"] != float64(60) {
		t.Fatalf("result = %#v, want stopped total=60", got)
	}
	startedAt, present := got["startedAt"]
	if !present || startedAt != nil {
		t.Fatalf("startedAt = (%#v, present=%v), want present nil", startedAt, present)
	}
}

func TestGetListsNormalizesMissingOrNullListsToEmptyArray(t *testing.T) {
	for _, body := range []string{
		`{"item":{"id":"B1"},"included":{}}`,
		`{"item":{"id":"B1"},"included":{"lists":null}}`,
	} {
		t.Run(body, func(t *testing.T) {
			m := &mockPlanka{
				tokens: []string{"tok"},
				handler: func(token string, r recordedRequest) (int, string) {
					return 200, body
				},
			}
			c, _ := newTestClient(t, m)

			lists, err := c.GetLists(context.Background(), "B1")
			if err != nil {
				t.Fatalf("GetLists: %v", err)
			}
			wire, err := json.Marshal(lists)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}
			if string(wire) != "[]" {
				t.Fatalf("lists JSON = %s, want []", wire)
			}
		})
	}
}

func TestCommentMarshalPreservesNullUpdatedAt(t *testing.T) {
	var comment Comment
	if err := json.Unmarshal([]byte(`{"id":"C1","text":"hello","updatedAt":null}`), &comment); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	wire, err := json.Marshal(comment)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(wire, &fields); err != nil {
		t.Fatalf("json.Unmarshal marshaled comment: %v", err)
	}
	updatedAt, present := fields["updatedAt"]
	if !present || updatedAt != nil {
		t.Fatalf("comment JSON = %s, want present updatedAt:null", wire)
	}
}

func TestTaskCardCacheConcurrentReadWrite(t *testing.T) {
	c := &Client{}
	const n = 64
	var wg sync.WaitGroup
	wg.Add(2 * n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start
			c.rememberTaskCard(fmt.Sprintf("T%d", i), fmt.Sprintf("C%d", i))
		}()
		go func() {
			defer wg.Done()
			<-start
			_ = c.taskCardID(fmt.Sprintf("T%d", i))
		}()
	}
	close(start)
	wg.Wait()

	for i := 0; i < n; i++ {
		if got := c.taskCardID(fmt.Sprintf("T%d", i)); got != fmt.Sprintf("C%d", i) {
			t.Fatalf("task T%d maps to %q, want C%d", i, got, i)
		}
	}
}
