package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cameronsjo/kanban-mcp/internal/planka"
)

func init() { addRegistrar(registerCardManager) }

// cardArgs is the flat, action-discriminated input for mcp_kanban_card_manager.
type cardArgs struct {
	Action      string    `json:"action" jsonschema:"The action to perform"`
	ID          *string   `json:"id,omitempty" jsonschema:"The ID of the card"`
	ListId      *string   `json:"listId,omitempty" jsonschema:"The ID of the list"`
	BoardID     *string   `json:"boardId,omitempty" jsonschema:"The ID of the board (if moving between boards)"`
	ProjectID   *string   `json:"projectId,omitempty" jsonschema:"The ID of the project (if moving between projects)"`
	Name        *string   `json:"name,omitempty" jsonschema:"The name of the card"`
	Description *string   `json:"description,omitempty" jsonschema:"The description of the card"`
	Position    *float64  `json:"position,omitempty" jsonschema:"The position of the card"`
	DueDate     *string   `json:"dueDate,omitempty" jsonschema:"The due date for the card (ISO format)"`
	IsCompleted *bool     `json:"isCompleted,omitempty" jsonschema:"Whether the card is completed"`
	Tasks       *[]string `json:"tasks,omitempty" jsonschema:"Array of task descriptions to create for create_with_tasks action"`
	Comment     *string   `json:"comment,omitempty" jsonschema:"Optional comment to add to the card"`
	CardID      *string   `json:"cardId,omitempty" jsonschema:"The ID of the card to get details for"`
}

var cardActions = []string{
	"get_all", "create", "get_one", "update", "move",
	"duplicate", "delete", "create_with_tasks", "get_details",
}

func registerCardManager(server *mcp.Server, client *planka.Client) error {
	schema, err := inferSchema[cardArgs](
		applyEnum("action", cardActions),
		applyIDPattern("id", "listId"),
	)
	if err != nil {
		return err
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "mcp_kanban_card_manager",
		Description: "Manage kanban cards with various operations",
		InputSchema: schema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args cardArgs) (*mcp.CallToolResult, any, error) {
		return respond(dispatchCard(ctx, client, args))
	})
	return nil
}

func dispatchCard(ctx context.Context, client *planka.Client, args cardArgs) (any, error) {
	switch args.Action {
	case "get_all":
		listID, err := requireID("listId", args.ListId)
		if err != nil {
			return nil, err
		}
		return client.GetCards(ctx, listID)

	case "create":
		listID, err := requireID("listId", args.ListId)
		if err != nil {
			return nil, err
		}
		name, err := requireName("name", args.Name)
		if err != nil {
			return nil, err
		}
		return client.CreateCard(ctx, planka.CreateCardOptions{
			ListID:      listID,
			Name:        name,
			Description: deref(args.Description, ""),
			Position:    deref(args.Position, 0),
			Type:        "",
		})

	case "get_one":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		return client.GetCard(ctx, id)

	case "update":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		// nil pointers mean "leave unchanged" and propagate as-is.
		return client.UpdateCard(ctx, id, planka.UpdateCardOptions{
			Name:        args.Name,
			Description: args.Description,
			Position:    args.Position,
			DueDate:     args.DueDate,
			IsCompleted: args.IsCompleted,
		})

	case "move":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		listID, err := requireID("listId", args.ListId)
		if err != nil {
			return nil, err
		}
		pos, err := requireFloat("position", args.Position)
		if err != nil {
			return nil, err
		}
		return client.MoveCard(ctx, id, listID, pos, args.BoardID, args.ProjectID)

	case "duplicate":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		pos, err := requireFloat("position", args.Position)
		if err != nil {
			return nil, err
		}
		return client.DuplicateCard(ctx, id, pos)

	case "delete":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		return client.DeleteCard(ctx, id)

	case "create_with_tasks":
		listID, err := requireID("listId", args.ListId)
		if err != nil {
			return nil, err
		}
		name, err := requireName("name", args.Name)
		if err != nil {
			return nil, err
		}
		var tasks []string
		if args.Tasks != nil {
			tasks = *args.Tasks
		}
		return createCardWithTasks(ctx, client, CreateCardWithTasksParams{
			ListID:      listID,
			Name:        name,
			Description: args.Description,
			Tasks:       tasks,
			Comment:     args.Comment,
			Position:    args.Position,
		})

	case "get_details":
		cardID, err := requireID("cardId", args.CardID)
		if err != nil {
			return nil, err
		}
		return cardDetails(ctx, client, cardID)

	default:
		return nil, fmt.Errorf("unknown action: %s", args.Action)
	}
}
