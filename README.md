# compass-mcp

MCP (Model Context Protocol) server for [Compass Deploy](https://github.com/compass-deploy/compass-deploy).

Runs as a subprocess of an AI agent (Claude Code, Cursor, etc.) over
stdio. Authenticates to a Compass api server with the user's credentials
and exposes a small set of read-only tools that let the agent answer
questions like *"why did the last promotion to staging fail?"* or
*"which envs are running 1.0.3?"* — bounded by the same RBAC the user
has in the UI.

## Status

V1: admin-account auth + six read-only tools. OIDC device-flow and
write tools (promote / approve / invalidate) are deferred.

## Tools

| Name | What it does |
|---|---|
| `whoami` | Show the authenticated user, groups, and per-pipeline capabilities. Run this first to confirm the connection. |
| `list_pipelines` | List every Compass Pipeline the caller can see. |
| `list_promotions` | List Promotions for a Pipeline, optionally filtered by environment and/or release. |
| `get_promotion` | Fetch the full Promotion CR — spec, status, conditions, audit fields. Use this to diagnose failures. |
| `list_promotion_steps` | List the Pod-backed Argo Workflow steps for a Promotion. Returns step name + opaque `nodeId`. |
| `get_promotion_step_logs` | Last 500 lines of one step's logs. Takes the `nodeId` from `list_promotion_steps`. |

All RBAC is enforced server-side via Kubernetes user impersonation —
the MCP server can only do what the configured user can do.

## Quick start

Build the binary:

```bash
go build -o bin/compass-mcp ./cmd/compass-mcp
```

Register the server with Claude Code:

```bash
claude mcp add compass \
  -e COMPASS_URL=https://compass.example.com \
  -e COMPASS_USERNAME=admin \
  -e COMPASS_PASSWORD=your-admin-password \
  -- /absolute/path/to/bin/compass-mcp
```

For Cursor or Claude Desktop, drop the same shape into the agent's MCP
config file (`~/.cursor/mcp.json` for Cursor;
`~/Library/Application Support/Claude/claude_desktop_config.json` for
Claude Desktop on macOS):

```json
{
  "mcpServers": {
    "compass": {
      "command": "/absolute/path/to/bin/compass-mcp",
      "env": {
        "COMPASS_URL": "https://compass.example.com",
        "COMPASS_USERNAME": "admin",
        "COMPASS_PASSWORD": "your-admin-password"
      }
    }
  }
}
```

Restart the agent, then ask it to run `whoami` to confirm. From there
the agent can discover and call any of the tools above.

## Configuration

Configuration comes from environment variables the MCP host passes
into the subprocess:

| Variable | Required | Purpose |
|---|---|---|
| `COMPASS_URL` | yes | Base URL of the Compass api server (e.g. `https://compass.example.com`). No trailing slash. |
| `COMPASS_USERNAME` | yes | Admin account username on the Compass server. |
| `COMPASS_PASSWORD` | yes | Admin account password. |

The server lazily logs in on the first tool call, caches the session
cookie in-process, and re-auths once on a 401 so long-running agent
sessions survive a JWT expiry mid-conversation.

## Development

```bash
go test ./...                # unit + integration + synthetic stdio smoke
go test -race ./...          # required before commit
go build -o bin/compass-mcp ./cmd/compass-mcp
```

End-to-end smoke against a real Compass:

```bash
COMPASS_URL=http://localhost:8080 \
COMPASS_USERNAME=admin COMPASS_PASSWORD=admin \
  go test ./cmd/compass-mcp -run TestRealCompass -v
```

The real-Compass test skips automatically when `COMPASS_URL` is unset,
so the default `go test ./...` stays hermetic.

## License

Apache 2.0 — see [LICENSE](LICENSE).
