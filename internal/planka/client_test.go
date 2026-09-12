package planka

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/cameronsjo/kanban-mcp/internal/config"
)

// mockPlanka is a configurable httptest stand-in for the Planka API.
type mockPlanka struct {
	mu sync.Mutex

	loginCount int
	tokens     []string // tokens handed out by successive logins
	tosGate    bool     // when true, login returns 403 {step:"accept-terms"}

	requests []recordedRequest

	// handler is invoked for non-login requests. It receives the bearer token
	// and the recorded request, and returns (status, jsonBody).
	handler func(token string, r recordedRequest) (int, string)
}

type recordedRequest struct {
	Method     string
	Path       string // decoded URL path
	RequestURI string // raw request target (colons etc. unescaped)
	Auth       string
}

func (m *mockPlanka) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if r.Method == http.MethodPost && r.URL.Path == "/api/access-tokens" {
		if m.tosGate {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"code":"E_AGENT_TERMS","step":"accept-terms"}`)
			return
		}
		idx := m.loginCount
		m.loginCount++
		tok := "tok-default"
		if idx < len(m.tokens) {
			tok = m.tokens[idx]
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"item":%q}`, tok)
		return
	}

	rec := recordedRequest{
		Method:     r.Method,
		Path:       r.URL.Path,
		RequestURI: r.RequestURI,
		Auth:       r.Header.Get("Authorization"),
	}
	m.requests = append(m.requests, rec)

	status, body := 200, `{"item":{}}`
	if m.handler != nil {
		status, body = m.handler(r.Header.Get("Authorization"), rec)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprint(w, body)
}

func newTestClient(t *testing.T, m *mockPlanka) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(m)
	t.Cleanup(srv.Close)
	cfg := config.Config{
		BaseURL:       srv.URL,
		AgentEmail:    "agent@example.com",
		AgentPassword: "secret",
	}
	c := NewClient(cfg, "test")
	c.SetHTTPClient(srv.Client())
	return c, srv
}

func TestDoHappyPathSendsBearerAndDecodes(t *testing.T) {
	m := &mockPlanka{
		tokens: []string{"tok-1"},
		handler: func(token string, r recordedRequest) (int, string) {
			if token != "Bearer tok-1" {
				t.Errorf("auth header = %q, want Bearer tok-1", token)
			}
			if r.Path != "/api/cards/42" {
				t.Errorf("path = %q, want /api/cards/42", r.Path)
			}
			return 200, `{"item":{"id":"42","name":"hi"}}`
		},
	}
	c, _ := newTestClient(t, m)

	var env itemEnvelope[Card]
	if err := c.Get(context.Background(), "/api/cards/42", &env); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if env.Item.ID != "42" || env.Item.Name != "hi" {
		t.Fatalf("decoded card = %+v", env.Item)
	}
	if m.loginCount != 1 {
		t.Fatalf("loginCount = %d, want 1", m.loginCount)
	}
}

func TestTokenRefreshOn401RetriesExactlyOnce(t *testing.T) {
	m := &mockPlanka{
		tokens: []string{"stale", "fresh"},
		handler: func(token string, r recordedRequest) (int, string) {
			if token == "Bearer stale" {
				return 401, `{"message":"jwt expired"}`
			}
			return 200, `{"item":{"id":"7"}}`
		},
	}
	c, _ := newTestClient(t, m)

	var env itemEnvelope[Card]
	if err := c.Get(context.Background(), "/api/cards/7", &env); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if env.Item.ID != "7" {
		t.Fatalf("decoded id = %q, want 7", env.Item.ID)
	}
	// One login to mint "stale", one more after the 401 to mint "fresh".
	if m.loginCount != 2 {
		t.Fatalf("loginCount = %d, want 2 (initial + one refresh)", m.loginCount)
	}
	// Exactly two data requests: the 401 and the successful retry.
	if len(m.requests) != 2 {
		t.Fatalf("data requests = %d, want 2", len(m.requests))
	}
}

func TestPersistent401ReturnsAuthErrorNoInfiniteRetry(t *testing.T) {
	m := &mockPlanka{
		tokens: []string{"a", "b", "c"},
		handler: func(token string, r recordedRequest) (int, string) {
			return 401, `{"message":"nope"}`
		},
	}
	c, _ := newTestClient(t, m)

	err := c.Get(context.Background(), "/api/cards/1", nil)
	if err == nil {
		t.Fatal("expected an error on persistent 401")
	}
	apiErr, ok := AsAPIError(err)
	if !ok || apiErr.Status != 401 || apiErr.Kind != KindAuth {
		t.Fatalf("err = %v, want APIError 401/auth", err)
	}
	// Two data attempts max (initial + single retry), not an infinite loop.
	if len(m.requests) != 2 {
		t.Fatalf("data requests = %d, want 2", len(m.requests))
	}
}

func TestConcurrentLoginsCollapseToOne(t *testing.T) {
	m := &mockPlanka{
		tokens: []string{"only"},
		handler: func(token string, r recordedRequest) (int, string) {
			return 200, `{"item":{"id":"1"}}`
		},
	}
	c, _ := newTestClient(t, m)

	const n = 24
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_ = c.Get(context.Background(), "/api/cards/1", nil)
		}()
	}
	wg.Wait()

	if m.loginCount != 1 {
		t.Fatalf("loginCount = %d, want 1 (singleflight should collapse)", m.loginCount)
	}
}

func TestToSGateReturnsTermsNotAccepted(t *testing.T) {
	m := &mockPlanka{tosGate: true}
	c, _ := newTestClient(t, m)

	err := c.Get(context.Background(), "/api/cards/1", nil)
	if !errors.Is(err, ErrTermsNotAccepted) {
		t.Fatalf("err = %v, want ErrTermsNotAccepted", err)
	}
}

func TestErrorMapping(t *testing.T) {
	cases := []struct {
		status int
		body   string
		kind   ErrorKind
	}{
		{404, `{"message":"missing"}`, KindNotFound},
		{422, `{"message":"bad input"}`, KindValidation},
		{409, `{"message":"dup"}`, KindConflict},
		{403, `{"message":"forbidden"}`, KindPermission},
		{500, `{"message":"boom"}`, KindAPI},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("status_%d", tc.status), func(t *testing.T) {
			m := &mockPlanka{
				tokens: []string{"t"},
				handler: func(token string, r recordedRequest) (int, string) {
					return tc.status, tc.body
				},
			}
			c, _ := newTestClient(t, m)
			err := c.Get(context.Background(), "/api/cards/1", nil)
			apiErr, ok := AsAPIError(err)
			if !ok {
				t.Fatalf("err = %v, want *APIError", err)
			}
			if apiErr.Status != tc.status || apiErr.Kind != tc.kind {
				t.Fatalf("got %d/%s, want %d/%s", apiErr.Status, apiErr.Kind, tc.status, tc.kind)
			}
		})
	}

	// IsNotFound helper.
	m := &mockPlanka{tokens: []string{"t"}, handler: func(string, recordedRequest) (int, string) {
		return 404, `{"message":"x"}`
	}}
	c, _ := newTestClient(t, m)
	if err := c.Get(context.Background(), "/api/cards/1", nil); !IsNotFound(err) {
		t.Fatalf("IsNotFound(%v) = false, want true", err)
	}
}

// TestLabelRemoveColonNotEscaped locks the v2.1 quirk that the card-label remove
// path uses a literal "labelId:" prefix that must NOT be percent-encoded.
func TestLabelRemoveColonNotEscaped(t *testing.T) {
	m := &mockPlanka{
		tokens: []string{"t"},
		handler: func(token string, r recordedRequest) (int, string) {
			return 200, `{}`
		},
	}
	c, _ := newTestClient(t, m)

	if err := c.Delete(context.Background(), "/api/cards/9/card-labels/labelId:5"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(m.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(m.requests))
	}
	got := m.requests[0]
	want := "/api/cards/9/card-labels/labelId:5"
	if got.Path != want {
		t.Errorf("decoded path = %q, want %q", got.Path, want)
	}
	if got.RequestURI != want {
		t.Errorf("raw request-uri = %q, want %q (colon must stay literal)", got.RequestURI, want)
	}
}
