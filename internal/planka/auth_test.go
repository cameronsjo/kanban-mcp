package planka

// Test Plan for internal/planka/auth.go
//
// AdminUserID (Classification: I/O boundary — reads from cache or /api/users)
//   [x] Happy/short-circuit: PLANKA_ADMIN_ID set → returns it with no API call
//   [x] Happy/email-priority: email match found before username match
//   [x] Happy/username-fallback: email not found → username used
//   [x] Happy/both-miss: neither email nor username in user list → ("", nil)
//   [x] Happy/none-configured: no admin fields set → ("", nil), no API call
//   [x] Unhappy: /api/users returns 500 → error propagates
//   [x] Caching: second call with same client returns cached value, /api/users called once
//
// getUserIDByEmail / getUserIDByUsername (Classification: thin wrappers — covered via AdminUserID)
//   Skipped as standalone: behaviour fully exercised by AdminUserID tests above.

import (
	"context"
	"testing"
)

func TestAdminUserIDFromAdminIDNoAPICall(t *testing.T) {
	// PLANKA_ADMIN_ID short-circuit: adminID set → return it without any API call.
	callCount := 0
	m := &mockPlanka{
		tokens: []string{"tok"},
		handler: func(token string, r recordedRequest) (int, string) {
			callCount++
			return 200, `{"items":[]}`
		},
	}
	c, _ := newTestClient(t, m)
	c.adminID = "direct-id-99"

	id, err := c.AdminUserID(context.Background())
	if err != nil {
		t.Fatalf("AdminUserID: %v", err)
	}
	if id != "direct-id-99" {
		t.Errorf("id = %q, want direct-id-99", id)
	}
	if callCount != 0 {
		t.Errorf("API call count = %d, want 0 — no request when adminID is set", callCount)
	}
	if m.loginCount != 0 {
		t.Errorf("loginCount = %d, want 0 — login implies an API call", m.loginCount)
	}
}

func TestAdminUserIDEmailPriorityOverUsername(t *testing.T) {
	// When both adminEmail and adminUsername are set and the same user matches
	// both, the first user whose email matches is returned (email beats username).
	m := &mockPlanka{
		tokens: []string{"tok"},
		handler: func(token string, r recordedRequest) (int, string) {
			if r.Path == "/api/users" {
				// Two users: one matches email, one shares the same username.
				return 200, `{"items":[
					{"id":"email-user","email":"admin@example.com","username":"shared-un"},
					{"id":"username-user","email":"other@x.com","username":"shared-un"}
				]}`
			}
			return 404, `{"message":"unexpected"}`
		},
	}
	c, _ := newTestClient(t, m)
	c.adminEmail = "admin@example.com"
	c.adminUsername = "shared-un"

	id, err := c.AdminUserID(context.Background())
	if err != nil {
		t.Fatalf("AdminUserID: %v", err)
	}
	if id != "email-user" {
		t.Errorf("id = %q, want email-user (email takes priority over username)", id)
	}
}

func TestAdminUserIDUsernameFallbackWhenEmailMiss(t *testing.T) {
	// adminEmail is configured but absent from the user list → fall through to
	// adminUsername which IS present.
	m := &mockPlanka{
		tokens: []string{"tok"},
		handler: func(token string, r recordedRequest) (int, string) {
			if r.Path == "/api/users" {
				return 200, `{"items":[{"id":"un-user","email":"other@x.com","username":"myuser"}]}`
			}
			return 404, `{"message":"unexpected"}`
		},
	}
	c, _ := newTestClient(t, m)
	c.adminEmail = "notfound@example.com"
	c.adminUsername = "myuser"

	id, err := c.AdminUserID(context.Background())
	if err != nil {
		t.Fatalf("AdminUserID: %v", err)
	}
	if id != "un-user" {
		t.Errorf("id = %q, want un-user (username fallback)", id)
	}
}

func TestAdminUserIDBothMissReturnsEmptyNil(t *testing.T) {
	// Neither adminEmail nor adminUsername matches any user → ("", nil).
	m := &mockPlanka{
		tokens: []string{"tok"},
		handler: func(token string, r recordedRequest) (int, string) {
			if r.Path == "/api/users" {
				return 200, `{"items":[{"id":"x","email":"other@x.com","username":"other"}]}`
			}
			return 404, `{"message":"unexpected"}`
		},
	}
	c, _ := newTestClient(t, m)
	c.adminEmail = "miss@example.com"
	c.adminUsername = "also-miss"

	id, err := c.AdminUserID(context.Background())
	if err != nil {
		t.Fatalf("AdminUserID: %v", err)
	}
	if id != "" {
		t.Errorf("id = %q, want empty string — neither email nor username matched", id)
	}
}

func TestAdminUserIDNoneConfiguredNoAPICall(t *testing.T) {
	// No admin fields set → early return ("", nil), no API call.
	callCount := 0
	m := &mockPlanka{
		tokens: []string{"tok"},
		handler: func(token string, r recordedRequest) (int, string) {
			callCount++
			return 200, `{"items":[]}`
		},
	}
	c, _ := newTestClient(t, m)
	// adminID / adminEmail / adminUsername all zero — newTestClient doesn't set them.

	id, err := c.AdminUserID(context.Background())
	if err != nil {
		t.Fatalf("AdminUserID: %v", err)
	}
	if id != "" {
		t.Errorf("id = %q, want empty", id)
	}
	if callCount != 0 {
		t.Errorf("API call count = %d, want 0 (no credentials configured)", callCount)
	}
}

func TestAdminUserIDRequestErrorPropagates(t *testing.T) {
	// /api/users returns 500 → error propagates (unlike the TS version that
	// swallows; the Go version surfaces request errors per the spec comment).
	m := &mockPlanka{
		tokens: []string{"tok"},
		handler: func(token string, r recordedRequest) (int, string) {
			if r.Path == "/api/users" {
				return 500, `{"message":"server error"}`
			}
			return 404, `{"message":"unexpected"}`
		},
	}
	c, _ := newTestClient(t, m)
	c.adminEmail = "admin@example.com"

	_, err := c.AdminUserID(context.Background())
	if err == nil {
		t.Fatal("expected error from /api/users 500, got nil")
	}
}

func TestAdminUserIDCachesResolvedID(t *testing.T) {
	// After a successful resolution, a second call must not hit /api/users again.
	usersCalled := 0
	m := &mockPlanka{
		tokens: []string{"tok"},
		handler: func(token string, r recordedRequest) (int, string) {
			if r.Path == "/api/users" {
				usersCalled++
				return 200, `{"items":[{"id":"cached-user","email":"a@b.com","username":""}]}`
			}
			return 200, `{"item":{}}`
		},
	}
	c, _ := newTestClient(t, m)
	c.adminEmail = "a@b.com"

	id1, err1 := c.AdminUserID(context.Background())
	id2, err2 := c.AdminUserID(context.Background())

	if err1 != nil || err2 != nil {
		t.Fatalf("AdminUserID errors: %v, %v", err1, err2)
	}
	if id1 != "cached-user" || id2 != "cached-user" {
		t.Errorf("ids = %q, %q, want cached-user both", id1, id2)
	}
	if usersCalled != 1 {
		t.Errorf("usersCalled = %d, want 1 (result cached after first call)", usersCalled)
	}
}
