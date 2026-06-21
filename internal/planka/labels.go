package planka

import "context"

// CreateLabelOptions ports the label creation parameters.
type CreateLabelOptions struct {
	BoardID  string
	Name     string
	Color    string
	Position float64
}

// UpdateLabelOptions ports the partial label update body; nil fields are omitted.
type UpdateLabelOptions struct {
	Name     *string
	Color    *string
	Position *float64
}

// CreateLabel creates a new label on a board.
// Mirrors createLabel in labels.ts — PROPAGATE errors.
func (c *Client) CreateLabel(ctx context.Context, opts CreateLabelOptions) (*Label, error) {
	body := map[string]any{
		"name":     opts.Name,
		"color":    opts.Color,
		"position": opts.Position,
	}
	var env itemEnvelope[Label]
	if err := c.Post(ctx, "/api/boards/"+opts.BoardID+"/labels", body, &env); err != nil {
		return nil, err
	}
	return &env.Item, nil
}

// GetLabels returns all labels for a board.
// Mirrors getLabels in labels.ts — PROPAGATE request errors (no try/catch around
// the request); return []Label{} only when the included block has no labels.
func (c *Client) GetLabels(ctx context.Context, boardID string) ([]Label, error) {
	var env itemEnvelope[Board]
	if err := c.Get(ctx, "/api/boards/"+boardID, &env); err != nil {
		return nil, err
	}
	if env.Included != nil && len(env.Included.Labels) > 0 {
		return env.Included.Labels, nil
	}
	return []Label{}, nil
}

// UpdateLabel patches a label's mutable fields.
// Mirrors updateLabel in labels.ts — PROPAGATE errors.
func (c *Client) UpdateLabel(ctx context.Context, id string, opts UpdateLabelOptions) (*Label, error) {
	body := map[string]any{}
	if opts.Name != nil {
		body["name"] = *opts.Name
	}
	if opts.Color != nil {
		body["color"] = *opts.Color
	}
	if opts.Position != nil {
		body["position"] = *opts.Position
	}
	var env itemEnvelope[Label]
	if err := c.Patch(ctx, "/api/labels/"+id, body, &env); err != nil {
		return nil, err
	}
	return &env.Item, nil
}

// DeleteLabel deletes a label by id.
// Mirrors deleteLabel in labels.ts — PROPAGATE errors.
func (c *Client) DeleteLabel(ctx context.Context, id string) (map[string]bool, error) {
	if err := c.Delete(ctx, "/api/labels/"+id); err != nil {
		return nil, err
	}
	return map[string]bool{"success": true}, nil
}

// AddLabelToCard adds a label to a card via the card-labels join.
// Mirrors addLabelToCard in labels.ts — PROPAGATE errors.
func (c *Client) AddLabelToCard(ctx context.Context, cardID, labelID string) (map[string]bool, error) {
	body := map[string]any{
		"labelId": labelID,
	}
	var env itemEnvelope[CardLabel]
	if err := c.Post(ctx, "/api/cards/"+cardID+"/card-labels", body, &env); err != nil {
		return nil, err
	}
	return map[string]bool{"success": true}, nil
}

// RemoveLabelFromCard removes a label from a card.
// Mirrors removeLabelFromCard in labels.ts — PROPAGATE errors.
// The path uses Planka's literal "labelId:" colon prefix; the colon is built by
// string concatenation and must NOT be percent-encoded.
func (c *Client) RemoveLabelFromCard(ctx context.Context, cardID, labelID string) (map[string]bool, error) {
	path := "/api/cards/" + cardID + "/card-labels/labelId:" + labelID
	if err := c.Delete(ctx, path); err != nil {
		return nil, err
	}
	return map[string]bool{"success": true}, nil
}
