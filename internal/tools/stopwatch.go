package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cameronsjo/kanban-mcp/internal/planka"
)

func init() { addRegistrar(registerStopwatch) }

// stopwatchArgs is the flat input for mcp_kanban_stopwatch.
// Per PORT_SPEC: id is REQUIRED (non-pointer) at the schema level; Action is
// also required.
type stopwatchArgs struct {
	Action string `json:"action" jsonschema:"The action to perform"`
	ID     string `json:"id" jsonschema:"The ID of the card"`
}

var stopwatchActions = []string{"start", "stop", "get", "reset"}

func registerStopwatch(server *mcp.Server, client *planka.Client) error {
	schema, err := inferSchema[stopwatchArgs](
		applyEnum("action", stopwatchActions),
		applyIDPattern("id"),
	)
	if err != nil {
		return err
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "mcp_kanban_stopwatch",
		Description: "Manage card stopwatches for time tracking",
		InputSchema: schema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args stopwatchArgs) (*mcp.CallToolResult, any, error) {
		return respond(dispatchStopwatch(ctx, client, args))
	})
	return nil
}

func dispatchStopwatch(ctx context.Context, client *planka.Client, args stopwatchArgs) (any, error) {
	id, err := requireID("id", &args.ID)
	if err != nil {
		return nil, err
	}

	switch args.Action {
	case "start":
		return client.StartCardStopwatch(ctx, id)

	case "stop":
		return client.StopCardStopwatch(ctx, id)

	case "get":
		return client.GetCardStopwatch(ctx, id)

	case "reset":
		return client.ResetCardStopwatch(ctx, id)

	default:
		return nil, fmt.Errorf("unknown action: %s", args.Action)
	}
}
