package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cameronsjo/kanban-mcp/internal/planka"
)

func init() { addRegistrar(registerListManager) }

// listArgs is the flat, action-discriminated input for mcp_kanban_list_manager.
// Every field except action is an optional pointer so the schema marks only
// action required; per-action requiredness is enforced in the handler (Layer B).
type listArgs struct {
	Action   string   `json:"action" jsonschema:"The action to perform"`
	ID       *string  `json:"id,omitempty" jsonschema:"The ID of the list"`
	BoardID  *string  `json:"boardId,omitempty" jsonschema:"The ID of the board"`
	Name     *string  `json:"name,omitempty" jsonschema:"The name of the list"`
	Position *float64 `json:"position,omitempty" jsonschema:"The position of the list"`
	Type     *string  `json:"type,omitempty" jsonschema:"List type (active|closed; default active)"`
}

var listActions = []string{"get_all", "create", "update", "delete", "get_one"}

func registerListManager(server *mcp.Server, client *planka.Client) error {
	schema, err := inferSchema[listArgs](
		applyEnum("action", listActions),
		applyIDPattern("id", "boardId"),
		applyEnum("type", listTypes),
	)
	if err != nil {
		return err
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "mcp_kanban_list_manager",
		Description: "Manage kanban lists with various operations",
		InputSchema: schema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listArgs) (*mcp.CallToolResult, any, error) {
		return respond(dispatchList(ctx, client, args))
	})
	return nil
}

func dispatchList(ctx context.Context, client *planka.Client, args listArgs) (any, error) {
	switch args.Action {
	case "get_all":
		boardID, err := requireID("boardId", args.BoardID)
		if err != nil {
			return nil, err
		}
		return client.GetLists(ctx, boardID)

	case "get_one":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		return client.GetList(ctx, id)

	case "create":
		boardID, err := requireID("boardId", args.BoardID)
		if err != nil {
			return nil, err
		}
		name, err := requireName("name", args.Name)
		if err != nil {
			return nil, err
		}
		if args.Position == nil {
			return nil, fmt.Errorf("boardId, name, and position are required for create action")
		}
		opts := planka.CreateListOptions{BoardID: boardID, Name: name, Position: *args.Position}
		if args.Type != nil {
			opts.Type = *args.Type
		}
		return client.CreateList(ctx, opts)

	case "update":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		name, err := requireName("name", args.Name)
		if err != nil {
			return nil, err
		}
		if args.Position == nil {
			return nil, fmt.Errorf("id, name, and position are required for update action")
		}
		return client.UpdateList(ctx, id, planka.UpdateListOptions{Name: &name, Position: args.Position})

	case "delete":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		return client.DeleteList(ctx, id)

	default:
		return nil, fmt.Errorf("unknown action: %s", args.Action)
	}
}
