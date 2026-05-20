package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/compass-deploy/compass-mcp/internal/client"
)

// TestWhoamiHandler_Integration drives the full chain: fake compass-api
// over httptest, real client.Client (cookie jar + admin-login flow),
// real tool handler producing the agent-visible CallToolResult. The MCP
// transport itself isn't in scope here — that's covered by the e2e
// smoke that launches the binary subprocess.
func TestWhoamiHandler_Integration(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/admin-login", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Username, Password string }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Username != "admin" || body.Password != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "compass_session", Value: "ok", Path: "/", HttpOnly: true})
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/me", func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie("compass_session"); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(client.Me{
			AuthEnabled: true, Authenticated: true, User: "admin",
			Groups: []string{"platform-admins"},
			Can: map[string]client.Capabilities{
				"myapp": {Promote: true, Approve: false, Invalidate: true},
			},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := client.New(client.Config{BaseURL: srv.URL, Username: "admin", Password: "secret"})
	if err != nil {
		t.Fatalf("client.New: %v", err)
	}
	h := whoamiHandler(c)

	res, err := h(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got IsError=true: %+v", res.Content)
	}

	text := textContent(t, res)
	for _, want := range []string{`"user": "admin"`, `"platform-admins"`, `"promote": true`, `"approve": false`} {
		if !strings.Contains(text, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, text)
		}
	}
}

func TestWhoamiHandler_BadCredsSurfacesAsToolError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/admin-login", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := client.New(client.Config{BaseURL: srv.URL, Username: "x", Password: "y"})
	if err != nil {
		t.Fatalf("client.New: %v", err)
	}
	h := whoamiHandler(c)

	res, err := h(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handler should not return Go error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true, got %+v", res.Content)
	}
}

// textContent extracts the concatenated text payload from a successful
// CallToolResult. The mcp-go API models Content as a typed interface,
// so we assert via the public TextContent type.
func textContent(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}
