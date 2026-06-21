package planka

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// login mints a Planka access token for the shared agent identity via
// POST /api/access-tokens. It ports authenticateAgent (common/utils.ts),
// including the Planka 2.1.x terms-of-service gate: a 403 carrying
// {"step":"accept-terms"} becomes the actionable ErrTermsNotAccepted rather
// than an opaque permission error. login does NOT cache; ensureToken owns the
// cache so the singleflight re-check and store stay in one place.
func (c *Client) login(ctx context.Context) (string, error) {
	url := c.baseURL + "/api/access-tokens"
	payload, err := json.Marshal(map[string]string{
		"emailOrUsername": c.email,
		"password":        c.password,
	})
	if err != nil {
		return "", fmt.Errorf("marshal login body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build login request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("authenticate agent with Planka: %w", err)
	}
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return "", fmt.Errorf("read login response: %w", readErr)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var step struct {
			Step string `json:"step"`
		}
		if json.Unmarshal(body, &step) == nil && step.Step == "accept-terms" {
			return "", ErrTermsNotAccepted
		}
		return "", newAPIError(resp.StatusCode, body)
	}

	// The token is returned directly in the item field.
	var env struct {
		Item string `json:"item"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return "", fmt.Errorf("decode login response: %w", err)
	}
	if env.Item == "" {
		return "", fmt.Errorf("login response missing token")
	}
	return env.Item, nil
}

// GetUserIDByEmail looks up a user ID by email. A request error propagates so a
// failed lookup is distinguishable from "no such user" (which returns "", nil).
func (c *Client) GetUserIDByEmail(ctx context.Context, email string) (string, error) {
	var env listEnvelope[User]
	if err := c.Get(ctx, "/api/users", &env); err != nil {
		return "", err
	}
	for _, u := range env.Items {
		if u.Email == email {
			return u.ID, nil
		}
	}
	return "", nil
}

// GetUserIDByUsername looks up a user ID by username (see GetUserIDByEmail).
func (c *Client) GetUserIDByUsername(ctx context.Context, username string) (string, error) {
	var env listEnvelope[User]
	if err := c.Get(ctx, "/api/users", &env); err != nil {
		return "", err
	}
	for _, u := range env.Items {
		if u.Username == username {
			return u.ID, nil
		}
	}
	return "", nil
}

// AdminUserID resolves and caches the admin user id. It tries, in order:
// PLANKA_ADMIN_ID, lookup by PLANKA_ADMIN_EMAIL, lookup by PLANKA_ADMIN_USERNAME.
// It returns ("", nil) when none resolve, mirroring getAdminUserId in
// common/setup.ts; callers that need the admin (e.g. adding it as a board
// member) treat that as best-effort. Unlike the TS version it surfaces request
// errors instead of swallowing them.
func (c *Client) AdminUserID(ctx context.Context) (string, error) {
	c.adminMu.Lock()
	defer c.adminMu.Unlock()

	if c.adminUserID != "" {
		return c.adminUserID, nil
	}
	if c.adminID != "" {
		c.adminUserID = c.adminID
		return c.adminUserID, nil
	}
	if c.adminEmail != "" {
		id, err := c.GetUserIDByEmail(ctx, c.adminEmail)
		if err != nil {
			return "", err
		}
		if id != "" {
			c.adminUserID = id
			return id, nil
		}
	}
	if c.adminUsername != "" {
		id, err := c.GetUserIDByUsername(ctx, c.adminUsername)
		if err != nil {
			return "", err
		}
		if id != "" {
			c.adminUserID = id
			return id, nil
		}
	}
	return "", nil
}
