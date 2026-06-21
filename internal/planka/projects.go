package planka

import (
	"context"
	"fmt"
)

// ProjectsPage is the paginated projects response, mirroring the {items, included}
// shape returned by GET /api/projects.
type ProjectsPage struct {
	Items    []Project `json:"items"`
	Included *Included `json:"included,omitempty"`
}

// GetProjects retrieves paginated projects. perPage is clamped to 100 if >100.
// Mirrors getProjects in projects.ts (PROPAGATE errors).
func (c *Client) GetProjects(ctx context.Context, page, perPage int) (*ProjectsPage, error) {
	if perPage > 100 {
		perPage = 100
	}
	var env listEnvelope[Project]
	path := fmt.Sprintf("/api/projects?page=%d&per_page=%d", page, perPage)
	if err := c.Get(ctx, path, &env); err != nil {
		return nil, err
	}
	return &ProjectsPage{Items: env.Items, Included: env.Included}, nil
}

// GetProject retrieves a single project by ID.
// Mirrors getProject in projects.ts (PROPAGATE errors).
func (c *Client) GetProject(ctx context.Context, id string) (*Project, error) {
	var env itemEnvelope[Project]
	if err := c.Get(ctx, "/api/projects/"+id, &env); err != nil {
		return nil, err
	}
	return &env.Item, nil
}
