# Changelog

## Unreleased

- Gate release publishing on race tests, vet and pinned vulnerability scanning; use the same pinned scanner in Go CI.
- Revalidate batch task card IDs, require remote bearer tokens of at least 32 characters, and refuse login redirects.
- Document TLS for LAN access and use per-user launchd log paths with explicit installation instructions.

- Require Go 1.26.9 or newer so builds include the patched standard library identified by the vulnerability scanner.

- Add a Go MCP server with shared Streamable HTTP and a stdio option while retaining the TypeScript server.
- Default HTTP to literal loopback addresses. Explicit remote/container mode requires a bearer token; configured authentication protects MCP and health endpoints.
- Preserve the fork’s numeric ID validation, URL credential redaction, safe operator-side terms guidance, and concurrent token-cache protections.
- Match label-write success, comment response/error handling and nullable timestamps, empty list results, batch failure payloads, nullable system-list names, and stopped stopwatch timestamps to the compatibility contract.
- Remember newly created tasks per client so subsequent task retrieval retains the TypeScript behavior under concurrent sessions.
- Add build/release definitions with pinned actions and scoped permissions. Publishing and migration remain separate operator actions.
