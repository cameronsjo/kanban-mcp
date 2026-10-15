# Finish the Go shared-server rewrite

Codex is finishing Cameron’s approved fork-only Go rewrite for PR #4. The TypeScript implementation stays available. No release, deployment, PR merge, or live Planka acceptance is authorized.

- [x] Merge `fork/planka-v2.1-compat` before implementation, preserving September security fixes.
- [x] Enforce loopback by default; explicit `--allow-remote` requires a nonblank bearer token before binding. Authenticate health checks when a token is configured. Align container usage and migration documentation.
- [x] Port September URL redaction, operator-only terms guidance, and numeric-ID schema protections to Go; preserve conditional token invalidation and singleflight.
- [x] Fix reviewed response parity: empty label success, comment errors/arrays, batch failure JSON/defaults, nullable system-list names, stopped stopwatch null timestamp, created-task lookup, empty list arrays, and nullable comment timestamps. Preserve the documented composite 404-as-empty exception.
- [x] Pin new workflow actions and scope token permissions without relaxing checks or publishing anything.
- [x] Prove regressions with deliberate red mutants and exact restoration; run build, lint, race tests, vulnerability scan, TypeScript compatibility checks, native configuration checks, and fresh independent review.
- [x] Prepare fork-only exact-ref delivery and report artifacts, with ready/open state, final remote SHA, and bounded CI outcomes recorded in the orchestrator’s `report.md` after delivery.

Acceptance limits approved by Cameron: no live Planka instance is available, so parity is verified against current fork source/contracts and mocked HTTP fixtures; no production migration or claimed measured memory savings.

Validation: full Go race suite, vet/build/gofmt, four static target builds, vulnerability scan, TypeScript build/lint/61 unit tests, native Compose and GoReleaser configuration checks, SDK/CLI HTTP acceptance, and independent security/code finding closure passed. Regression mutants and exact restoration are recorded in the orchestrator report; paid/live backend acceptance remains excluded.

CI follow-up: both Go jobs at e17f8d2 correctly failed vulnerability scanning under the original Go 1.26.4 pin. Raised the minimum to the patched Go 1.26.9 release. Exact `GOTOOLCHAIN=go1.26.9` full race suite, vet/build, CLI acceptance, all four cross-builds, and the unchanged vulnerability scanner passed; matching gofmt was clean. The binary metadata confirms Go 1.26.9. Scanner and workflow checks remain unchanged.
