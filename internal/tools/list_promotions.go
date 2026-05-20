package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/compass-deploy/compass-mcp/internal/client"
)

// RegisterListPromotions wires the list_promotions tool. The agent
// must supply a pipeline (Compass scopes everything below the
// pipeline); environment and release are optional server-side filters
// that mirror the UI's promotion-list filtering.
func RegisterListPromotions(s *server.MCPServer, c *client.Client) {
	t := mcp.NewTool(
		"list_promotions",
		mcp.WithDescription(
			"List recent Promotions for a Compass Pipeline. Optionally "+
				"filter by environment name and/or bundle release name. "+
				"Each entry includes phase, requesting user, the workflow "+
				"that ran it, and started/completed timestamps.",
		),
		mcp.WithString("pipeline",
			mcp.Required(),
			mcp.Description("Pipeline name (e.g. \"myapp\"). Use list_pipelines to discover available pipelines."),
		),
		mcp.WithString("environment",
			mcp.Description("Optional: only return promotions targeting this Environment."),
		),
		mcp.WithString("release",
			mcp.Description("Optional: only return promotions of this BundleRelease name."),
		),
	)
	s.AddTool(t, listPromotionsHandler(c))
}

func listPromotionsHandler(c *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pipeline, err := req.RequireString("pipeline")
		if err != nil {
			return mcp.NewToolResultErrorf("argument: %v", err), nil
		}
		opts := client.ListPromotionsOpts{
			Environment: req.GetString("environment", ""),
			Release:     req.GetString("release", ""),
		}
		promotions, err := c.ListPromotions(ctx, pipeline, opts)
		if err != nil {
			return upstreamError("list promotions", err), nil
		}
		return structuredJSON(promotions)
	}
}
