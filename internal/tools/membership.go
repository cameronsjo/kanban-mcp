package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cameronsjo/kanban-mcp/internal/planka"
)

func init() { addRegistrar(registerMembershipManager) }

// membershipArgs is the flat, action-discriminated input for
// mcp_kanban_membership_manager.
type membershipArgs struct {
	Action     string  `json:"action" jsonschema:"The action to perform"`
	ID         *string `json:"id,omitempty" jsonschema:"The ID of the membership"`
	BoardID    *string `json:"boardId,omitempty" jsonschema:"The ID of the board"`
	UserID     *string `json:"userId,omitempty" jsonschema:"The ID of the user"`
	Role       *string `json:"role,omitempty" jsonschema:"The role of the user in the board"`
	CanComment *bool   `json:"canComment,omitempty" jsonschema:"Whether the user can comment on the board"`
}

var membershipActions = []string{"get_all", "create", "get_one", "update", "delete"}

func registerMembershipManager(server *mcp.Server, client *planka.Client) error {
	schema, err := inferSchema[membershipArgs](
		applyEnum("action", membershipActions),
		applyIDPattern("id", "boardId", "userId"),
		applyEnum("role", membershipRoles),
	)
	if err != nil {
		return err
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "mcp_kanban_membership_manager",
		Description: "Manage board memberships with various operations",
		InputSchema: schema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args membershipArgs) (*mcp.CallToolResult, any, error) {
		return respond(dispatchMembership(ctx, client, args))
	})
	return nil
}

func dispatchMembership(ctx context.Context, client *planka.Client, args membershipArgs) (any, error) {
	switch args.Action {
	case "get_all":
		boardID, err := requireID("boardId", args.BoardID)
		if err != nil {
			return nil, err
		}
		return client.GetBoardMemberships(ctx, boardID)

	case "create":
		boardID, err := requireID("boardId", args.BoardID)
		if err != nil {
			return nil, err
		}
		userID, err := requireID("userId", args.UserID)
		if err != nil {
			return nil, err
		}
		role, err := requireString("role", args.Role)
		if err != nil {
			return nil, err
		}
		return client.CreateBoardMembership(ctx, planka.CreateBoardMembershipOptions{
			BoardID: boardID,
			UserID:  userID,
			Role:    role,
		})

	case "get_one":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		return client.GetBoardMembership(ctx, id)

	case "update":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		return client.UpdateBoardMembership(ctx, id, args.Role, args.CanComment)

	case "delete":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		return client.DeleteBoardMembership(ctx, id)

	default:
		return nil, fmt.Errorf("unknown action: %s", args.Action)
	}
}
