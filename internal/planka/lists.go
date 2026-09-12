package planka

import "context"

// CreateListOptions ports CreateListSchema. Type defaults to "active".
type CreateListOptions struct {
	BoardID  string
	Name     string
	Position float64
	Type     string
}

// UpdateListOptions ports the partial update body; nil fields are omitted.
type UpdateListOptions struct {
	Name     *string
	Position *float64
}

// GetLists returns the lists on a board. Planka has no list-lists route; lists
// ride along in the board detail's included block. Mirrors getLists in
// lists.ts, which SWALLOWS request errors and yields [] (best-effort).
func (c *Client) GetLists(ctx context.Context, boardID string) ([]List, error) {
	var env itemEnvelope[Board]
	if err := c.Get(ctx, "/api/boards/"+boardID, &env); err != nil {
		return []List{}, nil
	}
	if env.Included != nil {
		return env.Included.Lists, nil
	}
	return []List{}, nil
}

// GetList returns a single list by id. Mirrors getList in lists.ts, which
// SWALLOWS errors and returns null (nil here).
func (c *Client) GetList(ctx context.Context, id string) (*List, error) {
	var env itemEnvelope[List]
	if err := c.Get(ctx, "/api/lists/"+id, &env); err != nil {
		return nil, nil
	}
	return &env.Item, nil
}

// CreateList creates a list on a board. Planka v2.1 requires a list type; it
// defaults to "active".
func (c *Client) CreateList(ctx context.Context, opts CreateListOptions) (*List, error) {
	typ := opts.Type
	if typ == "" {
		typ = "active"
	}
	body := map[string]any{
		"name":     opts.Name,
		"position": opts.Position,
		"type":     typ,
	}
	var env itemEnvelope[List]
	if err := c.Post(ctx, "/api/boards/"+opts.BoardID+"/lists", body, &env); err != nil {
		return nil, err
	}
	return &env.Item, nil
}

// UpdateList patches a list's mutable fields.
func (c *Client) UpdateList(ctx context.Context, id string, opts UpdateListOptions) (*List, error) {
	body := map[string]any{}
	if opts.Name != nil {
		body["name"] = *opts.Name
	}
	if opts.Position != nil {
		body["position"] = *opts.Position
	}
	var env itemEnvelope[List]
	if err := c.Patch(ctx, "/api/lists/"+id, body, &env); err != nil {
		return nil, err
	}
	return &env.Item, nil
}

// DeleteList deletes a list by id.
func (c *Client) DeleteList(ctx context.Context, id string) (map[string]bool, error) {
	if err := c.Delete(ctx, "/api/lists/"+id); err != nil {
		return nil, err
	}
	return map[string]bool{"success": true}, nil
}
