package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cameronsjo/kanban-mcp/internal/planka"
)

func init() { addRegistrar(registerCommentManager) }

// commentArgs is the flat, action-discriminated input for mcp_kanban_comment_manager.
// Every field except action is an optional pointer; per-action requiredness is
// enforced in the handler (Layer B).
type commentArgs struct {
	Action string  `json:"action" jsonschema:"The action to perform"`
	ID     *string `json:"id,omitempty" jsonschema:"The ID of the comment"`
	CardID *string `json:"cardId,omitempty" jsonschema:"The ID of the card"`
	Text   *string `json:"text,omitempty" jsonschema:"The text content of the comment"`
}

var commentActions = []string{"get_all", "create", "get_one", "update", "delete"}

func registerCommentManager(server *mcp.Server, client *planka.Client) error {
	schema, err := inferSchema[commentArgs](
		applyEnum("action", commentActions),
		applyIDPattern("id", "cardId"),
	)
	if err != nil {
		return err
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "mcp_kanban_comment_manager",
		Description: "Manage card comments with various operations",
		InputSchema: schema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args commentArgs) (*mcp.CallToolResult, any, error) {
		return respond(dispatchComment(ctx, client, args))
	})
	return nil
}

func dispatchComment(ctx context.Context, client *planka.Client, args commentArgs) (any, error) {
	switch args.Action {
	case "get_all":
		cardID, err := requireID("cardId", args.CardID)
		if err != nil {
			return nil, err
		}
		return client.GetComments(ctx, cardID)

	case "create":
		cardID, err := requireID("cardId", args.CardID)
		if err != nil {
			return nil, err
		}
		text, err := requireString("text", args.Text)
		if err != nil {
			return nil, err
		}
		return client.CreateComment(ctx, cardID, text)

	case "get_one":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		return client.GetComment(ctx, id)

	case "update":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		text, err := requireString("text", args.Text)
		if err != nil {
			return nil, err
		}
		return client.UpdateComment(ctx, id, text)

	case "delete":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		return client.DeleteComment(ctx, id)

	default:
		return nil, fmt.Errorf("unknown action: %s", args.Action)
	}
}
