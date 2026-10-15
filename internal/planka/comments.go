package planka

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

// CreateComment creates a new comment on a card.
// Mirrors createComment in comments.ts — PROPAGATE errors.
func (c *Client) CreateComment(ctx context.Context, cardID, text string) (*Comment, error) {
	body := map[string]any{
		"text": text,
	}
	var env itemEnvelope[Comment]
	if err := c.Post(ctx, "/api/cards/"+cardID+"/comments", body, &env); err != nil {
		return nil, err
	}
	return &env.Item, nil
}

// GetComments returns all comments on a card.
// Mirrors getComments in comments.ts — PROPAGATE request errors (no try/catch
// around the request itself).
func (c *Client) GetComments(ctx context.Context, cardID string) ([]Comment, error) {
	var raw json.RawMessage
	if err := c.Get(ctx, "/api/cards/"+cardID+"/comments", &raw); err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var comments []Comment
		if err := json.Unmarshal(trimmed, &comments); err != nil {
			return nil, fmt.Errorf("decode comments response: %w", err)
		}
		return comments, nil
	}

	var env listEnvelope[Comment]
	if err := json.Unmarshal(trimmed, &env); err != nil {
		return nil, fmt.Errorf("decode comments response: %w", err)
	}
	if env.Items == nil {
		return []Comment{}, nil
	}
	return env.Items, nil
}

// GetComment retrieves a comment by id via a board-walk.
// Planka has no GET-by-id route for comments; mirrors getComment in comments.ts
// which walks projects → boards → cards → comments — PROPAGATE errors.
func (c *Client) GetComment(ctx context.Context, id string) (*Comment, error) {
	// GET /api/projects to discover all boards via included.boards.
	var projectsEnv listEnvelope[Project]
	if err := c.Get(ctx, "/api/projects", &projectsEnv); err != nil {
		return nil, err
	}

	if projectsEnv.Included == nil || len(projectsEnv.Included.Boards) == 0 {
		return nil, fmt.Errorf("No boards found")
	}

	for _, board := range projectsEnv.Included.Boards {
		// GET /api/boards/{id} to enumerate cards.
		var boardEnv itemEnvelope[Board]
		if err := c.Get(ctx, "/api/boards/"+board.ID, &boardEnv); err != nil {
			return nil, err
		}
		if boardEnv.Included == nil || len(boardEnv.Included.Cards) == 0 {
			continue
		}

		for _, card := range boardEnv.Included.Cards {
			// GET /api/cards/{cardId}/comments to search for the comment.
			var commentsEnv listEnvelope[Comment]
			if err := c.Get(ctx, "/api/cards/"+card.ID+"/comments", &commentsEnv); err != nil {
				return nil, err
			}
			for i := range commentsEnv.Items {
				if commentsEnv.Items[i].ID == id {
					return &commentsEnv.Items[i], nil
				}
			}
		}
	}

	return nil, fmt.Errorf("Comment not found: %s", id)
}

// UpdateComment patches a comment's text.
// Mirrors updateComment in comments.ts — PROPAGATE errors.
func (c *Client) UpdateComment(ctx context.Context, id, text string) (*Comment, error) {
	body := map[string]any{
		"text": text,
	}
	var env itemEnvelope[Comment]
	if err := c.Patch(ctx, "/api/comments/"+id, body, &env); err != nil {
		return nil, err
	}
	return &env.Item, nil
}

// DeleteComment deletes a comment by id.
// Mirrors deleteComment in comments.ts — PROPAGATE errors.
func (c *Client) DeleteComment(ctx context.Context, id string) (map[string]bool, error) {
	if err := c.Delete(ctx, "/api/comments/"+id); err != nil {
		return nil, err
	}
	return map[string]bool{"success": true}, nil
}
