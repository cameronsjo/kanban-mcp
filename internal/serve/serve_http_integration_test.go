package serve_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cameronsjo/kanban-mcp/internal/config"
	"github.com/cameronsjo/kanban-mcp/internal/planka"
	"github.com/cameronsjo/kanban-mcp/internal/serve"
	"github.com/cameronsjo/kanban-mcp/internal/tools"
)

func newRegisteredServer(t *testing.T) *mcp.Server {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "planka-mcp-server", Version: "test"}, nil)
	client := planka.NewClient(config.Config{AgentEmail: "a", AgentPassword: "b"}, "test")
	if err := tools.RegisterAll(server, client); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}
	return server
}

// TestStreamableHTTPListsTools drives the real Streamable-HTTP handler with the
// SDK client and confirms all eight managers list — the end-to-end proof of the
// shared-server HTTP transport.
func TestStreamableHTTPListsTools(t *testing.T) {
	ctx := context.Background()
	ts := httptest.NewServer(serve.Handler(newRegisteredServer(t), ""))
	defer ts.Close()

	mc := mcp.NewClient(&mcp.Implementation{Name: "tester", Version: "test"}, nil)
	sess, err := mc.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             ts.URL + serve.MCPPath,
		HTTPClient:           ts.Client(),
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })

	res, err := sess.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(res.Tools) != 8 {
		t.Fatalf("listed %d tools over HTTP, want 8", len(res.Tools))
	}
}

func TestAuthenticatedHealthzOverHTTP(t *testing.T) {
	ts := httptest.NewServer(serve.Handler(newRegisteredServer(t), "s3cret"))
	defer ts.Close()
	for _, header := range []string{"", "Bearer wrong", "Bearer s3cret"} {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/healthz", nil)
		req.Header.Set("Authorization", header)
		resp, err := ts.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		want := http.StatusUnauthorized
		if header == "Bearer s3cret" {
			want = http.StatusOK
		}
		if resp.StatusCode != want {
			t.Fatalf("healthz=%d want=%d body=%q", resp.StatusCode, want, body)
		}
	}
}

type bearerTransport struct{ next http.RoundTripper }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	cloned := r.Clone(r.Context())
	cloned.Header.Set("Authorization", "Bearer s3cret")
	return b.next.RoundTrip(cloned)
}
func TestAuthenticatedMCPListsTools(t *testing.T) {
	ts := httptest.NewServer(serve.Handler(newRegisteredServer(t), "s3cret"))
	defer ts.Close()
	for _, header := range []string{"", "Bearer wrong"} {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+serve.MCPPath, nil)
		req.Header.Set("Authorization", header)
		resp, err := ts.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("unauthenticated MCP status=%d", resp.StatusCode)
		}
	}
	client := ts.Client()
	client.Transport = bearerTransport{next: client.Transport}
	mc := mcp.NewClient(&mcp.Implementation{Name: "tester", Version: "test"}, nil)
	sess, err := mc.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: ts.URL + serve.MCPPath, HTTPClient: client, DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	res, err := sess.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Tools) != 8 {
		t.Fatalf("listed %d tools, want8", len(res.Tools))
	}
}
