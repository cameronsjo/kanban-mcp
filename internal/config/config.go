// Package config parses the kanban-mcp server configuration from environment
// variables and command-line flags.
//
// The Planka credentials mirror the TypeScript server's env contract exactly
// (PLANKA_BASE_URL, PLANKA_AGENT_EMAIL/PASSWORD, PLANKA_ADMIN_ID/EMAIL/USERNAME)
// so the MCP config can be migrated without touching secrets. The transport,
// addr and auth-token fields are new surface introduced by the shared
// long-lived HTTP server model.
package config

import (
	"errors"
	"os"
	"strings"
)

// Transport selects how the server speaks MCP.
const (
	TransportStdio = "stdio"
	TransportHTTP  = "http"
)

// Config is the fully-resolved server configuration.
type Config struct {
	// BaseURL is the Planka origin, normalized to never end in "/api".
	BaseURL string
	// AgentEmail and AgentPassword are the shared agent identity used to mint
	// Planka access tokens. Required.
	AgentEmail    string
	AgentPassword string

	// Admin identity resolution, tried in order: ID, then Email, then Username.
	AdminID       string
	AdminEmail    string
	AdminUsername string

	// Transport is "stdio" or "http".
	Transport string
	// Addr is the HTTP listen address (http transport only). Bound to loopback
	// by default because the process holds Planka admin credentials.
	Addr string
	// AuthToken, when set, gates the HTTP transport behind a static
	// "Authorization: Bearer <token>" header.
	AuthToken string
}

// DefaultBaseURL matches the TypeScript server's fallback.
const DefaultBaseURL = "http://localhost:3000"

// DefaultAddr binds loopback only — the daemon holds admin creds and must never
// listen on a routable interface.
const DefaultAddr = "127.0.0.1:8900"

// normalizeBaseURL strips a trailing "/api" so request paths can be joined
// unambiguously, mirroring common/utils.ts.
func normalizeBaseURL(raw string) string {
	if raw == "" {
		raw = DefaultBaseURL
	}
	return strings.TrimSuffix(strings.TrimRight(raw, "/"), "/api")
}

// FromEnv reads the configuration from the process environment, applying the
// same defaults as the TypeScript server. Flag overrides are layered on top by
// the caller (see cmd/kanban-mcp).
func FromEnv() Config {
	return Config{
		BaseURL:       normalizeBaseURL(os.Getenv("PLANKA_BASE_URL")),
		AgentEmail:    os.Getenv("PLANKA_AGENT_EMAIL"),
		AgentPassword: os.Getenv("PLANKA_AGENT_PASSWORD"),
		AdminID:       os.Getenv("PLANKA_ADMIN_ID"),
		AdminEmail:    os.Getenv("PLANKA_ADMIN_EMAIL"),
		AdminUsername: os.Getenv("PLANKA_ADMIN_USERNAME"),
		Transport:     TransportStdio,
		Addr:          DefaultAddr,
		AuthToken:     os.Getenv("PLANKA_MCP_AUTH_TOKEN"),
	}
}

// ErrMissingCredentials is returned by Validate when the agent identity is unset.
var ErrMissingCredentials = errors.New("PLANKA_AGENT_EMAIL and PLANKA_AGENT_PASSWORD environment variables are required")

// Validate checks that the configuration is usable.
func (c Config) Validate() error {
	if c.AgentEmail == "" || c.AgentPassword == "" {
		return ErrMissingCredentials
	}
	return nil
}
