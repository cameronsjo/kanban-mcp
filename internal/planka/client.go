// Package planka is the API layer for the Planka kanban backend. It ports
// common/utils.ts (request choke point, token handling, ToS detection) and the
// operations/*.ts modules into typed Go.
//
// Concurrency note (load-bearing): unlike the Node server — where the single-
// threaded event loop serialized every request for free — the MCP SDK dispatches
// tool handlers concurrently. The cached agent token is therefore guarded by a
// mutex and concurrent logins are collapsed with singleflight, so one shared
// Client can safely serve every session.
package planka

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/cameronsjo/kanban-mcp/internal/config"
)

var logger = slog.Default()

// Client is a Planka API client. It is safe for concurrent use and is intended
// to be constructed once and shared across all MCP sessions so the token cache
// persists process-wide.
type Client struct {
	baseURL       string
	email         string
	password      string
	adminID       string
	adminEmail    string
	adminUsername string

	http      *http.Client
	userAgent string

	// mu guards token; sf collapses concurrent logins.
	mu    sync.RWMutex
	token string
	sf    singleflight.Group

	// adminMu guards the lazily-resolved admin user id.
	adminMu     sync.Mutex
	adminUserID string
}

// NewClient builds a Client from configuration. version flows into the
// User-Agent header, matching the TS server's format.
func NewClient(cfg config.Config, version string) *Client {
	return &Client{
		baseURL:       cfg.BaseURL,
		email:         cfg.AgentEmail,
		password:      cfg.AgentPassword,
		adminID:       cfg.AdminID,
		adminEmail:    cfg.AdminEmail,
		adminUsername: cfg.AdminUsername,
		http:          &http.Client{Timeout: 30 * time.Second},
		userAgent:     fmt.Sprintf("modelcontextprotocol/servers/planka/v%s (go)", version),
	}
}

// SetHTTPClient swaps the underlying http.Client. Used by tests to point the
// client at an httptest server.
func (c *Client) SetHTTPClient(h *http.Client) { c.http = h }

// BaseURL returns the normalized Planka origin (no trailing /api).
func (c *Client) BaseURL() string { return c.baseURL }

// normalizePath ensures the request path is rooted at /api/, mirroring
// plankaRequest in common/utils.ts.
func normalizePath(path string) string {
	if strings.HasPrefix(path, "/api/") {
		return path
	}
	return "/api/" + strings.TrimPrefix(path, "/")
}

// do is the single authenticated-request choke point. It ensures a token,
// attaches it as a Bearer header, decodes a 2xx JSON body into out (out may be
// nil), and on a 401 drops the cached token and retries exactly once so a
// long-lived daemon self-heals from an expired or revoked token. Non-2xx
// responses become typed *APIError values.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	normPath := normalizePath(path)
	// Defense in depth: no legitimate Planka path contains a parent-directory
	// segment, so reject one outright. normalizePath deliberately does not
	// path.Clean, so this backstops any ID that reaches interpolation without a
	// numeric (^\d+$) check and stops cross-resource traversal at the wire.
	if strings.Contains(normPath, "..") {
		return &APIError{Status: http.StatusBadRequest, Kind: KindValidation, Message: "refusing request path containing '..'"}
	}
	url := c.baseURL + normPath
	logger.Debug("Preparing request", "method", method, "path", path)

	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			logger.Debug("Failed to marshal request body", "path", path, "error", err.Error())
			return fmt.Errorf("marshal request body: %w", err)
		}
	}

	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.ensureToken(ctx)
		if err != nil {
			return err
		}

		var reqBody io.Reader
		if payload != nil {
			reqBody = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
		if err != nil {
			logger.Debug("Failed to build request", "path", path, "error", err.Error())
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("User-Agent", c.userAgent)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := c.http.Do(req)
		if err != nil {
			logger.Debug("Request failed", "method", method, "path", path, "error", err.Error())
			return fmt.Errorf("planka request to %s: %w", url, err)
		}
		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			logger.Debug("Failed to read response", "path", path, "error", readErr.Error())
			return fmt.Errorf("read response from %s: %w", url, readErr)
		}

		// A cached token rejected on the first attempt: drop it (only if it is
		// still the one we used) and retry with a fresh login.
		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			logger.Debug("Token rejected, retrying after refresh", "path", path, "attempt", attempt)
			c.clearToken(token)
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			logger.Debug("Request failed with non-2xx status", "method", method, "path", path, "statusCode", resp.StatusCode)
			return newAPIError(resp.StatusCode, respBody)
		}

		if out != nil {
			if err := json.Unmarshal(respBody, out); err != nil {
				logger.Debug("Failed to decode response", "path", path, "error", err.Error())
				return fmt.Errorf("decode response from %s: %w", url, err)
			}
		}
		return nil
	}

	// Unreachable: the second attempt cannot 401-continue, so it always returns
	// or errors above.
	return &APIError{Status: http.StatusUnauthorized, Kind: KindAuth, Message: "authentication retry exhausted"}
}

// ensureToken returns the cached token, minting a new one under singleflight if
// the cache is empty so a burst of concurrent handlers triggers a single login.
func (c *Client) ensureToken(ctx context.Context) (string, error) {
	c.mu.RLock()
	t := c.token
	c.mu.RUnlock()
	if t != "" {
		logger.Debug("Using cached token")
		return t, nil
	}

	logger.Debug("Token not cached, acquiring via singleflight")
	v, err, _ := c.sf.Do("login", func() (any, error) {
		// Re-check inside the flight: another goroutine may have logged in while
		// this one waited for the flight to start.
		c.mu.RLock()
		t := c.token
		c.mu.RUnlock()
		if t != "" {
			logger.Debug("Token cached by concurrent goroutine")
			return t, nil
		}
		tok, err := c.login(ctx)
		if err != nil {
			return "", err
		}
		c.mu.Lock()
		c.token = tok
		c.mu.Unlock()
		return tok, nil
	})
	if err != nil {
		logger.Debug("Failed to ensure token", "error", err.Error())
		return "", err
	}
	return v.(string), nil
}

// clearToken drops the cached token only if it still matches used, so a 401
// from a stale token cannot clobber a token a concurrent goroutine just minted.
func (c *Client) clearToken(used string) {
	c.mu.Lock()
	if c.token == used {
		logger.Debug("Clearing cached token due to 401")
		c.token = ""
	}
	c.mu.Unlock()
}

// Get decodes GET path into out.
func (c *Client) Get(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

// Post sends body as JSON to path and decodes the response into out.
func (c *Client) Post(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPost, path, body, out)
}

// Patch sends body as JSON via PATCH to path and decodes the response into out.
func (c *Client) Patch(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPatch, path, body, out)
}

// Delete issues DELETE path, discarding any response body.
func (c *Client) Delete(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}
