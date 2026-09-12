package tools

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cameronsjo/kanban-mcp/internal/config"
	"github.com/cameronsjo/kanban-mcp/internal/planka"
)

// schemaExpect is the locked contract for one manager's input schema, derived
// from index.ts. It is the golden reference the live schema must match so the
// action verbs, the ^\d+$ ID hardening (commit 93da5a3), and the value enums
// cannot silently drift from the zod contract.
type schemaExpect struct {
	actions          []string
	idPattern        []string // MUST carry ^\d+$
	noPattern        []string // MUST NOT carry a pattern (plain z.string in TS)
	enums            map[string][]string
	required         []string
	nestedTaskCardID bool
}

func goldenSchemas() map[string]schemaExpect {
	return map[string]schemaExpect{
		"mcp_kanban_project_board_manager": {
			actions:   []string{"get_projects", "get_project", "get_boards", "create_board", "get_board", "update_board", "delete_board", "get_board_summary"},
			idPattern: []string{"id", "projectId"},
			noPattern: []string{"boardId"},
			required:  []string{"action"},
		},
		"mcp_kanban_list_manager": {
			actions:   []string{"get_all", "create", "update", "delete", "get_one"},
			idPattern: []string{"id", "boardId"},
			enums:     map[string][]string{"type": listTypes},
			required:  []string{"action"},
		},
		"mcp_kanban_card_manager": {
			actions:   []string{"get_all", "create", "get_one", "update", "move", "duplicate", "delete", "create_with_tasks", "get_details"},
			idPattern: []string{"id", "listId"},
			noPattern: []string{"boardId", "projectId", "cardId"},
			required:  []string{"action"},
		},
		"mcp_kanban_stopwatch": {
			actions:   []string{"start", "stop", "get", "reset"},
			idPattern: []string{"id"},
			required:  []string{"action", "id"},
		},
		"mcp_kanban_label_manager": {
			actions:   []string{"get_all", "create", "update", "delete", "add_to_card", "remove_from_card"},
			idPattern: []string{"id", "boardId", "cardId"},
			noPattern: []string{"labelId"},
			enums:     map[string][]string{"color": labelColors},
			required:  []string{"action"},
		},
		"mcp_kanban_task_manager": {
			actions:          []string{"get_all", "create", "batch_create", "get_one", "update", "delete", "complete_task"},
			idPattern:        []string{"id", "cardId"},
			required:         []string{"action"},
			nestedTaskCardID: true,
		},
		"mcp_kanban_comment_manager": {
			actions:   []string{"get_all", "create", "get_one", "update", "delete"},
			idPattern: []string{"id", "cardId"},
			required:  []string{"action"},
		},
		"mcp_kanban_membership_manager": {
			actions:   []string{"get_all", "create", "get_one", "update", "delete"},
			idPattern: []string{"id", "boardId", "userId"},
			enums:     map[string][]string{"role": membershipRoles},
			required:  []string{"action"},
		},
	}
}

// listRegisteredTools registers every manager and lists them through an
// in-memory MCP session, returning each tool by name. This exercises the real
// AddTool path (schema inference + mutation) end to end.
func listRegisteredTools(t *testing.T) map[string]*mcp.Tool {
	t.Helper()
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	client := planka.NewClient(config.Config{AgentEmail: "a", AgentPassword: "b"}, "test")
	if err := RegisterAll(server, client); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}

	serverT, clientT := mcp.NewInMemoryTransports()
	go func() { _ = server.Run(ctx, serverT) }()

	mc := mcp.NewClient(&mcp.Implementation{Name: "tester", Version: "test"}, nil)
	sess, err := mc.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })

	res, err := sess.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	out := map[string]*mcp.Tool{}
	for _, tl := range res.Tools {
		out[tl.Name] = tl
	}
	return out
}

func props(t *testing.T, tool *mcp.Tool) map[string]any {
	t.Helper()
	schema, ok := tool.InputSchema.(map[string]any)
	if !ok {
		t.Fatalf("%s InputSchema is %T, want map[string]any", tool.Name, tool.InputSchema)
	}
	p, _ := schema["properties"].(map[string]any)
	return p
}

func fieldStr(p map[string]any, field, key string) (string, bool) {
	f, ok := p[field].(map[string]any)
	if !ok {
		return "", false
	}
	s, ok := f[key].(string)
	return s, ok
}

func fieldEnum(p map[string]any, field string) []string {
	f, ok := p[field].(map[string]any)
	if !ok {
		return nil
	}
	raw, ok := f["enum"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, len(raw))
	for i, v := range raw {
		out[i], _ = v.(string)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestToolSchemaGolden(t *testing.T) {
	tools := listRegisteredTools(t)
	golden := goldenSchemas()

	if len(tools) != len(golden) {
		t.Fatalf("registered %d tools, want %d: %v", len(tools), len(golden), keys(tools))
	}

	for name, exp := range golden {
		tool, ok := tools[name]
		if !ok {
			t.Errorf("tool %s not registered", name)
			continue
		}
		t.Run(name, func(t *testing.T) {
			p := props(t, tool)

			if got := fieldEnum(p, "action"); !equalStrings(got, exp.actions) {
				t.Errorf("action enum = %v, want %v", got, exp.actions)
			}
			for _, f := range exp.idPattern {
				if pat, _ := fieldStr(p, f, "pattern"); pat != `^\d+$` {
					t.Errorf("field %q pattern = %q, want ^\\d+$", f, pat)
				}
			}
			for _, f := range exp.noPattern {
				if pat, ok := fieldStr(p, f, "pattern"); ok {
					t.Errorf("field %q must NOT be hardened, has pattern %q", f, pat)
				}
			}
			for f, want := range exp.enums {
				if got := fieldEnum(p, f); !equalStrings(got, want) {
					t.Errorf("field %q enum = %v, want %v", f, got, want)
				}
			}
			if exp.required != nil {
				schema := tool.InputSchema.(map[string]any)
				req := toStrings(schema["required"])
				if !equalStringSet(req, exp.required) {
					t.Errorf("required = %v, want %v", req, exp.required)
				}
			}
			if exp.nestedTaskCardID {
				tasks, _ := p["tasks"].(map[string]any)
				items, _ := tasks["items"].(map[string]any)
				ip, _ := items["properties"].(map[string]any)
				if pat, _ := fieldStr(ip, "cardId", "pattern"); pat != `^\d+$` {
					t.Errorf("tasks[].cardId nested pattern = %q, want ^\\d+$", pat)
				}
			}
		})
	}
}

func keys(m map[string]*mcp.Tool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func toStrings(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, len(raw))
	for i, e := range raw {
		out[i], _ = e.(string)
	}
	return out
}

func equalStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]int{}
	for _, s := range a {
		seen[s]++
	}
	for _, s := range b {
		seen[s]--
	}
	for _, c := range seen {
		if c != 0 {
			return false
		}
	}
	return true
}
