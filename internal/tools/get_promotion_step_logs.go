package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/compass-deploy/compass-mcp/internal/client"
)

// RegisterGetPromotionStepLogs wires the get_promotion_step_logs tool.
// Returns the last 500 lines of the "main" container as plain text —
// not wrapped in JSON — because logs are inherently text and the agent
// shouldn't have to unwrap quoted/escaped JSON to read them.
func RegisterGetPromotionStepLogs(s *server.MCPServer, c *client.Client) {
	t := mcp.NewTool(
		"get_promotion_step_logs",
		mcp.WithDescription(
			"Fetch the last 500 lines of stdout/stderr from a single "+
				"promotion step. The step argument is the opaque nodeId "+
				"returned by list_promotion_steps. Logs are returned as "+
				"plain text.",
		),
		mcp.WithString("pipeline",
			mcp.Required(),
			mcp.Description("Pipeline name."),
		),
		mcp.WithString("promotion",
			mcp.Required(),
			mcp.Description("Promotion CR name."),
		),
		mcp.WithString("step",
			mcp.Required(),
			mcp.Description("Step nodeId from list_promotion_steps."),
		),
	)
	s.AddTool(t, getPromotionStepLogsHandler(c))
}

func getPromotionStepLogsHandler(c *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pipeline, err := req.RequireString("pipeline")
		if err != nil {
			return mcp.NewToolResultErrorf("argument: %v", err), nil
		}
		promotion, err := req.RequireString("promotion")
		if err != nil {
			return mcp.NewToolResultErrorf("argument: %v", err), nil
		}
		step, err := req.RequireString("step")
		if err != nil {
			return mcp.NewToolResultErrorf("argument: %v", err), nil
		}
		logs, err := c.GetPromotionStepLogs(ctx, pipeline, promotion, step)
		if err != nil {
			return upstreamError("get promotion step logs", err), nil
		}
		return mcp.NewToolResultText(logs), nil
	}
}
