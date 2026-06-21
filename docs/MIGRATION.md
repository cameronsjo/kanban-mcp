# Migrating from the per-session TypeScript server to the shared Go daemon

The TypeScript server is spawned **once per Claude Code session over stdio**.
Each Node process idles at ~50 MB RSS, so N concurrent sessions cost ~N × 50 MB.
The Go server replaces those N processes with **one** long-lived
Streamable-HTTP daemon every session shares — flat RSS regardless of session
count.

## 1. Build / install the binary

```bash
# From source (this repo):
go build -o ./bin/kanban-mcp ./cmd/kanban-mcp

# Or, once released:
brew install cameronsjo/tap/kanban-mcp
```

## 2. Run the daemon (one process, loopback only)

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
`kanban-mcp` service in `docker-compose.yml` (`docker compose --profile mcp up -d`).

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

Memory proof — the whole point. With several Claude sessions open against the Go
server, confirm exactly one process and flat RSS:

```bash
pgrep -fl kanban-mcp                 # expect ONE pid
ps -o rss= -p "$(pgrep -f kanban-mcp | head -1)"   # KB; stays ~12–20 MB flat
```

Compare against the TS baseline: summed RSS of the per-session `node` processes
(`pgrep -fl 'dist/index.js' | wc -l` × ~50 MB).
