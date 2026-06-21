package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cameronsjo/kanban-mcp/internal/planka"
)

func init() { addRegistrar(registerProjectBoardManager) }

// projectBoardArgs is the flat, action-discriminated input for
// mcp_kanban_project_board_manager. Only Action is non-pointer (required by the
// schema); all other fields are optional pointers enforced per-action in the
// handler (Layer B).
type projectBoardArgs struct {
	Action             string   `json:"action" jsonschema:"The action to perform"`
	ID                 *string  `json:"id,omitempty" jsonschema:"The ID of the project or board"`
	ProjectID          *string  `json:"projectId,omitempty" jsonschema:"The ID of the project"`
	Name               *string  `json:"name,omitempty" jsonschema:"The name of the board"`
	Position           *float64 `json:"position,omitempty" jsonschema:"The position of the board"`
	Type               *string  `json:"type,omitempty" jsonschema:"The type of the board"`
	Page               *float64 `json:"page,omitempty" jsonschema:"The page number for pagination (1-indexed)"`
	PerPage            *float64 `json:"perPage,omitempty" jsonschema:"The number of items per page"`
	BoardID            *string  `json:"boardId,omitempty" jsonschema:"The ID of the board to get a summary for"`
	IncludeTaskDetails *bool    `json:"includeTaskDetails,omitempty" jsonschema:"Whether to include detailed task information for each card"`
	IncludeComments    *bool    `json:"includeComments,omitempty" jsonschema:"Whether to include comments for each card"`
}

var projectBoardActions = []string{
	"get_projects",
	"get_project",
	"get_boards",
	"create_board",
	"get_board",
	"update_board",
	"delete_board",
	"get_board_summary",
}

func registerProjectBoardManager(server *mcp.Server, client *planka.Client) error {
	// Layer A: applyIDPattern("id", "projectId") — boardId is plain z.string, NOT hardened.
	schema, err := inferSchema[projectBoardArgs](
		applyEnum("action", projectBoardActions),
		applyIDPattern("id", "projectId"),
	)
	if err != nil {
		return err
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "mcp_kanban_project_board_manager",
		Description: "Manage projects and boards with various operations",
		InputSchema: schema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args projectBoardArgs) (*mcp.CallToolResult, any, error) {
		return respond(dispatchProjectBoard(ctx, client, args))
	})
	return nil
}

// derefBool safely dereferences a *bool, returning false for nil.
func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

func dispatchProjectBoard(ctx context.Context, client *planka.Client, args projectBoardArgs) (any, error) {
	switch args.Action {
	case "get_projects":
		if args.Page == nil || args.PerPage == nil {
			return nil, fmt.Errorf("page and perPage are required for get_projects action")
		}
		return client.GetProjects(ctx, int(*args.Page), int(*args.PerPage))

	case "get_project":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		return client.GetProject(ctx, id)

	case "get_boards":
		projectID, err := requireID("projectId", args.ProjectID)
		if err != nil {
			return nil, err
		}
		return client.GetBoards(ctx, projectID)

	case "create_board":
		projectID, err := requireID("projectId", args.ProjectID)
		if err != nil {
			return nil, err
		}
		name, err := requireName("name", args.Name)
		if err != nil {
			return nil, err
		}
		if args.Position == nil {
			return nil, fmt.Errorf("projectId, name, and position are required for create_board action")
		}
		return client.CreateBoard(ctx, planka.CreateBoardOptions{
			ProjectID: projectID,
			Name:      name,
			Position:  *args.Position,
		})

	case "get_board":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		return client.GetBoard(ctx, id)

	case "update_board":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		name, err := requireName("name", args.Name)
		if err != nil {
			return nil, err
		}
		if args.Position == nil {
			return nil, fmt.Errorf("id, name, and position are required for update_board action")
		}
		opts := planka.UpdateBoardOptions{
			Name:     &name,
			Position: args.Position,
		}
		if args.Type != nil {
			opts.Type = args.Type
		}
		return client.UpdateBoard(ctx, id, opts)

	case "delete_board":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		return client.DeleteBoard(ctx, id)

	case "get_board_summary":
		boardID, err := requireString("boardId", args.BoardID)
		if err != nil {
			return nil, err
		}
		return boardSummary(ctx, client, boardID, derefBool(args.IncludeTaskDetails), derefBool(args.IncludeComments))

	default:
		return nil, fmt.Errorf("unknown action: %s", args.Action)
	}
}
