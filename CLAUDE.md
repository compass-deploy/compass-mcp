# compass-mcp — agent context

Project-specific guidance for Claude. Pair this with [README.md](README.md)
(what the tool is) and [compass-deploy/docs/](https://github.com/compass-deploy/compass-deploy/tree/main/docs)
(what the upstream is).

## What this is

MCP server that exposes Compass operations as tools an AI agent can
call. Runs locally as a subprocess of the agent over stdio. Talks to a
remote `compass-api` over HTTP. Holds the user's session and forwards
calls; all authz happens server-side via the existing Compass RBAC. No
k8s client here, no controllers — this is a thin HTTP client wrapped in
the MCP protocol.

## Layout

```
cmd/compass-mcp/      main.go — stdio MCP server entrypoint
internal/
├── client/           HTTP client to compass-api; admin-account auth + cookie cache
├── mcpsrv/           MCP server wiring (tool registration, transport)
└── tools/            one file per MCP tool; each is a thin shim over client/
```

Keep `client/` Compass-shaped: methods like `ListPipelines`,
`GetPromotion`. Keep `tools/` MCP-shaped: each tool defines its schema,
unpacks args, calls `client/`, formats the result for the agent.

## Tech stack

- Go (no specific version pin yet; whatever the team's local toolchain
  is, currently 1.25.0).
- [`github.com/mark3labs/mcp-go`](https://github.com/mark3labs/mcp-go)
  for the MCP protocol implementation. Stdio transport.
- Stdlib `net/http` for the Compass client. No frameworks.
- Standard `testing` + `httptest` for tests. No Ginkgo here.

## Conventions

Inherits the project-wide rules:

- **No defensive code.** Trust internal contracts (the client knows
  what the api returns). Only validate at the user-facing boundary
  (MCP tool arg parsing).
- **No emojis** anywhere (code, commits, docs).
- **Latest stable API versions** of libraries we depend on.
- **Comments only when crucial.** Self-explaining code first; comments
  carry the *why* when it's non-obvious (a workaround, a subtle
  invariant, a constraint imposed elsewhere).
- **Never set `--no-verify`, `--amend`, force-push** without an
  explicit ask.
- **Commit + push after each completed unit of work** (small,
  validated increments). Don't batch.

## Testing — non-negotiable

**Every change ships with tests.** No exceptions. The CI gate enforces
this; reviewers reject changes that miss it. Three layers, with the
right one chosen for each change:

### 1. Unit tests
Per-package, table-driven where possible. Cover:
- Happy path.
- Each error branch the code explicitly handles.
- Boundary conditions (empty input, zero values, max-length strings).

For the auth + HTTP client, that means: success, 401 (re-auth path),
404, 5xx, network error, body-parse failure.

### 2. Integration tests
Stand up a fake `compass-api` with `httptest.NewServer`. Each MCP tool
test runs the full request-response cycle: agent invokes tool → MCP
server parses args → calls our HTTP client → hits the fake api →
client decodes → tool formats response → assertions on the
agent-visible result. No mocks beyond the fake server.

Every new tool needs at least one integration test.

### 3. End-to-end smoke
A small test (or `make e2e`) that launches the real binary as a
subprocess, speaks the MCP protocol over stdio, and asserts at least
one tool round-trips against a real `compass-api`. Runs against a
locally-running Compass (host-mode or in-cluster); skipped when
`COMPASS_URL` env var is unset.

### Rules

- **Don't merge with failing tests.** `go test ./...` must be green
  locally before commit; CI re-runs and blocks merge if not.
- **One test at a time when debugging** — change one assumption, run,
  read output. Don't shotgun "maybe this fixes it" edits.
- **Reproduce before fixing.** If a bug is reported, write the failing
  test first. Then fix. Then commit both together.
- **Prove the root cause.** Bug-fix commit messages name the cause and
  cite evidence (log line, stack trace, network capture). Workarounds
  without root cause are not acceptable.

## Building incrementally

1. Make a focused change (one tool, one client method, one bug fix).
2. Add/update the corresponding tests.
3. Run `go test ./...`.
4. Build the binary, run it locally against your Compass, sanity-check
   one call via Claude Code or the equivalent.
5. Commit + push. Report what landed and propose the next step.

Don't bundle multiple tools or speculate ahead.

## Sharing types with compass-deploy

For V1: each type the client needs is duplicated as a small struct
under `internal/client/types.go`. The Compass HTTP API is stable enough
that duplication is cheaper than wiring a shared module. If drift
becomes painful, promote to a published `types` Go module under
compass-deploy.

## Common commands

```bash
go test ./...                # all tests
go test -race ./...          # race detector — required before commit on anything with goroutines
go build -o bin/compass-mcp ./cmd/compass-mcp
COMPASS_URL=http://compass.local COMPASS_USERNAME=admin COMPASS_PASSWORD=admin \
  ./bin/compass-mcp          # run the server with stdio (will sit waiting for input)
```

## Where we are right now

Bootstrap stage. No tools shipped yet. Building toward V1:

- M1 — MCP skeleton + admin-account auth + caching client.
- M2 — Four read-only tools: list_pipelines, list_promotions,
  get_promotion, get_promotion_logs.
- M3 — README walkthrough for Claude Code config + a smoke e2e.

Deferred from V1: OIDC device-flow auth (needed for production), write
tools (promote/approve/invalidate) with explicit user confirmation,
streaming log tail.
