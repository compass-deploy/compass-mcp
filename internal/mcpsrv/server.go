// Package mcpsrv wires the MCP server up to its tool set. It owns the
// MCPServer instance and the registration calls for each tool; cmd/main
// stays a thin process-lifecycle binding.
package mcpsrv

import (
	"github.com/mark3labs/mcp-go/server"

	"github.com/compass-deploy/compass-mcp/internal/client"
	"github.com/compass-deploy/compass-mcp/internal/tools"
)

const (
	Name    = "compass-mcp"
	Version = "0.1.0"
)

// New builds an MCPServer with every Compass tool registered. The
// client is shared across tools so the session cookie and re-auth state
// are reused.
func New(c *client.Client) *server.MCPServer {
	s := server.NewMCPServer(Name, Version)
	tools.RegisterWhoami(s, c)
	tools.RegisterListPipelines(s, c)
	return s
}

// ServeStdio blocks reading the MCP protocol off stdin and writing it to
// stdout. This is the only transport M1 supports.
func ServeStdio(s *server.MCPServer) error {
	return server.ServeStdio(s)
}
