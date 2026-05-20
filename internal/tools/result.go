package tools

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// structuredJSON wraps a tool response with both an MCP "structured"
// payload (consumed by hosts that support it) and a code-fenced JSON
// text fallback for the rest. Tools never write to stdout directly —
// they return through this helper so the agent-visible shape stays
// uniform.
func structuredJSON(v any) (*mcp.CallToolResult, error) {
	body, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultErrorf("encode response: %v", err), nil
	}
	return mcp.NewToolResultStructured(v, fmt.Sprintf("```json\n%s\n```", body)), nil
}

// upstreamError funnels client.* errors into an MCP tool error. Stdio
// MCP doesn't distinguish "tool failed" from "protocol failed" so we
// keep tool errors out-of-band by returning a CallToolResult with
// IsError=true rather than a Go error.
func upstreamError(label string, err error) *mcp.CallToolResult {
	return mcp.NewToolResultErrorf("%s: %v", label, err)
}
