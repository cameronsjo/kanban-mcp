# Changelog

## Unreleased

- Add a Go MCP server with shared Streamable HTTP and a stdio option while retaining the TypeScript server.
- Default HTTP to literal loopback addresses. Explicit remote/container mode requires a bearer token; configured authentication protects MCP and health endpoints.
- Preserve the fork’s numeric ID validation, URL credential redaction, safe operator-side terms guidance, and concurrent token-cache protections.
- Match label-write success, comment response/error handling, batch failure payloads, nullable system-list names, and stopped stopwatch timestamps to the compatibility contract.
- Add build/release definitions with pinned actions and scoped permissions. Publishing and migration remain separate operator actions.
