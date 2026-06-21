package planka

// This file ports the zod entity schemas from common/types.ts. Two rules carry
// across from the TypeScript design notes:
//
//   - All Planka IDs are modeled as strings. They are numeric snowflakes; Go's
//     strict encoding/json will not coerce a JSON number into a string the way
//     zod's permissive parse did, so keeping them strings end-to-end preserves
//     the "42"-works / 42-rejected contract and dodges large-ID precision loss.
//   - Fields that Planka may return as null OR omit entirely use pointers
//     (background, updatedAt, stopwatch, ...). A JSON null unmarshals into a
//     pointer as nil and into a value type as the zero value, so a plain string
//     tolerates null too — pointers are used where nil vs "" is meaningful.

// User ports PlankaUserSchema.
type User struct {
	ID        string  `json:"id"`
	Email     string  `json:"email"`
	Name      *string `json:"name"`
	Username  string  `json:"username"`
	AvatarURL *string `json:"avatarUrl"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt *string `json:"updatedAt"`
}

// Project ports PlankaProjectSchema. background is optional AND nullable in
// Planka 2.1.1 (omitted entirely when unset).
type Project struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Background *string `json:"background,omitempty"`
	CreatedAt  string  `json:"createdAt"`
	UpdatedAt  *string `json:"updatedAt"`
}

// Board ports PlankaBoardSchema.
type Board struct {
	ID        string  `json:"id"`
	ProjectID string  `json:"projectId"`
	Name      string  `json:"name"`
	Position  float64 `json:"position"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt *string `json:"updatedAt"`
}

// List ports PlankaListSchema. Planka 2.1.1 auto-creates system lists with a
// null name; a JSON null decodes to "" here, which simply fails name matching
// (the intended behavior) rather than erroring.
type List struct {
	ID        string  `json:"id"`
	BoardID   string  `json:"boardId"`
	Name      string  `json:"name"`
	Position  float64 `json:"position"`
	Type      string  `json:"type,omitempty"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt *string `json:"updatedAt"`
}

// Label ports PlankaLabelSchema.
type Label struct {
	ID        string  `json:"id"`
	BoardID   string  `json:"boardId"`
	Name      string  `json:"name"`
	Color     string  `json:"color"`
	Position  float64 `json:"position,omitempty"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt *string `json:"updatedAt"`
}

// Stopwatch ports PlankaStopwatchSchema. startedAt is null when the stopwatch
// is stopped; total is accumulated seconds.
type Stopwatch struct {
	StartedAt *string `json:"startedAt"`
	Total     float64 `json:"total"`
}

// Card ports PlankaCardSchema, plus labelIds which the denormalized board
// detail attaches to each included card (used by board-summary label counts).
type Card struct {
	ID          string     `json:"id"`
	ListID      string     `json:"listId"`
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	Position    float64    `json:"position"`
	DueDate     *string    `json:"dueDate"`
	IsCompleted *bool      `json:"isCompleted,omitempty"`
	Stopwatch   *Stopwatch `json:"stopwatch,omitempty"`
	LabelIDs    []string   `json:"labelIds,omitempty"`
	CreatedAt   string     `json:"createdAt"`
	UpdatedAt   *string    `json:"updatedAt"`
}

// Task ports PlankaTaskSchema. Planka v2.1 nests tasks under task lists, so the
// task carries taskListId (and linkedCardId when it links to a card).
type Task struct {
	ID           string  `json:"id"`
	TaskListID   *string `json:"taskListId,omitempty"`
	LinkedCardID *string `json:"linkedCardId,omitempty"`
	Name         string  `json:"name"`
	IsCompleted  bool    `json:"isCompleted"`
	Position     float64 `json:"position"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    *string `json:"updatedAt"`
}

// TaskList is the v2.1 task-list container that holds tasks under a card.
type TaskList struct {
	ID       string  `json:"id"`
	CardID   string  `json:"cardId,omitempty"`
	Name     string  `json:"name,omitempty"`
	Position float64 `json:"position,omitempty"`
}

// Comment ports the v2.1 first-class comment (text at the top level). The TS
// schema is .passthrough(); the fields below cover every consumer (card-details
// reads text + createdAt).
type Comment struct {
	ID        string  `json:"id"`
	Text      string  `json:"text"`
	CardID    string  `json:"cardId,omitempty"`
	UserID    string  `json:"userId,omitempty"`
	CreatedAt string  `json:"createdAt,omitempty"`
	UpdatedAt *string `json:"updatedAt,omitempty"`
}

// BoardMembership ports the board-membership entity. role is a free string
// (Planka returns editor/admin/viewer); canComment is nullable.
type BoardMembership struct {
	ID         string  `json:"id"`
	BoardID    string  `json:"boardId"`
	UserID     string  `json:"userId"`
	Role       string  `json:"role"`
	CanComment *bool   `json:"canComment"`
	CreatedAt  string  `json:"createdAt"`
	UpdatedAt  *string `json:"updatedAt"`
}

// CardLabel ports PlankaCardLabelSchema (the card↔label join row).
type CardLabel struct {
	ID        string  `json:"id"`
	CardID    string  `json:"cardId"`
	LabelID   string  `json:"labelId"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt *string `json:"updatedAt"`
}

// CardMembership ports PlankaCardMembershipSchema.
type CardMembership struct {
	ID        string  `json:"id"`
	CardID    string  `json:"cardId"`
	UserID    string  `json:"userId"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt *string `json:"updatedAt"`
}

// Included is the denormalized "included" block Planka attaches to many
// responses. It is parsed with typed slices (not map[string]any) so composites
// can traverse project→board→card→list safely; unknown keys are dropped.
type Included struct {
	Projects         []Project         `json:"projects,omitempty"`
	Boards           []Board           `json:"boards,omitempty"`
	Lists            []List            `json:"lists,omitempty"`
	Cards            []Card            `json:"cards,omitempty"`
	Labels           []Label           `json:"labels,omitempty"`
	Tasks            []Task            `json:"tasks,omitempty"`
	TaskLists        []TaskList        `json:"taskLists,omitempty"`
	BoardMemberships []BoardMembership `json:"boardMemberships,omitempty"`
	CardLabels       []CardLabel       `json:"cardLabels,omitempty"`
	CardMemberships  []CardMembership  `json:"cardMemberships,omitempty"`
	Comments         []Comment         `json:"comments,omitempty"`
	Users            []User            `json:"users,omitempty"`
}

// itemEnvelope is the single-resource response shape: {"item": T, "included": ...}.
type itemEnvelope[T any] struct {
	Item     T         `json:"item"`
	Included *Included `json:"included,omitempty"`
}

// listEnvelope is the collection response shape: {"items": [T], "included": ...}.
type listEnvelope[T any] struct {
	Items    []T       `json:"items"`
	Included *Included `json:"included,omitempty"`
}
