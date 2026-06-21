package tools

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cameronsjo/kanban-mcp/internal/planka"
)

func init() { addRegistrar(registerTaskManager) }

// taskBatchItem is the per-task shape inside a batch_create request.
type taskBatchItem struct {
	CardID   string   `json:"cardId" jsonschema:"The ID of the card for this task"`
	Name     string   `json:"name" jsonschema:"The name of this task"`
	Position *float64 `json:"position,omitempty" jsonschema:"The position of this task"`
}

// taskArgs is the flat, action-discriminated input for mcp_kanban_task_manager.
type taskArgs struct {
	Action      string           `json:"action" jsonschema:"The action to perform"`
	ID          *string          `json:"id,omitempty" jsonschema:"The ID of the task"`
	CardID      *string          `json:"cardId,omitempty" jsonschema:"The ID of the card"`
	Name        *string          `json:"name,omitempty" jsonschema:"The name of the task"`
	IsCompleted *bool            `json:"isCompleted,omitempty" jsonschema:"Whether the task is completed"`
	Position    *float64         `json:"position,omitempty" jsonschema:"The position of the task"`
	Tasks       *[]taskBatchItem `json:"tasks,omitempty" jsonschema:"Array of tasks to create in batch"`
}

var taskActions = []string{"get_all", "create", "batch_create", "get_one", "update", "delete", "complete_task"}

// applyTasksCardIDPattern sets the ^\d+$ pattern on tasks[].cardId, which is a
// plankaId field in the TS zod schema (see index.ts line ~588).
func applyTasksCardIDPattern(s *jsonschema.Schema) {
	if s.Properties == nil {
		return
	}
	tasksP := s.Properties["tasks"]
	if tasksP == nil || tasksP.Items == nil {
		return
	}
	if tasksP.Items.Properties == nil {
		return
	}
	cardIDProp := tasksP.Items.Properties["cardId"]
	if cardIDProp == nil {
		return
	}
	cardIDProp.Pattern = numericIDPattern
}

func registerTaskManager(server *mcp.Server, client *planka.Client) error {
	schema, err := inferSchema[taskArgs](
		applyEnum("action", taskActions),
		applyIDPattern("id", "cardId"),
		applyTasksCardIDPattern,
	)
	if err != nil {
		return err
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "mcp_kanban_task_manager",
		Description: "Manage kanban tasks with various operations",
		InputSchema: schema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args taskArgs) (*mcp.CallToolResult, any, error) {
		return respond(dispatchTask(ctx, client, args))
	})
	return nil
}

func dispatchTask(ctx context.Context, client *planka.Client, args taskArgs) (any, error) {
	switch args.Action {
	case "get_all":
		cardID, err := requireID("cardId", args.CardID)
		if err != nil {
			return nil, err
		}
		return client.GetTasks(ctx, cardID)

	case "create":
		cardID, err := requireID("cardId", args.CardID)
		if err != nil {
			return nil, err
		}
		name, err := requireName("name", args.Name)
		if err != nil {
			return nil, err
		}
		pos := 65535.0
		if args.Position != nil {
			pos = *args.Position
		}
		return client.CreateTask(ctx, cardID, name, pos)

	case "batch_create":
		if args.Tasks == nil || len(*args.Tasks) == 0 {
			return nil, fmt.Errorf("tasks array is required for batch_create action")
		}
		inputs := make([]planka.BatchTaskInput, len(*args.Tasks))
		for i, t := range *args.Tasks {
			inputs[i] = planka.BatchTaskInput{
				CardID:   t.CardID,
				Name:     t.Name,
				Position: t.Position,
			}
		}
		return client.BatchCreateTasks(ctx, inputs)

	case "get_one":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		// No cardId in this tool's schema — pass "" and GetTask errors per TS.
		return client.GetTask(ctx, id, "")

	case "update":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		return client.UpdateTask(ctx, id, args.Name, args.IsCompleted, args.Position)

	case "complete_task":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		t := true
		return client.UpdateTask(ctx, id, nil, &t, nil)

	case "delete":
		id, err := requireID("id", args.ID)
		if err != nil {
			return nil, err
		}
		return client.DeleteTask(ctx, id)

	default:
		return nil, fmt.Errorf("unknown action: %s", args.Action)
	}
}
