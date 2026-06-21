// Command kanban-mcp is a Model Context Protocol server for Planka kanban
// boards. It runs as a single long-lived Streamable-HTTP daemon that every
// Claude Code session shares (default), or over stdio for local debugging.
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	transport := flag.String("transport", "stdio", "transport: stdio|http")
	addr := flag.String("addr", "127.0.0.1:8900", "http listen address (http transport)")
	flag.Parse()
	_ = addr

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "planka-mcp-server",
		Version: version,
	}, nil)

	// Placeholder hello tool — replaced by tools.RegisterAll in step 2+.
	type helloArgs struct {
		Name string `json:"name" jsonschema:"the person to greet"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "hello",
		Description: "say hi (scaffold smoke-test tool)",
	}, func(_ context.Context, _ *mcp.CallToolRequest, args helloArgs) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Hi " + args.Name}},
		}, nil, nil
	})

	switch *transport {
	case "stdio":
		if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			log.Fatalf("kanban-mcp: %v", err)
		}
	default:
		log.Fatalf("kanban-mcp: unsupported transport %q (http arrives in step 6)", *transport)
		os.Exit(2)
	}
}
