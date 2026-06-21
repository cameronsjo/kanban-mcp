// Command kanban-mcp is a Model Context Protocol server for Planka kanban
// boards. It runs as a single long-lived Streamable-HTTP daemon that every
// Claude Code session shares (default in production), or over stdio for local
// debugging and the MCP Inspector.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cameronsjo/kanban-mcp/internal/config"
	"github.com/cameronsjo/kanban-mcp/internal/planka"
	"github.com/cameronsjo/kanban-mcp/internal/serve"
	"github.com/cameronsjo/kanban-mcp/internal/tools"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	transport := flag.String("transport", "stdio", "transport: stdio|http")
	addr := flag.String("addr", config.DefaultAddr, "http listen address (http transport); MUST be loopback")
	flag.Parse()

	cfg := config.FromEnv()
	cfg.Transport = *transport
	cfg.Addr = *addr

	// Credentials are not required to register or list tools — only to invoke
	// them — so a missing-creds startup is a warning, not a fatal (matches the
	// TS server, which surfaces auth errors at call time).
	if err := cfg.Validate(); err != nil {
		log.Printf("kanban-mcp: warning: %v", err)
	}

	// One shared Planka client, reused across every request and session, so the
	// token cache (mutex + singleflight) persists process-wide.
	client := planka.NewClient(cfg, version)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "planka-mcp-server",
		Version: version,
	}, nil)
	if err := tools.RegisterAll(server, client); err != nil {
		log.Fatalf("kanban-mcp: register tools: %v", err)
	}

	switch cfg.Transport {
	case config.TransportStdio:
		if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			log.Fatalf("kanban-mcp: %v", err)
		}
	case config.TransportHTTP:
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := serve.HTTP(ctx, cfg.Addr, cfg.AuthToken, server); err != nil {
			log.Fatalf("kanban-mcp: %v", err)
		}
	default:
		log.Fatalf("kanban-mcp: unsupported transport %q", cfg.Transport)
		os.Exit(2)
	}
}
