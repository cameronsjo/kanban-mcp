package planka

import "context"

// CreateBoardOptions holds the fields needed to create a board.
type CreateBoardOptions struct {
	ProjectID string
	Name      string
	Position  float64
}

// UpdateBoardOptions holds the optional fields for a board PATCH.
// Nil fields are omitted from the request body.
type UpdateBoardOptions struct {
	Name     *string
	Position *float64
	Type     *string
}

// GetBoards returns boards for a project. Planka has no direct project-boards
// route; boards ride along in the GET /api/projects included block.
// Mirrors getBoards in boards.ts (SWALLOW all errors → []Board{}).
func (c *Client) GetBoards(ctx context.Context, projectID string) ([]Board, error) {
	var env listEnvelope[Project]
	if err := c.Get(ctx, "/api/projects", &env); err != nil {
		return []Board{}, nil
	}
	if env.Included == nil {
		return []Board{}, nil
	}
	result := []Board{}
	for _, b := range env.Included.Boards {
		if b.ProjectID == projectID {
			result = append(result, b)
		}
	}
	return result, nil
}

// GetBoard retrieves a single board by ID.
// Mirrors getBoard in boards.ts (PROPAGATE errors).
func (c *Client) GetBoard(ctx context.Context, id string) (*Board, error) {
	var env itemEnvelope[Board]
	if err := c.Get(ctx, "/api/boards/"+id, &env); err != nil {
		return nil, err
	}
	return &env.Item, nil
}

// BoardDetail fetches a board together with its denormalized included block
// (lists, labels, cards, ...) in one request, so callers that need several of
// those — board_summary — don't re-GET the same board detail per resource.
func (c *Client) BoardDetail(ctx context.Context, id string) (*Board, *Included, error) {
	var env itemEnvelope[Board]
	if err := c.Get(ctx, "/api/boards/"+id, &env); err != nil {
		return nil, nil, err
	}
	return &env.Item, env.Included, nil
}

// CreateBoard creates a board in a project, then best-effort: adds the admin
// user as an editor, creates default lists and labels.
// Mirrors createBoard in boards.ts (PROPAGATE board-creation error; ignore rest).
func (c *Client) CreateBoard(ctx context.Context, opts CreateBoardOptions) (*Board, error) {
	body := map[string]any{
		"name":     opts.Name,
		"position": opts.Position,
	}
	var env itemEnvelope[Board]
	if err := c.Post(ctx, "/api/projects/"+opts.ProjectID+"/boards", body, &env); err != nil {
		return nil, err
	}
	board := &env.Item

	// Best-effort: add admin user as editor (ignore all errors).
	if id, _ := c.AdminUserID(ctx); id != "" {
		_, _ = c.CreateBoardMembership(ctx, CreateBoardMembershipOptions{
			BoardID: board.ID,
			UserID:  id,
			Role:    "editor",
		})
	}

	// Best-effort: create default lists (ignore errors).
	createDefaultLists(ctx, c, board.ID)

	// Best-effort: create default labels (ignore errors).
	createDefaultLabels(ctx, c, board.ID)

	return board, nil
}

// createDefaultLists creates the standard set of lists on a new board.
// All errors are ignored (best-effort).
func createDefaultLists(ctx context.Context, c *Client, boardID string) {
	defaults := []struct {
		name     string
		position float64
	}{
		{"Backlog", 65535},
		{"To Do", 131070},
		{"In Progress", 196605},
		{"On Hold", 262140},
		{"Review", 327675},
		{"Done", 393210},
	}
	for _, d := range defaults {
		_, _ = c.CreateList(ctx, CreateListOptions{
			BoardID:  boardID,
			Name:     d.name,
			Position: d.position,
		})
	}
}

// createDefaultLabels creates the standard priority/type/status labels on a
// new board. All errors are ignored (best-effort).
func createDefaultLabels(ctx context.Context, c *Client, boardID string) {
	defaults := []struct {
		name     string
		color    string
		position float64
	}{
		{"P0: Critical", "berry-red", 65535},
		{"P1: High", "red-burgundy", 131070},
		{"P2: Medium", "pumpkin-orange", 196605},
		{"P3: Low", "sunny-grass", 262140},
		{"Bug", "coral-green", 327675},
		{"Feature", "lagoon-blue", 393210},
		{"Enhancement", "bright-moss", 458745},
		{"Documentation", "light-orange", 524280},
		{"Blocked", "midnight-blue", 589815},
		{"Needs Info", "desert-sand", 655350},
		{"Ready", "egg-yellow", 720885},
	}
	for _, d := range defaults {
		_, _ = c.CreateLabel(ctx, CreateLabelOptions{
			BoardID:  boardID,
			Name:     d.name,
			Color:    d.color,
			Position: d.position,
		})
	}
}

// UpdateBoard patches a board's mutable fields.
// Mirrors updateBoard in boards.ts (PROPAGATE errors).
func (c *Client) UpdateBoard(ctx context.Context, id string, opts UpdateBoardOptions) (*Board, error) {
	body := map[string]any{}
	if opts.Name != nil {
		body["name"] = *opts.Name
	}
	if opts.Position != nil {
		body["position"] = *opts.Position
	}
	if opts.Type != nil {
		body["type"] = *opts.Type
	}
	var env itemEnvelope[Board]
	if err := c.Patch(ctx, "/api/boards/"+id, body, &env); err != nil {
		return nil, err
	}
	return &env.Item, nil
}

// DeleteBoard deletes a board by ID.
// Mirrors deleteBoard in boards.ts (PROPAGATE errors).
func (c *Client) DeleteBoard(ctx context.Context, id string) (map[string]bool, error) {
	if err := c.Delete(ctx, "/api/boards/"+id); err != nil {
		return nil, err
	}
	return map[string]bool{"success": true}, nil
}
