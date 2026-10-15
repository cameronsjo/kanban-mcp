package planka

import (
	"context"
	"fmt"
)

// CreateCardOptions ports CreateCardSchema.
type CreateCardOptions struct {
	ListID      string
	Name        string
	Description string
	Position    float64
	Type        string
}

// UpdateCardOptions ports UpdateCardSchema; nil fields are omitted from the PATCH body.
type UpdateCardOptions struct {
	Name        *string
	Description *string
	DueDate     *string
	Position    *float64
	IsCompleted *bool
}

// GetCards returns all cards for a given list ID. It performs a board-walk and
// SWALLOWS all errors (mirrors getCards in cards.ts which catches and returns []).
func (c *Client) GetCards(ctx context.Context, listID string) ([]Card, error) {
	var projEnv listEnvelope[interface{}]
	if err := c.Get(ctx, "/api/projects", &projEnv); err != nil {
		return []Card{}, nil
	}
	if projEnv.Included == nil || len(projEnv.Included.Boards) == 0 {
		return []Card{}, nil
	}

	for _, board := range projEnv.Included.Boards {
		var boardEnv itemEnvelope[Board]
		if err := c.Get(ctx, "/api/boards/"+board.ID, &boardEnv); err != nil {
			continue
		}
		if boardEnv.Included == nil || len(boardEnv.Included.Cards) == 0 {
			continue
		}
		var matched []Card
		for _, card := range boardEnv.Included.Cards {
			if card.ListID == listID {
				matched = append(matched, card)
			}
		}
		if len(matched) > 0 {
			return matched, nil
		}
	}
	return []Card{}, nil
}

// GetCard returns a single card by ID. PROPAGATE errors.
func (c *Client) GetCard(ctx context.Context, id string) (*Card, error) {
	var env itemEnvelope[Card]
	if err := c.Get(ctx, "/api/cards/"+id, &env); err != nil {
		return nil, err
	}
	return &env.Item, nil
}

// CreateCard creates a card in a list. Type defaults to "project" when empty.
// PROPAGATE errors.
func (c *Client) CreateCard(ctx context.Context, opts CreateCardOptions) (*Card, error) {
	typ := opts.Type
	if typ == "" {
		typ = "project"
	}
	body := map[string]any{
		"name":        opts.Name,
		"description": opts.Description,
		"position":    opts.Position,
		"type":        typ,
	}
	var env itemEnvelope[Card]
	if err := c.Post(ctx, "/api/lists/"+opts.ListID+"/cards", body, &env); err != nil {
		return nil, err
	}
	return &env.Item, nil
}

// UpdateCard patches a card's mutable fields. PROPAGATE errors.
func (c *Client) UpdateCard(ctx context.Context, id string, opts UpdateCardOptions) (*Card, error) {
	body := map[string]any{}
	if opts.Name != nil {
		body["name"] = *opts.Name
	}
	if opts.Description != nil {
		body["description"] = *opts.Description
	}
	if opts.Position != nil {
		body["position"] = *opts.Position
	}
	if opts.DueDate != nil {
		body["dueDate"] = *opts.DueDate
	}
	if opts.IsCompleted != nil {
		body["isCompleted"] = *opts.IsCompleted
	}
	var env itemEnvelope[Card]
	if err := c.Patch(ctx, "/api/cards/"+id, body, &env); err != nil {
		return nil, err
	}
	return &env.Item, nil
}

// MoveCard moves a card to a different list/position. boardID and projectID are
// included in the body only when non-nil (cross-board moves). PROPAGATE errors.
func (c *Client) MoveCard(ctx context.Context, id, listID string, position float64, boardID, projectID *string) (*Card, error) {
	body := map[string]any{
		"listId":   listID,
		"position": position,
	}
	if boardID != nil {
		body["boardId"] = *boardID
	}
	if projectID != nil {
		body["projectId"] = *projectID
	}
	var env itemEnvelope[Card]
	if err := c.Patch(ctx, "/api/cards/"+id, body, &env); err != nil {
		return nil, err
	}
	return &env.Item, nil
}

// DuplicateCard creates a "Copy of" clone of an existing card.
// Propagates GetCard errors; errors if the card's list ID is unknown.
func (c *Client) DuplicateCard(ctx context.Context, id string, position float64) (*Card, error) {
	orig, err := c.GetCard(ctx, id)
	if err != nil {
		return nil, err
	}
	if orig.ListID == "" {
		return nil, fmt.Errorf("Could not determine list ID for card duplication")
	}
	desc := ""
	if orig.Description != nil {
		desc = *orig.Description
	}
	return c.CreateCard(ctx, CreateCardOptions{
		ListID:      orig.ListID,
		Name:        "Copy of " + orig.Name,
		Description: desc,
		Position:    position,
		Type:        "",
	})
}

// DeleteCard deletes a card by ID. PROPAGATE errors.
func (c *Client) DeleteCard(ctx context.Context, id string) (map[string]bool, error) {
	if err := c.Delete(ctx, "/api/cards/"+id); err != nil {
		return nil, err
	}
	return map[string]bool{"success": true}, nil
}
