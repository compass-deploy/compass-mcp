// End-to-end smoke for the compass-mcp binary. Launches the real
// subprocess, points it at an in-test fake compass-api, and drives the
// MCP protocol over stdio: initialize -> tools/list -> tools/call
// whoami. This is the only test that exercises the stdio transport
// itself; the package-level unit + integration tests stop at the
// handler boundary.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStdioSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("smoke test builds the binary; skip with -short")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/admin-login", func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "compass_session", Value: "ok", Path: "/", HttpOnly: true})
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/me", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"authEnabled":   true,
			"authenticated": true,
			"user":          "admin",
			"groups":        []string{"platform-admins"},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	bin := buildBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin)
	cmd.Env = []string{
		"COMPASS_URL=" + srv.URL,
		"COMPASS_USERNAME=admin",
		"COMPASS_PASSWORD=secret",
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Wait()
	}()
	r := bufio.NewReader(stdout)

	// 1. initialize
	send(t, stdin, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-11-25",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "smoke", "version": "0.0"},
		},
	})
	initResp := recv(t, r)
	if got, _ := jpath(initResp, "result", "serverInfo", "name").(string); got != "compass-mcp" {
		t.Fatalf("initialize: serverInfo.name = %q, want compass-mcp\nraw=%s", got, initResp)
	}

	// MCP requires the client to confirm initialization before tool calls
	// are accepted. mark3labs/mcp-go enforces this on the server side.
	send(t, stdin, map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
		"params":  map[string]any{},
	})

	// 2. tools/list — must advertise every registered tool. Smoke against
	// the full set so a forgotten Register* call shows up here, not in
	// production.
	send(t, stdin, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]any{}})
	listResp := recv(t, r)
	tools, _ := jpath(listResp, "result", "tools").([]any)
	expected := map[string]bool{"whoami": false, "list_pipelines": false}
	for _, tt := range tools {
		if m, ok := tt.(map[string]any); ok {
			if name, _ := m["name"].(string); name != "" {
				if _, want := expected[name]; want {
					expected[name] = true
				}
			}
		}
	}
	for name, present := range expected {
		if !present {
			t.Fatalf("%s not in tools/list response: %s", name, listResp)
		}
	}

	// 3. tools/call whoami — must round-trip the impersonated user back
	send(t, stdin, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params":  map[string]any{"name": "whoami", "arguments": map[string]any{}},
	})
	callResp := recv(t, r)
	if isErr, _ := jpath(callResp, "result", "isError").(bool); isErr {
		t.Fatalf("whoami returned isError=true: %s", callResp)
	}
	content, _ := jpath(callResp, "result", "content").([]any)
	if len(content) == 0 {
		t.Fatalf("whoami returned empty content: %s", callResp)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, `"user": "admin"`) {
		t.Fatalf("whoami text missing user=admin:\n%s", text)
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "compass-mcp")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build binary: %v\n%s", err, out)
	}
	return bin
}

func send(t *testing.T, w io.Writer, msg any) {
	t.Helper()
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if _, err := fmt.Fprintf(w, "%s\n", b); err != nil {
		t.Fatalf("send: %v", err)
	}
}

func recv(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatalf("recv: %v (partial=%q)", err, line)
	}
	return strings.TrimSpace(line)
}

func jpath(raw string, keys ...any) any {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil
	}
	for _, k := range keys {
		switch kk := k.(type) {
		case string:
			m, ok := v.(map[string]any)
			if !ok {
				return nil
			}
			v = m[kk]
		case int:
			s, ok := v.([]any)
			if !ok || kk < 0 || kk >= len(s) {
				return nil
			}
			v = s[kk]
		}
	}
	return v
}
