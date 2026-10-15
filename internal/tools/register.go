package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cameronsjo/kanban-mcp/internal/planka"
)

// managerRegistrars is populated by each resource file's init() via addRegistrar.
// Self-registration keeps RegisterAll from being a merge point that every
// resource file must edit (so the eight ports never collide on one file).
var managerRegistrars []func(*mcp.Server, *planka.Client) error

func addRegistrar(r func(*mcp.Server, *planka.Client) error) {
	managerRegistrars = append(managerRegistrars, r)
}

// RegisterAll registers every manager tool on the server, each bound to the one
// shared Planka client. Registration order does not affect behavior (the MCP
// client keys tools by name).
func RegisterAll(server *mcp.Server, client *planka.Client) error {
	for _, r := range managerRegistrars {
		if err := r(server, client); err != nil {
			return err
		}
	}
	return nil
}
