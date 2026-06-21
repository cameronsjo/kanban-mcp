// Package serve hosts the kanban-mcp server over its two transports. The HTTP
// transport is the load-bearing one: a single long-lived process bound to
// loopback that every Claude Code session shares, so the one planka.Client's
// token cache persists process-wide.
package serve

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPPath is the endpoint Streamable-HTTP clients connect to.
const MCPPath = "/mcp"

// BearerGate wraps next behind a static "Authorization: Bearer <token>" check.
// It is a defense-in-depth layer on top of the loopback bind; when token is
// empty the caller should not install it.
func BearerGate(token string, next http.Handler) http.Handler {
	want := "Bearer " + token
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != want {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Handler builds the HTTP mux: the shared MCP server at /mcp (optionally behind
// the bearer gate) and an open /healthz. getServer returns the same server for
// every session — that is what makes the token cache shared.
func Handler(server *mcp.Server, authToken string) http.Handler {
	streamable := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, nil)

	var mcpHandler http.Handler = streamable
	if authToken != "" {
		mcpHandler = BearerGate(authToken, mcpHandler)
	}

	mux := http.NewServeMux()
	mux.Handle(MCPPath, mcpHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
	return mux
}

// HTTP serves the MCP server over Streamable HTTP on addr until ctx is canceled,
// then shuts down gracefully. addr MUST be a loopback address — the process
// holds Planka admin credentials and must never listen on a routable interface.
func HTTP(ctx context.Context, addr, authToken string, server *mcp.Server) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           Handler(server, authToken),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("kanban-mcp: serving MCP on http://%s%s", addr, MCPPath)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
