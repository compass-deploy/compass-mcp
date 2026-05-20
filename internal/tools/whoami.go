// Package tools holds one file per MCP tool. Each tool defines its
// schema, unpacks arguments, calls into internal/client, and formats the
// result for the agent.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/compass-deploy/compass-mcp/internal/client"
)

// RegisterWhoami wires the whoami tool onto the given server. The tool
// is a diagnostic: it exercises the full auth chain (admin-login →
// session cookie → /api/me) so the operator can sanity-check the
// connection without invoking anything that mutates Compass state.
func RegisterWhoami(s *server.MCPServer, c *client.Client) {
	t := mcp.NewTool(
		"whoami",
		mcp.WithDescription(
			"Show the Compass user the MCP server is acting as, the groups "+
				"they belong to, and the per-pipeline capabilities they hold. "+
				"Use this to confirm the MCP server can reach Compass and is "+
				"authenticated correctly before invoking other tools.",
		),
	)
	s.AddTool(t, whoamiHandler(c))
}

func whoamiHandler(c *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		me, err := c.Me(ctx)
		if err != nil {
			return mcp.NewToolResultErrorf("compass /api/me failed: %v", err), nil
		}
		body, err := json.MarshalIndent(me, "", "  ")
		if err != nil {
			return mcp.NewToolResultErrorf("encode response: %v", err), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("```json\n%s\n```", body)), nil
	}
}
