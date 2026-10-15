# Finish the Go shared-server rewrite

Codex is finishing Cameron’s approved fork-only Go rewrite for PR #4. The TypeScript implementation stays available. No release, deployment, PR merge, or live Planka acceptance is authorized.

- [x] Merge `fork/planka-v2.1-compat` before implementation, preserving September security fixes.
- [x] Enforce loopback by default; explicit `--allow-remote` requires a nonblank bearer token before binding. Authenticate health checks when a token is configured. Align container usage and migration documentation.
- [x] Port September URL redaction, operator-only terms guidance, and numeric-ID schema protections to Go; preserve conditional token invalidation and singleflight.
- [x] Fix reviewed response parity: empty label success, comment errors/arrays, batch failure JSON/defaults, nullable system-list names, stopped stopwatch null timestamp. Preserve the documented composite 404-as-empty exception.
- [x] Pin new workflow actions and scope token permissions without relaxing checks or publishing anything.
- [ ] Prove regressions with deliberate red mutants and exact restoration; run build, lint, race tests, vulnerability scan, TypeScript compatibility checks, native configuration checks, and fresh independent review.
- [ ] Push only the existing fork feature branch by exact ref, verify remote SHA, update PR body, leave ready/open, inspect bounded CI, and write the orchestrator report.

Acceptance limits approved by Cameron: no live Planka instance is available, so parity is verified against current fork source/contracts and mocked HTTP fixtures; no production migration or claimed measured memory savings.
