package planka

import "context"

// CreateBoardMembershipOptions ports CreateBoardMembershipSchema.
type CreateBoardMembershipOptions struct {
	BoardID string
	UserID  string
	Role    string
}

// CreateBoardMembership adds a user to a board with the given role.
// Mirrors createBoardMembership in boardMemberships.ts. PROPAGATE.
func (c *Client) CreateBoardMembership(ctx context.Context, opts CreateBoardMembershipOptions) (*BoardMembership, error) {
	body := map[string]any{
		"userId": opts.UserID,
		"role":   opts.Role,
	}
	var env itemEnvelope[BoardMembership]
	if err := c.Post(ctx, "/api/boards/"+opts.BoardID+"/board-memberships", body, &env); err != nil {
		return nil, err
	}
	return &env.Item, nil
}

// GetBoardMemberships returns all memberships for a board via the board detail's
// included block. Mirrors getBoardMemberships in boardMemberships.ts. PROPAGATE
// request errors (no try/catch around the request in the TS source).
func (c *Client) GetBoardMemberships(ctx context.Context, boardID string) ([]BoardMembership, error) {
	var env itemEnvelope[Board]
	if err := c.Get(ctx, "/api/boards/"+boardID, &env); err != nil {
		return nil, err
	}
	if env.Included != nil && len(env.Included.BoardMemberships) > 0 {
		return env.Included.BoardMemberships, nil
	}
	return []BoardMembership{}, nil
}

// GetBoardMembership retrieves a specific board membership by id.
// Mirrors getBoardMembership in boardMemberships.ts. PROPAGATE.
func (c *Client) GetBoardMembership(ctx context.Context, id string) (*BoardMembership, error) {
	var env itemEnvelope[BoardMembership]
	if err := c.Get(ctx, "/api/board-memberships/"+id, &env); err != nil {
		return nil, err
	}
	return &env.Item, nil
}

// UpdateBoardMembership patches a membership's role and/or canComment flag.
// Mirrors updateBoardMembership in boardMemberships.ts. PROPAGATE.
func (c *Client) UpdateBoardMembership(ctx context.Context, id string, role *string, canComment *bool) (*BoardMembership, error) {
	body := map[string]any{}
	if role != nil {
		body["role"] = *role
	}
	if canComment != nil {
		body["canComment"] = *canComment
	}
	var env itemEnvelope[BoardMembership]
	if err := c.Patch(ctx, "/api/board-memberships/"+id, body, &env); err != nil {
		return nil, err
	}
	return &env.Item, nil
}

// DeleteBoardMembership removes a user from a board.
// Mirrors deleteBoardMembership in boardMemberships.ts. PROPAGATE.
func (c *Client) DeleteBoardMembership(ctx context.Context, id string) (map[string]bool, error) {
	if err := c.Delete(ctx, "/api/board-memberships/"+id); err != nil {
		return nil, err
	}
	return map[string]bool{"success": true}, nil
}
