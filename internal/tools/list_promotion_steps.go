package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/compass-deploy/compass-mcp/internal/client"
)

// RegisterListPromotionSteps wires the list_promotion_steps tool. The
// agent calls this to discover step names and node IDs before
// requesting any logs — the NodeID it returns is the argument
// get_promotion_step_logs takes.
func RegisterListPromotionSteps(s *server.MCPServer, c *client.Client) {
	t := mcp.NewTool(
		"list_promotion_steps",
		mcp.WithDescription(
			"List the executable steps (Pod-backed Argo Workflow nodes) for "+
				"a Promotion. Returns step name, opaque nodeId, phase, and "+
				"timestamps in execution order. The nodeId is what "+
				"get_promotion_step_logs takes to fetch a specific step's logs.",
		),
		mcp.WithString("pipeline",
			mcp.Required(),
			mcp.Description("Pipeline name."),
		),
		mcp.WithString("promotion",
			mcp.Required(),
			mcp.Description("Promotion CR name (from list_promotions)."),
		),
	)
	s.AddTool(t, listPromotionStepsHandler(c))
}

func listPromotionStepsHandler(c *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pipeline, err := req.RequireString("pipeline")
		if err != nil {
			return mcp.NewToolResultErrorf("argument: %v", err), nil
		}
		promotion, err := req.RequireString("promotion")
		if err != nil {
			return mcp.NewToolResultErrorf("argument: %v", err), nil
		}
		steps, err := c.ListPromotionSteps(ctx, pipeline, promotion)
		if err != nil {
			return upstreamError("list promotion steps", err), nil
		}
		return structuredJSON(steps)
	}
}
