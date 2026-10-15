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
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
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
	// AllowRemote explicitly enables non-loopback HTTP with mandatory bearer auth.
	AllowRemote bool
}

// DefaultBaseURL matches the TypeScript server's fallback.
const DefaultBaseURL = "http://localhost:3000"

// DefaultAddr binds loopback only — the daemon holds admin creds and must never
// listen on a routable interface without explicit authenticated opt-in.
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

// ValidateHTTP rejects unsafe or malformed listener configurations before binding.
// Literal loopback addresses avoid DNS changes turning a local bind into remote exposure.
func (c Config) ValidateHTTP() error {
	host, port, err := net.SplitHostPort(c.Addr)
	if err != nil {
		return fmt.Errorf("invalid HTTP listen address: %w", err)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 0 || n > 65535 {
		return errors.New("HTTP listen port must be a number between 0 and 65535")
	}
	if strings.TrimSpace(host) != host {
		return errors.New("HTTP listen host must not contain surrounding whitespace")
	}
	if c.AllowRemote {
		if utf8.RuneCountInString(strings.TrimSpace(c.AuthToken)) < 32 {
			return errors.New("--allow-remote requires PLANKA_MCP_AUTH_TOKEN with at least 32 characters after trimming surrounding whitespace")
		}
	} else {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return errors.New("HTTP listen address must be a literal loopback IP; remote/container binds require --allow-remote and PLANKA_MCP_AUTH_TOKEN")
		}
	}
	return nil
}
