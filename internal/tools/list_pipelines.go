package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/compass-deploy/compass-mcp/internal/client"
)

// RegisterListPipelines wires the list_pipelines tool: no arguments,
// returns every Pipeline the caller can see as a flat summary array.
func RegisterListPipelines(s *server.MCPServer, c *client.Client) {
	t := mcp.NewTool(
		"list_pipelines",
		mcp.WithDescription(
			"List every Compass Pipeline the authenticated user has read "+
				"access to. Returns one entry per Pipeline with its name, "+
				"managed namespace, and creation timestamp.",
		),
	)
	s.AddTool(t, listPipelinesHandler(c))
}

func listPipelinesHandler(c *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pipelines, err := c.ListPipelines(ctx)
		if err != nil {
			return upstreamError("list pipelines", err), nil
		}
		return structuredJSON(pipelines)
	}
}
