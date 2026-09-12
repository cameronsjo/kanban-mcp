package planka

import (
	"context"
	"strings"
	"testing"
)

// TestDoRejectsPathTraversal locks the do() backstop: a path with a parent-dir
// segment is refused before any request reaches the wire, so a mis-validated ID
// can never traverse to another resource (normalizePath does not path.Clean).
func TestDoRejectsPathTraversal(t *testing.T) {
	m := &mockPlanka{tokens: []string{"t"}}
	c, _ := newTestClient(t, m)

	err := c.Get(context.Background(), "/api/cards/1/../../users", nil)
	if err == nil || !strings.Contains(err.Error(), "'..'") {
		t.Fatalf("expected path-traversal rejection, got %v", err)
	}
	if len(m.requests) != 0 {
		t.Fatalf("traversal path must not hit the wire, got %d requests", len(m.requests))
	}
}
