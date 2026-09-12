package tools

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/cameronsjo/kanban-mcp/internal/config"
	"github.com/cameronsjo/kanban-mcp/internal/planka"
)

// capturedRequest records one non-login request hitting the mock Planka.
type capturedRequest struct {
	Method string
	Path   string // r.URL.Path (decoded)
	RawURI string // r.RequestURI (literal, e.g. keeps "labelId:5")
	Body   string
}

// mockPlanka is the shared httptest stand-in for the Planka API used by every
// resource dispatch test. Login is auto-answered; non-login requests are routed
// to respond. Resource tests must REUSE this type (do not redefine it — one
// definition per test package).
type mockPlanka struct {
	mu       sync.Mutex
	requests []capturedRequest
	// respond returns (status, jsonBody) for a recorded non-login request.
	respond func(req capturedRequest) (status int, body string)
}

func (m *mockPlanka) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if r.Method == http.MethodPost && r.URL.Path == "/api/access-tokens" {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"item":"test-token"}`)
		return
	}

	body, _ := io.ReadAll(r.Body)
	rec := capturedRequest{
		Method: r.Method,
		Path:   r.URL.Path,
		RawURI: r.RequestURI,
		Body:   string(body),
	}
	m.requests = append(m.requests, rec)

	status, respBody := 200, `{"item":{}}`
	if m.respond != nil {
		status, respBody = m.respond(rec)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprint(w, respBody)
}

// reqs returns a copy of the recorded requests.
func (m *mockPlanka) reqs() []capturedRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]capturedRequest, len(m.requests))
	copy(out, m.requests)
	return out
}

// newToolClient builds a planka.Client pointed at the mock.
func newToolClient(t *testing.T, m *mockPlanka) *planka.Client {
	t.Helper()
	srv := httptest.NewServer(m)
	t.Cleanup(srv.Close)
	cfg := config.Config{BaseURL: srv.URL, AgentEmail: "a", AgentPassword: "b"}
	c := planka.NewClient(cfg, "test")
	c.SetHTTPClient(srv.Client())
	return c
}

// Pointer helpers for building flat action-arg structs in tests.
func sp(s string) *string   { return &s }
func fp(f float64) *float64 { return &f }
func bp(b bool) *bool       { return &b }
