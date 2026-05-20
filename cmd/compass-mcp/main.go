// compass-mcp is a stdio MCP server fronting compass-api. An MCP host
// (Claude Code, Cursor, ...) launches this binary as a subprocess and
// talks JSON-RPC over stdin/stdout. Configuration comes from env vars:
//
//	COMPASS_URL       e.g. https://compass.example.com
//	COMPASS_USERNAME  admin account username
//	COMPASS_PASSWORD  admin account password
package main

import (
	"fmt"
	"os"

	"github.com/compass-deploy/compass-mcp/internal/client"
	"github.com/compass-deploy/compass-mcp/internal/mcpsrv"
)

func main() {
	c, err := client.NewFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := mcpsrv.ServeStdio(mcpsrv.New(c)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
