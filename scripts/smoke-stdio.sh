#!/usr/bin/env bash
# Drives the kanban-mcp stdio transport through a minimal MCP handshake and
# prints the tools/list response. Used to smoke-test that tools register.
set -euo pipefail

BIN="${1:?usage: smoke-stdio.sh <binary>}"

{
  printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}'
  printf '%s\n' '{"jsonrpc":"2.0","method":"notifications/initialized"}'
  printf '%s\n' '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'
  sleep 0.3
} | "$BIN" --transport stdio
