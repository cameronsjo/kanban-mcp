package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cameronsjo/kanban-mcp/internal/planka"
)

func init() { addRegistrar(registerLabelManager) }

// labelArgs is the flat, action-discriminated input for mcp_kanban_label_manager.
// Every field except action is an optional pointer; per-action requiredness is
// enforced in the handler (Layer B).
type labelArgs struct {
	Action   string   `json:"action" jsonschema:"The action to perform"`
	ID       *string  `json:"id,omitempty" jsonschema:"The ID of the label"`
	BoardID  *string  `json:"boardId,omitempty" jsonschema:"The ID of the board"`
	CardID   *string  `json:"cardId,omitempty" jsonschema:"The ID of the card"`
	LabelId  *string  `json:"labelId,omitempty" jsonschema:"The ID of the label (for card operations)"`
	Name     *string  `json:"name,omitempty" jsonschema:"The name of the label"`
	Color    *string  `json:"color,omitempty" jsonschema:"The color of the label"`
	Position *float64 `json:"position,omitempty" jsonschema:"The position of the label"`
}

var labelActions = []string{"get_all", "create", "update", "delete", "add_to_card", "remove_from_card"}

func registerLabelManager(server *mcp.Server, client *planka.Client) error {
	schema, err := inferSchema[labelArgs](
		applyEnum("action", labelActions),
		applyIDPattern("id", "boardId", "cardId"),
		applyEnum("color", labelColors),
	)
	if err != nil {
		return err
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "mcp_kanban_label_manager",
		Description: "Manage kanban labels with various operations",
		InputSchema: schema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args labelArgs) (*mcp.CallToolResult, any, error) {
		return respond(dispatchLabel(ctx, client, args))
	})
	return nil
}

func dispatchLabel(ctx context.Context, client *planka.Client, args labelArgs) (any, error) {
	switch args.Action {
	case "get_all":
		boardID, err := requireID("boardId", args.BoardID)
		if err != nil {
			return nil, err
		}
		return client.GetLabels(ctx, boardID)

	case "create":
		boardID, err := requireID("boardId", args.BoardID)
		if err != nil {
			return nil, err
		}
		name, err := requireName("name", args.Name)
		if err != nil {
			return nil, err
		}
		color, err := requireString("color", args.Color)
		if err != nil {
			return nil, err
		}
		if args.Position == nil {
			return nil, fmt.Errorf("boardId, name, color, and position are required for create action")
		}
		return client.CreateLabel(ctx, planka.CreateLabelOptions{
			BoardID:  boardID,
			Name:     name,
			Color:    color,
			Position: *args.Position,
		})

	case "update":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		name, err := requireName("name", args.Name)
		if err != nil {
			return nil, err
		}
		color, err := requireString("color", args.Color)
		if err != nil {
			return nil, err
		}
		if args.Position == nil {
			return nil, fmt.Errorf("id, name, color, and position are required for update action")
		}
		return client.UpdateLabel(ctx, id, planka.UpdateLabelOptions{
			Name:     &name,
			Color:    &color,
			Position: args.Position,
		})

	case "delete":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		return client.DeleteLabel(ctx, id)

	case "add_to_card":
		cardID, err := requireID("cardId", args.CardID)
		if err != nil {
			return nil, err
		}
		labelID, err := requireString("labelId", args.LabelId)
		if err != nil {
			return nil, err
		}
		return client.AddLabelToCard(ctx, cardID, labelID)

	case "remove_from_card":
		cardID, err := requireID("cardId", args.CardID)
		if err != nil {
			return nil, err
		}
		labelID, err := requireString("labelId", args.LabelId)
		if err != nil {
			return nil, err
		}
		return client.RemoveLabelFromCard(ctx, cardID, labelID)

	default:
		return nil, fmt.Errorf("unknown action: %s", args.Action)
	}
}
