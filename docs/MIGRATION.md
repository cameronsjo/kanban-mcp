# Migrating from the per-session TypeScript server to the shared Go daemon

The TypeScript server is spawned **once per Claude Code session over stdio**.
The Go server provides **one** long-lived Streamable-HTTP daemon that sessions
can share, including its Planka token cache. Actual memory savings require
measurement with the intended workload.

## 1. Build / install the binary

```bash
# From source (this repo):
go build -o ./bin/kanban-mcp ./cmd/kanban-mcp

# Or, once released:
brew install cameronsjo/tap/kanban-mcp
```

## 2. Run the daemon (one process, loopback by default)

The env contract is unchanged from the TS server.

```bash
export PLANKA_BASE_URL=http://localhost:3000
export PLANKA_AGENT_EMAIL=...        # the shared agent identity
export PLANKA_AGENT_PASSWORD=...
export PLANKA_ADMIN_ID=...           # or PLANKA_ADMIN_EMAIL / PLANKA_ADMIN_USERNAME
# Optional: gate the HTTP transport behind a static bearer token.
export PLANKA_MCP_AUTH_TOKEN=...

kanban-mcp --transport http --addr 127.0.0.1:8900
```

Run it under launchd (macOS) for crash-restart + login-start — see
`deploy/launchd/com.cameronsjo.kanban-mcp.plist` — or as the profile-gated
`kanban-mcp` service in `docker-compose.yml` (`docker compose --profile mcp up -d`). The container explicitly opts into
remote binding and refuses startup unless `PLANKA_MCP_AUTH_TOKEN` is nonblank.

For an intentional LAN bind, set the token and pass `--allow-remote`:

```bash
kanban-mcp --transport http --addr 0.0.0.0:8900 --allow-remote
```

Without that flag, only literal loopback IPs (`127.0.0.1`, `[::1]`) are accepted.
The explicit mode requires a token even when its address is loopback. Both
`/mcp` and `/healthz` require the bearer header when a token is configured;
TLS and network access restrictions remain the operator’s responsibility.
Compose retains optional token interpolation so its default Planka-only profile
can be validated without MCP credentials; the MCP process enforces the requirement.

## 3. Switch the MCP config: N stdio entries → 1 HTTP entry

Remove the per-session stdio entry and register one user-scoped HTTP endpoint:

```bash
# Remove the old stdio server (name as registered, e.g. "kanban"):
claude mcp remove kanban

# Add the shared HTTP endpoint, user scope so every session reuses it:
claude mcp add --transport http --scope user kanban http://127.0.0.1:8900/mcp
```

If a bearer token is set, add the header:

```bash
claude mcp add --transport http --scope user kanban http://127.0.0.1:8900/mcp \
  --header "Authorization: Bearer $PLANKA_MCP_AUTH_TOKEN"
```

Tool names and the `action` discriminator are identical to the TS server, so
existing prompt scaffolding works unchanged.

## 4. Verify (evidence before claims)

Functional parity — exercise each manager and diff against the TS server, or use
the MCP Inspector pointed at the URL:

```bash
npx -y @modelcontextprotocol/inspector
# connect to http://127.0.0.1:8900/mcp
```

Memory acceptance (not measured by this change): with several sessions open,
measure the daemon’s resident memory before and after concurrent use:

```bash
pgrep -fl kanban-mcp                 # expect ONE pid
ps -o rss= -p "$(pgrep -f kanban-mcp | head -1)"   # measured KB
```

Compare against the measured summed RSS of the per-session TypeScript processes
under the same workload. Process count alone does not prove a memory saving.


## Acceptance limits

The TypeScript server remains available during migration. Local automated tests
use mocked Planka HTTP responses and exercise the real MCP HTTP transport; a live
Planka parity run and production migration have not been performed. The composite
board-summary path intentionally treats a comments endpoint 404 as no comments,
while other backend failures propagate. No release or deployment is part of this PR.
