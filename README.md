# compass-mcp

MCP (Model Context Protocol) server for [Compass Deploy](https://github.com/compass-deploy/compass-deploy).

Runs as a subprocess of an AI agent (Claude Code, Cursor, etc.) over
stdio. Authenticates to a Compass api server with the user's credentials
and exposes a small set of read-only tools that let the agent answer
questions like *"why did the last promotion to staging fail?"* or
*"which envs are running 1.0.3?"* — all bounded by the same RBAC the
user has in the UI.

## Status

Early. V1 is admin-account auth + a handful of read-only tools. OIDC
device-flow and mutation tools are deferred.

## Quick start

Build:

```bash
go build -o bin/compass-mcp ./cmd/compass-mcp
```

Configure your agent. For Claude Code, add to
`~/Library/Application Support/Claude/claude_desktop_config.json` (macOS):

```json
{
  "mcpServers": {
    "compass": {
      "command": "/absolute/path/to/bin/compass-mcp",
      "env": {
        "COMPASS_URL": "http://compass.local",
        "COMPASS_USERNAME": "admin",
        "COMPASS_PASSWORD": "admin"
      }
    }
  }
}
```

The agent then sees Compass tools available and can call them within the
auth context above. The MCP server can only do what that user can do.

## License

Apache 2.0 — see [LICENSE](LICENSE).
