package planka

import (
	"context"
	"fmt"
)

// BatchTaskInput is one item in a BatchCreateTasks call.
type BatchTaskInput struct {
	CardID   string
	Name     string
	Position *float64
}

// BatchTaskResult is the per-item shape inside BatchResult.Results.
type BatchTaskResult struct {
	Success bool  `json:"success"`
	Result  *Task `json:"result,omitempty"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// BatchTaskFailure records one item that failed during a batch create.
type BatchTaskFailure struct {
	Index int            `json:"index"`
	Task  BatchTaskInput `json:"task"`
	Error string         `json:"error"`
}

// BatchResult is the aggregate return from BatchCreateTasks.
type BatchResult struct {
	Results   []BatchTaskResult  `json:"results"`
	Successes []Task             `json:"successes"`
	Failures  []BatchTaskFailure `json:"failures"`
}

// ensureTaskListID returns the id of the first existing task-list on a card,
// creating one named "Tasks" if none exist. Mirrors ensureTaskListId in tasks.ts.
// Errors PROPAGATE.
func (c *Client) ensureTaskListID(ctx context.Context, cardID string) (string, error) {
	var env itemEnvelope[Card]
	if err := c.Get(ctx, "/api/cards/"+cardID, &env); err != nil {
		return "", err
	}
	if env.Included != nil && len(env.Included.TaskLists) > 0 {
		return env.Included.TaskLists[0].ID, nil
	}
	// Create a "Tasks" task-list.
	body := map[string]any{
		"name":     "Tasks",
		"position": 65535,
	}
	var created itemEnvelope[TaskList]
	if err := c.Post(ctx, "/api/cards/"+cardID+"/task-lists", body, &created); err != nil {
		return "", err
	}
	return created.Item.ID, nil
}

// CreateTask creates a task under the first (or newly created) task-list for
// the given card. Mirrors createTask in tasks.ts. PROPAGATE.
func (c *Client) CreateTask(ctx context.Context, cardID, name string, position float64) (*Task, error) {
	tlID, err := c.ensureTaskListID(ctx, cardID)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"name":     name,
		"position": position,
	}
	var env itemEnvelope[Task]
	if err := c.Post(ctx, "/api/task-lists/"+tlID+"/tasks", body, &env); err != nil {
		return nil, err
	}
	return &env.Item, nil
}

// GetTasks returns all tasks for a card via the card detail's included block.
// Mirrors getTasks in tasks.ts. SWALLOW errors → []Task{}.
func (c *Client) GetTasks(ctx context.Context, cardID string) ([]Task, error) {
	var env itemEnvelope[Card]
	if err := c.Get(ctx, "/api/cards/"+cardID, &env); err != nil {
		return []Task{}, nil
	}
	if env.Included != nil && len(env.Included.Tasks) > 0 {
		return env.Included.Tasks, nil
	}
	return []Task{}, nil
}

// GetTask retrieves a specific task by id from its card's included block.
// Mirrors getTask in tasks.ts. PROPAGATE.
func (c *Client) GetTask(ctx context.Context, id, cardID string) (*Task, error) {
	if cardID == "" {
		return nil, fmt.Errorf(
			"Card ID is required to get a task. Either provide it directly or create the task first.",
		)
	}
	var env itemEnvelope[Card]
	if err := c.Get(ctx, "/api/cards/"+cardID, &env); err != nil {
		return nil, err
	}
	if env.Included == nil || len(env.Included.Tasks) == 0 {
		return nil, fmt.Errorf("Failed to get tasks for card %s", cardID)
	}
	for i := range env.Included.Tasks {
		if env.Included.Tasks[i].ID == id {
			return &env.Included.Tasks[i], nil
		}
	}
	return nil, fmt.Errorf("Task with ID %s not found in card %s", id, cardID)
}

// UpdateTask patches a task's mutable fields. Mirrors updateTask in tasks.ts.
// PROPAGATE.
func (c *Client) UpdateTask(ctx context.Context, id string, name *string, isCompleted *bool, position *float64) (*Task, error) {
	body := map[string]any{}
	if name != nil {
		body["name"] = *name
	}
	if isCompleted != nil {
		body["isCompleted"] = *isCompleted
	}
	if position != nil {
		body["position"] = *position
	}
	var env itemEnvelope[Task]
	if err := c.Patch(ctx, "/api/tasks/"+id, body, &env); err != nil {
		return nil, err
	}
	return &env.Item, nil
}

// DeleteTask deletes a task by id. Mirrors deleteTask in tasks.ts. PROPAGATE.
func (c *Client) DeleteTask(ctx context.Context, id string) (map[string]bool, error) {
	if err := c.Delete(ctx, "/api/tasks/"+id); err != nil {
		return nil, err
	}
	return map[string]bool{"success": true}, nil
}

// BatchCreateTasks creates multiple tasks sequentially, never aborting on a
// single failure. Mirrors batchCreateTasks in tasks.ts.
func (c *Client) BatchCreateTasks(ctx context.Context, items []BatchTaskInput) (BatchResult, error) {
	var out BatchResult
	out.Results = make([]BatchTaskResult, 0, len(items))
	out.Successes = make([]Task, 0)
	out.Failures = make([]BatchTaskFailure, 0)

	for i, item := range items {
		pos := float64(65535 * (i + 1))
		if item.Position != nil && *item.Position != 0 {
			pos = *item.Position
		}
		task, err := c.CreateTask(ctx, item.CardID, item.Name, pos)
		if err != nil {
			msg := err.Error()
			out.Results = append(out.Results, BatchTaskResult{
				Success: false,
				Error: &struct {
					Message string `json:"message"`
				}{Message: msg},
			})
			out.Failures = append(out.Failures, BatchTaskFailure{
				Index: i,
				Task:  item,
				Error: msg,
			})
		} else {
			out.Results = append(out.Results, BatchTaskResult{Success: true, Result: task})
			out.Successes = append(out.Successes, *task)
		}
	}
	return out, nil
}
