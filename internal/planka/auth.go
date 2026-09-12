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
	logger.Debug("Preparing to login agent", "email", c.email, "baseURL", c.baseURL)

	url := c.baseURL + "/api/access-tokens"
	payload, err := json.Marshal(map[string]string{
		"emailOrUsername": c.email,
		"password":        c.password,
	})
	if err != nil {
		logger.Error("Failed to marshal login body", "email", c.email, "error", err.Error())
		return "", fmt.Errorf("marshal login body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		logger.Error("Failed to build login request", "email", c.email, "error", err.Error())
		return "", fmt.Errorf("build login request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		logger.Error("Failed to authenticate agent with Planka", "email", c.email, "error", err.Error())
		return "", fmt.Errorf("authenticate agent with Planka: %w", err)
	}
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		logger.Error("Failed to read login response", "email", c.email, "error", readErr.Error())
		return "", fmt.Errorf("read login response: %w", readErr)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var step struct {
			Step string `json:"step"`
		}
		if json.Unmarshal(body, &step) == nil && step.Step == "accept-terms" {
			logger.Warn("Login failed: terms of service not accepted", "email", c.email)
			return "", ErrTermsNotAccepted
		}
		logger.Warn("Login failed with HTTP error", "email", c.email, "statusCode", resp.StatusCode)
		return "", newAPIError(resp.StatusCode, body)
	}

	// The token is returned directly in the item field.
	var env struct {
		Item string `json:"item"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		logger.Error("Failed to decode login response", "email", c.email, "error", err.Error())
		return "", fmt.Errorf("decode login response: %w", err)
	}
	if env.Item == "" {
		logger.Error("Login response missing token", "email", c.email)
		return "", fmt.Errorf("login response missing token")
	}

	logger.Info("Successfully logged in agent", "email", c.email)
	return env.Item, nil
}

// getUserIDBy fetches the user list once and returns the ID of the first user
// matching pred, or "" when none match. A request error propagates so a failed
// lookup is distinguishable from "no such user".
func (c *Client) getUserIDBy(ctx context.Context, pred func(User) bool) (string, error) {
	var env listEnvelope[User]
	if err := c.Get(ctx, "/api/users", &env); err != nil {
		return "", err
	}
	for _, u := range env.Items {
		if pred(u) {
			return u.ID, nil
		}
	}
	return "", nil
}

// GetUserIDByEmail looks up a user ID by email ("", nil when not found).
func (c *Client) GetUserIDByEmail(ctx context.Context, email string) (string, error) {
	return c.getUserIDBy(ctx, func(u User) bool { return u.Email == email })
}

// GetUserIDByUsername looks up a user ID by username ("", nil when not found).
func (c *Client) GetUserIDByUsername(ctx context.Context, username string) (string, error) {
	return c.getUserIDBy(ctx, func(u User) bool { return u.Username == username })
}

// AdminUserID resolves and caches the admin user id. It tries, in order:
// PLANKA_ADMIN_ID, then a single /api/users fetch matched against
// PLANKA_ADMIN_EMAIL (priority) and PLANKA_ADMIN_USERNAME. It returns ("", nil)
// when none resolve, mirroring getAdminUserId in common/setup.ts; callers that
// need the admin (e.g. adding it as a board member) treat that as best-effort.
// Unlike the TS version it surfaces request errors instead of swallowing them.
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
	if c.adminEmail == "" && c.adminUsername == "" {
		return "", nil
	}

	// Fetch once; match email across all users first, then username.
	var env listEnvelope[User]
	if err := c.Get(ctx, "/api/users", &env); err != nil {
		return "", err
	}
	var id string
	if c.adminEmail != "" {
		for _, u := range env.Items {
			if u.Email == c.adminEmail {
				id = u.ID
				break
			}
		}
	}
	if id == "" && c.adminUsername != "" {
		for _, u := range env.Items {
			if u.Username == c.adminUsername {
				id = u.ID
				break
			}
		}
	}
	if id != "" {
		c.adminUserID = id
	}
	return id, nil
}
