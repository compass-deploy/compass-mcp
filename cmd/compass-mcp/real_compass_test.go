// Real-Compass e2e smoke. Skipped unless the operator opts in by
// setting COMPASS_URL + COMPASS_USERNAME + COMPASS_PASSWORD. When run,
// it launches the binary, drives the MCP protocol over stdio, and
// asserts at least one tool round-trips against the live api.
//
// Purpose: catch contract drift that the synthetic httptest-based
// tests cannot — e.g. the upstream renaming the session cookie, the
// /api/me response shape changing, the admin-login route being gated
// off. The synthetic smoke proves the wire protocol; this one proves
// the wire contract.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestRealCompass_WhoamiRoundTrip(t *testing.T) {
	url := os.Getenv("COMPASS_URL")
	user := os.Getenv("COMPASS_USERNAME")
	pass := os.Getenv("COMPASS_PASSWORD")
	if url == "" || user == "" || pass == "" {
		t.Skip("set COMPASS_URL, COMPASS_USERNAME, COMPASS_PASSWORD to run the real-Compass smoke")
	}

	bin := buildBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin)
	cmd.Env = []string{
		"COMPASS_URL=" + url,
		"COMPASS_USERNAME=" + user,
		"COMPASS_PASSWORD=" + pass,
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout: %v", err)
	}
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Wait()
		if stderr != nil {
			// Drain any subprocess complaints onto the test log so a
			// surprise auth failure or network error is visible.
			b, _ := bufio.NewReader(stderr).ReadString(0)
			if b != "" {
				t.Logf("stderr: %s", b)
			}
		}
	}()
	r := bufio.NewReader(stdout)

	send(t, stdin, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-11-25",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "real-compass-smoke", "version": "0.0"},
		},
	})
	initResp := recv(t, r)
	if got, _ := jpath(initResp, "result", "serverInfo", "name").(string); got != "compass-mcp" {
		t.Fatalf("initialize: serverInfo.name = %q, want compass-mcp\nraw=%s", got, initResp)
	}
	send(t, stdin, map[string]any{
		"jsonrpc": "2.0", "method": "notifications/initialized", "params": map[string]any{},
	})

	// Call whoami against the live api. Success criteria: tool returned
	// non-error AND the text fallback contains a "user" field. We don't
	// assert the user's value because the operator's credentials are
	// configured externally and may be any admin name.
	send(t, stdin, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/call",
		"params": map[string]any{"name": "whoami", "arguments": map[string]any{}},
	})
	callResp := recv(t, r)
	if isErr, _ := jpath(callResp, "result", "isError").(bool); isErr {
		t.Fatalf("whoami returned isError=true (likely auth or contract drift):\n%s", callResp)
	}
	content, _ := jpath(callResp, "result", "content").([]any)
	if len(content) == 0 {
		t.Fatalf("whoami returned empty content: %s", callResp)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, `"user"`) || !strings.Contains(text, `"authenticated"`) {
		t.Fatalf("whoami text missing expected /api/me fields — possible contract drift:\n%s", text)
	}
	// Confirm the JSON inside the fence actually parses; a malformed
	// fence would suggest the upstream returned something other than
	// the meResponse shape we expect.
	if i := strings.Index(text, "{"); i >= 0 {
		if j := strings.LastIndex(text, "}"); j > i {
			var probe map[string]any
			if err := json.Unmarshal([]byte(text[i:j+1]), &probe); err != nil {
				t.Errorf("whoami text fallback didn't parse as JSON: %v", err)
			}
		}
	}
}
