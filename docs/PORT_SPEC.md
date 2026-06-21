# Go Port Spec — resource fan-out contract

This is the binding contract for porting the 7 remaining Planka resources to Go.
Code to these signatures exactly so the package compiles as one unit and matches
the TypeScript server's behavior. Cross-check each function against the named
`operations/*.ts` / `index.ts` source in this same worktree.

## Read first (the canonical template)

- `internal/planka/lists.go` — operations layer pattern.
- `internal/tools/list.go` — dispatch + Layer-A/B validation pattern.
- `internal/tools/helpers_test.go` — the shared `mockPlanka` test harness. REUSE
  it (`newToolClient`, `mockPlanka{respond:...}`, `sp/fp/bp`). Do NOT redefine
  `mockPlanka` or the pointer helpers — one definition per package.
- `internal/planka/types.go` — all entity structs already exist (Project, Board,
  List, Card, Task, TaskList, Label, Comment, BoardMembership, ...). Do NOT
  redefine them. The `Included` block and `itemEnvelope[T]`/`listEnvelope[T]`
  generics already exist (envelopes are unexported — decode into a local
  `itemEnvelope[Card]` etc. inside the planka package).

## Conventions (apply to every resource)

1. **Operations layer** = methods on `*planka.Client` in `internal/planka/<r>.go`.
   Use `c.Get/Post/Patch/Delete`. Build request bodies as `map[string]any`
   containing ONLY the fields the TS sends (omit a field by not adding the key).
2. **IDs are strings** end-to-end. Positions and page/perPage are `float64`
   (zod `z.number()` → JSON "number"; keep `float64` for schema parity).
3. **Error handling is per-function and load-bearing — match the TS exactly:**
   - *Propagate* (return the error) where the TS function has NO try/catch around
     the request or re-throws.
   - *Swallow* (return the empty/null value, `nil` error) where the TS function
     wraps the request in try/catch and returns `[]`/`null` on failure. Each
     signature below says which.
4. **Mutation success** (delete/remove/add) returns `map[string]bool{"success": true}`
   (marshals to `{"success":true}`), matching the TS `{ success: true }`.
5. **Dispatch layer** = `internal/tools/<r>.go`, following `tools/list.go`:
   `init(){ addRegistrar(registerXManager) }`, a flat `xArgs` struct (only
   `Action string \`json:"action"\`` is non-pointer; everything else is an
   optional pointer with `,omitempty` + a `jsonschema:"..."` description),
   `registerXManager` that builds the schema via `inferSchema[xArgs](mutators...)`
   and `mcp.AddTool` with `InputSchema: schema`, and a `dispatchX` switch ending
   in `default: return nil, fmt.Errorf("unknown action: %s", args.Action)`.
   Handler body: `return respond(dispatchX(ctx, client, args))`.
6. **Layer A schema mutators** — apply `applyIDPattern(...)` ONLY to the fields
   listed per tool below (these are the `plankaId` fields in index.ts; the others
   are plain `z.string()` and MUST stay unhardened for golden-schema parity).
   Apply `applyEnum("action", xActions)` and any value enums noted.
7. **Layer B requiredness** — in the handler: use `requireID(field, ptr)` for the
   `plankaId` fields (re-checks `^\d+$`), `requireString(field, ptr)` for plain
   `z.string()` id-like fields (presence only), `requireName(field, ptr)` for
   names (trim + non-empty). Mirror the TS `if (!args.x) throw` checks exactly,
   including which fields each action requires.
8. **Tests** — `internal/tools/<r>_test.go`, table-driven through `newToolClient`.
   Cover: each action's happy path (assert method + path + request body), the
   key validation errors, unknown-action, and the resource's specific quirk.

---

## 1. project_board  →  planka/projects.go, planka/boards.go, tools/projectboard.go

Tool name: `mcp_kanban_project_board_manager`
Description: `Manage projects and boards with various operations`
Actions: `get_projects, get_project, get_boards, create_board, get_board, update_board, delete_board, get_board_summary`
Layer-A IDs: `applyIDPattern("id", "projectId")`  (NOTE: `boardId` here is plain z.string — do NOT harden it.)
No value enums beyond action. `type` is plain string.

`xArgs` fields: Action; ID*, ProjectID* (plankaId); Name*; Position*float64;
Type*; Page*float64; PerPage*float64; BoardID*; IncludeTaskDetails*bool;
IncludeComments*bool.

### planka/projects.go
- `type ProjectsPage struct { Items []Project \`json:"items"\`; Included *Included \`json:"included,omitempty"\` }`
- `func (c *Client) GetProjects(ctx, page, perPage int) (*ProjectsPage, error)` —
  clamp perPage to 100 if >100; GET `/api/projects?page={page}&per_page={perPage}`;
  PROPAGATE; return whole `{items, included}`.
- `func (c *Client) GetProject(ctx, id string) (*Project, error)` — GET
  `/api/projects/{id}`; PROPAGATE; return item.

### planka/boards.go
- `type CreateBoardOptions struct { ProjectID, Name string; Position float64 }`
- `type UpdateBoardOptions struct { Name *string; Position *float64; Type *string }`
- `func (c *Client) GetBoards(ctx, projectID string) ([]Board, error)` — GET
  `/api/projects`; from `included.Boards` keep those with `ProjectID == projectID`;
  SWALLOW errors → `[]Board{}`.
- `func (c *Client) GetBoard(ctx, id string) (*Board, error)` — GET
  `/api/boards/{id}`; PROPAGATE; return item.
- `func (c *Client) CreateBoard(ctx, opts CreateBoardOptions) (*Board, error)` —
  POST `/api/projects/{projectId}/boards` body `{name, position}` → board (PROPAGATE
  this error). THEN best-effort (ignore all errors): if `id,_ := c.AdminUserID(ctx)`
  non-empty, `c.CreateBoardMembership(ctx, CreateBoardMembershipOptions{BoardID: board.ID, UserID: id, Role: "editor"})`;
  then `createDefaultLists(ctx, board.ID)`; then `createDefaultLabels(ctx, board.ID)`.
  Return board.
  - `createDefaultLists`: in order, `c.CreateList(ctx, CreateListOptions{BoardID, Name, Position})` for
    {Backlog 65535, To Do 131070, In Progress 196605, On Hold 262140, Review 327675, Done 393210}. Ignore errors.
  - `createDefaultLabels`: `c.CreateLabel(ctx, CreateLabelOptions{BoardID, Name, Color, Position})` for the 11 in boards.ts
    (P0:Critical berry-red 65535, P1:High red-burgundy 131070, P2:Medium pumpkin-orange 196605, P3:Low sunny-grass 262140,
    Bug coral-green 327675, Feature lagoon-blue 393210, Enhancement bright-moss 458745, Documentation light-orange 524280,
    Blocked midnight-blue 589815, Needs Info desert-sand 655350, Ready egg-yellow 720885). Ignore errors.
- `func (c *Client) UpdateBoard(ctx, id string, opts UpdateBoardOptions) (*Board, error)` —
  PATCH `/api/boards/{id}` body of set fields {name?, position?, type?}; PROPAGATE; item.
- `func (c *Client) DeleteBoard(ctx, id string) (map[string]bool, error)` — DELETE
  `/api/boards/{id}`; PROPAGATE; `{"success":true}`.

### tools/projectboard.go dispatch (mirror index.ts requiredness)
- get_projects: require Page AND PerPage present (else error "page and perPage are required for get_projects action"); `client.GetProjects(ctx, int(*Page), int(*PerPage))`.
- get_project: requireID("id"); GetProject.
- get_boards: requireString("projectId")... NO — projectId IS plankaId here → requireID("projectId"); GetBoards.
- create_board: require projectId(requireID), name(requireName), Position present; `CreateBoard(CreateBoardOptions{ProjectID, Name, Position:*Position})`.
- get_board: requireID("id"); GetBoard.
- update_board: require id(requireID), name(requireName), Position present; UpdateBoard with Name+Position (and Type if set).
- delete_board: requireID("id"); DeleteBoard.
- get_board_summary: requireString("boardId") (boardId is z.string here, presence only); `boardSummary(ctx, client, boardID, derefBool(IncludeTaskDetails), derefBool(IncludeComments))` (the composite stub already exists).

---

## 2. card  →  planka/cards.go, tools/card.go

Tool: `mcp_kanban_card_manager`  Desc: `Manage kanban cards with various operations`
Actions: `get_all, create, get_one, update, move, duplicate, delete, create_with_tasks, get_details`
Layer-A IDs: `applyIDPattern("id", "listId")`  (boardId, projectId, cardId here are plain z.string — do NOT harden.)
`xArgs`: Action; ID*, ListId* (plankaId); BoardID*, ProjectID*, CardID* (plain); Name*; Description*; Position*float64; DueDate*; IsCompleted*bool; Tasks*[]string; Comment*.

### planka/cards.go
- `type CreateCardOptions struct { ListID, Name, Description string; Position float64; Type string }`
- `type UpdateCardOptions struct { Name, Description, DueDate *string; Position *float64; IsCompleted *bool }`
- `func (c *Client) GetCards(ctx, listID string) ([]Card, error)` — board-walk,
  SWALLOW all errors → `[]Card{}`: GET `/api/projects`; if no `included.Boards` → `[]`;
  for each board: GET `/api/boards/{board.ID}`; if no `included.Cards` → continue;
  keep cards with `ListID == listID`; if any matched → return them; else continue.
  Return `[]Card{}` if none.
- `func (c *Client) GetCard(ctx, id string) (*Card, error)` — GET `/api/cards/{id}`;
  PROPAGATE; return item.
- `func (c *Client) CreateCard(ctx, opts CreateCardOptions) (*Card, error)` —
  POST `/api/lists/{listId}/cards` body `{name, description, position, type}` where
  type defaults "project" when empty; PROPAGATE; item.
- `func (c *Client) UpdateCard(ctx, id string, opts UpdateCardOptions) (*Card, error)` —
  PATCH `/api/cards/{id}` body of set fields; PROPAGATE; item.
- `func (c *Client) MoveCard(ctx, id, listID string, position float64, boardID, projectID *string) (*Card, error)` —
  PATCH `/api/cards/{id}` body `{listId, position}` + `boardId`/`projectId` only if non-nil;
  PROPAGATE; item.
- `func (c *Client) DuplicateCard(ctx, id string, position float64) (*Card, error)` —
  `orig,_ := GetCard(id)` (propagate its error); if `orig.ListID == ""` →
  error "Could not determine list ID for card duplication"; `CreateCard(CreateCardOptions{
  ListID: orig.ListID, Name: "Copy of "+orig.Name, Description: deref(orig.Description,""), Position: position, Type:""})`.
- `func (c *Client) DeleteCard(ctx, id string) (map[string]bool, error)` — DELETE
  `/api/cards/{id}`; PROPAGATE; `{"success":true}`.

### tools/card.go dispatch (index.ts requiredness; defaults differ per action)
- get_all: requireID("listId"); GetCards.
- create: require listId(requireID), name(requireName); `CreateCard(CreateCardOptions{ListID, Name, Description: deref(Description,""), Position: deref(Position,0), Type:""})`. (create defaults position 0, description "".)
- get_one: requireID("id"); GetCard.
- update: requireID("id"); UpdateCard with whichever of name/description/position/dueDate/isCompleted are set.
- move: require id(requireID), listId(requireID), Position present; `MoveCard(id, listId, *Position, BoardID, ProjectID)`.
- duplicate: require id(requireID), Position present; `DuplicateCard(id, *Position)`.
- delete: requireID("id"); DeleteCard.
- create_with_tasks: require listId(requireID), name(requireName); `createCardWithTasks(ctx, client, CreateCardWithTasksParams{ListID: listId, Name: name, Description, Tasks: deref(Tasks,nil), Comment, Position})` (composite stub exists).
- get_details: requireString("cardId"); `cardDetails(ctx, client, cardID)` (composite stub exists).

---

## 3. stopwatch  →  planka/stopwatch.go, tools/stopwatch.go

Tool: `mcp_kanban_stopwatch`  Desc: `Manage card stopwatches for time tracking`
Actions: `start, stop, get, reset`.  Layer-A IDs: `applyIDPattern("id")`.
NOTE: in this tool `id` is REQUIRED at the schema level (zod `plankaId` not
`.optional()`). So `Action string` + `ID string \`json:"id"\`` (non-pointer →
required). Still requireID("id", &args.ID) in each case for the numeric re-check.
`xArgs`: Action; ID (required string).

Stopwatch ops live here but call `c.GetCard` (cards.go) and PATCH `/api/cards/{id}`.
Use `time` package; format timestamps as `time.Now().UTC().Format(time.RFC3339)`;
elapsed seconds = `int(time.Since(parsed).Seconds())`, parsed via
`time.Parse(time.RFC3339, *startedAt)`.

### planka/stopwatch.go
- `func (c *Client) StartCardStopwatch(ctx, id string) (*Card, error)` — GetCard;
  `sw := map[string]any{"startedAt": nowISO, "total": existingTotal}` where
  existingTotal = card.Stopwatch.Total if card.Stopwatch != nil else 0; PATCH
  `/api/cards/{id}` body `{stopwatch: sw}`; return item. (wrap errors; propagate)
- `func (c *Client) StopCardStopwatch(ctx, id string) (*Card, error)` — GetCard;
  if card.Stopwatch == nil OR card.Stopwatch.StartedAt == nil → return the card
  unchanged (no PATCH); else elapsed = now - startedAt seconds; total = (existing
  total) + elapsed; PATCH body `{stopwatch:{startedAt: nil, total: total}}`; item.
- `func (c *Client) GetCardStopwatch(ctx, id string) (any, error)` — GetCard;
  if card.Stopwatch == nil → return `{isRunning:false, total:0, current:0,
  formattedTotal: formatDuration(0), formattedCurrent: formatDuration(0)}`.
  Else isRunning = StartedAt != nil; current = elapsed if running else 0; return
  `{isRunning, total: sw.Total, current, startedAt: sw.StartedAt, formattedTotal:
  formatDuration(sw.Total), formattedCurrent: formatDuration(current)}`. Use a
  map[string]any or a small struct to match these exact JSON keys.
- `func (c *Client) ResetCardStopwatch(ctx, id string) (*Card, error)` — PATCH
  `/api/cards/{id}` body `{stopwatch: nil}` (literal JSON null — use
  `map[string]any{"stopwatch": nil}`); item.
- `func formatDuration(totalSeconds float64) string` — ports formatDuration:
  h=floor(s/3600), m=floor((s%3600)/60), rem=s%60; build "Xh Ym Zs" (h only if
  h>0; m if m>0 or h>0; always s); TrimSpace. (Operate on int seconds.)

### tools/stopwatch.go dispatch
- start/stop/get/reset → the four methods, each `requireID("id", &args.ID)` first.

---

## 4. label  →  planka/labels.go, tools/label.go

Tool: `mcp_kanban_label_manager`  Desc: `Manage kanban labels with various operations`
Actions: `get_all, create, update, delete, add_to_card, remove_from_card`
Layer-A IDs: `applyIDPattern("id", "boardId", "cardId")`  (labelId is plain z.string — do NOT harden.)
Value enum: `applyEnum("color", labelColors)`.
`xArgs`: Action; ID*, BoardID*, CardID* (plankaId); LabelId* (plain); Name*; Color*; Position*float64.

### planka/labels.go
- `type CreateLabelOptions struct { BoardID, Name, Color string; Position float64 }`
- `type UpdateLabelOptions struct { Name, Color *string; Position *float64 }`
- `func (c *Client) CreateLabel(ctx, opts CreateLabelOptions) (*Label, error)` —
  POST `/api/boards/{boardId}/labels` body `{name, color, position}`; PROPAGATE; item.
- `func (c *Client) GetLabels(ctx, boardID string) ([]Label, error)` — GET
  `/api/boards/{boardId}`; return `included.Labels` or `[]Label{}` if absent;
  PROPAGATE request errors (NO try/catch in labels.ts getLabels).
- `func (c *Client) UpdateLabel(ctx, id string, opts UpdateLabelOptions) (*Label, error)` —
  PATCH `/api/labels/{id}` body of set fields; PROPAGATE; item.
- `func (c *Client) DeleteLabel(ctx, id string) (map[string]bool, error)` — DELETE
  `/api/labels/{id}`; PROPAGATE; `{"success":true}`.
- `func (c *Client) AddLabelToCard(ctx, cardID, labelID string) (map[string]bool, error)` —
  POST `/api/cards/{cardId}/card-labels` body `{labelId}`; PROPAGATE; `{"success":true}`.
- `func (c *Client) RemoveLabelFromCard(ctx, cardID, labelID string) (map[string]bool, error)` —
  DELETE `/api/cards/{cardId}/card-labels/labelId:{labelID}` — build by string
  concatenation; the literal `labelId:` colon must NOT be escaped (the client
  preserves it). PROPAGATE; `{"success":true}`. ADD A TEST asserting the raw path.

### tools/label.go dispatch
- get_all: requireID("boardId"); GetLabels.
- create: require boardId(requireID), name(requireName), color present (requireString), Position present; CreateLabel.
- update: require id(requireID), name(requireName), color present, Position present; UpdateLabel.
- delete: requireID("id"); DeleteLabel.
- add_to_card: require cardId(requireID), labelId(requireString); AddLabelToCard.
- remove_from_card: require cardId(requireID), labelId(requireString); RemoveLabelFromCard.

---

## 5. task  →  planka/tasks.go, tools/task.go

Tool: `mcp_kanban_task_manager`  Desc: `Manage kanban tasks with various operations`
Actions: `get_all, create, batch_create, get_one, update, delete, complete_task`
Layer-A IDs: `applyIDPattern("id", "cardId")`. The nested `tasks[].cardId` is also
plankaId in zod — set its pattern too: after inferSchema, the array item schema is
`schema.Properties["tasks"].Items`; set `.Properties["cardId"].Pattern = numericIDPattern`
(do this in an extra mutator; guard nils).
`xArgs`: Action; ID*, CardID* (plankaId); Name*; IsCompleted*bool; Position*float64;
Tasks*[]taskBatchItem where `taskBatchItem struct { CardID string \`json:"cardId"\`; Name string \`json:"name"\`; Position *float64 \`json:"position,omitempty"\` }`.

### planka/tasks.go  (v2.1 quirk: tasks nest under a task list)
- `func (c *Client) ensureTaskListID(ctx, cardID string) (string, error)` — GET
  `/api/cards/{cardId}`; if `included.TaskLists` non-empty → return `[0].ID`; else
  POST `/api/cards/{cardId}/task-lists` body `{name:"Tasks", position:65535}` →
  return created item id. PROPAGATE.
- `func (c *Client) CreateTask(ctx, cardID, name string, position float64) (*Task, error)` —
  `tlID,_ := ensureTaskListID(cardID)` (propagate); POST `/api/task-lists/{tlID}/tasks`
  body `{name, position}`; return item. (No taskCardId map needed in Go — getTask
  takes an explicit cardId; see below.)
- `func (c *Client) GetTasks(ctx, cardID string) ([]Task, error)` — GET
  `/api/cards/{cardId}`; return `included.Tasks` or `[]Task{}`; SWALLOW errors → `[]Task{}`.
- `func (c *Client) GetTask(ctx, id, cardID string) (*Task, error)` — if cardID=="" →
  error "Card ID is required to get a task. Either provide it directly or create
  the task first."; GET `/api/cards/{cardId}`; if no `included.Tasks` → error
  "Failed to get tasks for card {cardId}"; find task by id; if not found → error
  "Task with ID {id} not found in card {cardId}"; return it. PROPAGATE.
- `func (c *Client) UpdateTask(ctx, id string, name *string, isCompleted *bool, position *float64) (*Task, error)` —
  PATCH `/api/tasks/{id}` body of set fields; PROPAGATE; item.
- `func (c *Client) DeleteTask(ctx, id string) (map[string]bool, error)` — DELETE
  `/api/tasks/{id}`; PROPAGATE; `{"success":true}`.
- `func (c *Client) BatchCreateTasks(ctx, items []taskBatchItem-equivalent) (BatchResult, error)` —
  port batchCreateTasks: for each item i, default position to `65535*(i+1)` if
  position is nil/0; call CreateTask; collect into `{results, successes, failures}`
  where results[i] = `{success:true, result:task}` or `{success:false, error:{message}}`,
  successes = []Task, failures = `[{index, task, error}]`. Define exported result
  structs with matching JSON keys. Never aborts on a single failure. The planka
  function takes a slice of `{CardID string; Name string; Position *float64}` (define
  `type BatchTaskInput struct{...}` in planka).

### tools/task.go dispatch
- get_all: requireID("cardId"); GetTasks.
- create: require cardId(requireID), name(requireName); `CreateTask(cardID, name, deref(Position, 65535))`.
- batch_create: require Tasks non-empty (else "tasks array is required for batch_create action");
  map xArgs taskBatchItem → planka.BatchTaskInput; `BatchCreateTasks`.
- get_one: requireID("id"); `GetTask(id, "")` (no cardId in this tool → pass ""; GetTask errors per TS).
- update: requireID("id"); UpdateTask with set fields.
- complete_task: requireID("id"); `UpdateTask(id, nil, bp(true)... )` i.e. isCompleted=true only.
- delete: requireID("id"); DeleteTask.

---

## 6. comment  →  planka/comments.go, tools/comment.go

Tool: `mcp_kanban_comment_manager`  Desc: `Manage card comments with various operations`
Actions: `get_all, create, get_one, update, delete`
Layer-A IDs: `applyIDPattern("id", "cardId")`. No value enums.
`xArgs`: Action; ID*, CardID* (plankaId); Text*.

### planka/comments.go  (v2.1: first-class comments; no GET-by-id route)
- `func (c *Client) CreateComment(ctx, cardID, text string) (*Comment, error)` —
  POST `/api/cards/{cardId}/comments` body `{text}`; PROPAGATE; item.
- `func (c *Client) GetComments(ctx, cardID string) ([]Comment, error)` — GET
  `/api/cards/{cardId}/comments`; decode `listEnvelope[Comment]`; return items;
  PROPAGATE request errors (no try/catch around the request).
- `func (c *Client) GetComment(ctx, id string) (*Comment, error)` — board-walk
  (Planka has no GET-by-id): GET `/api/projects`; if no `included.Boards` → error
  "No boards found"; for each board GET `/api/boards/{id}`; for each card in its
  `included.Cards` GET `/api/cards/{cardId}/comments`; find comment by id; return
  it; if none across all → error "Comment not found: {id}". PROPAGATE.
- `func (c *Client) UpdateComment(ctx, id, text string) (*Comment, error)` — PATCH
  `/api/comments/{id}` body `{text}`; PROPAGATE; item.
- `func (c *Client) DeleteComment(ctx, id string) (map[string]bool, error)` — DELETE
  `/api/comments/{id}`; PROPAGATE; `{"success":true}`.

### tools/comment.go dispatch
- get_all: requireID("cardId"); GetComments.
- create: require cardId(requireID), text(requireString); CreateComment.
- get_one: requireID("id"); GetComment.
- update: require id(requireID), text(requireString); UpdateComment.
- delete: requireID("id"); DeleteComment.

---

## 7. membership  →  planka/memberships.go, tools/membership.go

Tool: `mcp_kanban_membership_manager`  Desc: `Manage board memberships with various operations`
Actions: `get_all, create, get_one, update, delete`
Layer-A IDs: `applyIDPattern("id", "boardId", "userId")`.
Value enum: `applyEnum("role", membershipRoles)` (editor|viewer).
`xArgs`: Action; ID*, BoardID*, UserID* (plankaId); Role*; CanComment*bool.

### planka/memberships.go
- `type CreateBoardMembershipOptions struct { BoardID, UserID, Role string }`
- `func (c *Client) CreateBoardMembership(ctx, opts CreateBoardMembershipOptions) (*BoardMembership, error)` —
  POST `/api/boards/{boardId}/board-memberships` body `{userId, role}`; PROPAGATE; item.
- `func (c *Client) GetBoardMemberships(ctx, boardID string) ([]BoardMembership, error)` —
  GET `/api/boards/{boardId}`; return `included.BoardMemberships` or `[]`; PROPAGATE
  request errors (no try/catch around the request).
- `func (c *Client) GetBoardMembership(ctx, id string) (*BoardMembership, error)` —
  GET `/api/board-memberships/{id}`; PROPAGATE; item.
- `func (c *Client) UpdateBoardMembership(ctx, id string, role *string, canComment *bool) (*BoardMembership, error)` —
  PATCH `/api/board-memberships/{id}` body of set fields {role?, canComment?};
  PROPAGATE; item.
- `func (c *Client) DeleteBoardMembership(ctx, id string) (map[string]bool, error)` —
  DELETE `/api/board-memberships/{id}`; PROPAGATE; `{"success":true}`.

### tools/membership.go dispatch
- get_all: requireID("boardId"); GetBoardMemberships.
- create: require boardId(requireID), userId(requireID), role present (requireString); CreateBoardMembership.
- get_one: requireID("id"); GetBoardMembership.
- update: requireID("id"); UpdateBoardMembership with set role/canComment.
- delete: requireID("id"); DeleteBoardMembership.

---

## Build expectation

After all files exist: `go build ./...` and `go vet ./...` must pass, and
`go test ./...` must pass. `boards.go` depends on `CreateList` (exists),
`CreateLabel` (label resource), `CreateBoardMembership` (membership resource);
`stopwatch.go` depends on `GetCard` (cards.go). These resolve once every file is
present — the signatures above are exact, so do not improvise alternatives.
