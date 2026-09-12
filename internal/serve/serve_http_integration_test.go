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

func TestHealthzOverHTTP(t *testing.T) {
	ts := httptest.NewServer(serve.Handler(newRegisteredServer(t), "s3cret"))
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Fatalf("/healthz = %d %q, want 200 ok (must be open even with a bearer token set)", resp.StatusCode, string(body))
	}
}
