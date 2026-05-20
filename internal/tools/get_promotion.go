package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/compass-deploy/compass-mcp/internal/client"
)

// RegisterGetPromotion wires the get_promotion tool. Returns the raw
// Promotion CR (full fidelity, including conditions and audit fields)
// because the typical caller wants to debug a failure — context budget
// matters less for one record and we shouldn't hide diagnostic fields.
func RegisterGetPromotion(s *server.MCPServer, c *client.Client) {
	t := mcp.NewTool(
		"get_promotion",
		mcp.WithDescription(
			"Fetch the full Compass Promotion CR by name, including its "+
				"spec, status, conditions, and audit fields. Use this to "+
				"diagnose why a promotion failed or stalled.",
		),
		mcp.WithString("pipeline",
			mcp.Required(),
			mcp.Description("Pipeline name the Promotion belongs to."),
		),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Promotion CR name (e.g. \"myapp-1.0.0-staging-abc12\"). Discover via list_promotions."),
		),
	)
	s.AddTool(t, getPromotionHandler(c))
}

func getPromotionHandler(c *client.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pipeline, err := req.RequireString("pipeline")
		if err != nil {
			return mcp.NewToolResultErrorf("argument: %v", err), nil
		}
		name, err := req.RequireString("name")
		if err != nil {
			return mcp.NewToolResultErrorf("argument: %v", err), nil
		}
		raw, err := c.GetPromotion(ctx, pipeline, name)
		if err != nil {
			return upstreamError("get promotion", err), nil
		}
		// Re-encode through indent for the text fallback. Decoding the raw
		// message into `any` first lets MarshalIndent format it; passing
		// the RawMessage directly to MarshalIndent emits a single line.
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return mcp.NewToolResultErrorf("decode promotion: %v", err), nil
		}
		body, err := json.MarshalIndent(decoded, "", "  ")
		if err != nil {
			return mcp.NewToolResultErrorf("encode promotion: %v", err), nil
		}
		return mcp.NewToolResultStructured(decoded, fmt.Sprintf("```json\n%s\n```", body)), nil
	}
}
