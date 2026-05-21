package tools

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/compass-deploy/compass-mcp/internal/client"
)

func TestListPromotionStepsHandler_Success(t *testing.T) {
	mux := newAuthMux()
	mux.HandleFunc("GET /api/pipelines/myapp/promotions/p1/workflow", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"status":{"nodes":{
			"p1-render":{"type":"Pod","templateName":"render","displayName":"render","phase":"Succeeded","startedAt":"2026-05-01T10:00:00Z","finishedAt":"2026-05-01T10:01:00Z"},
			"p1-deploy":{"type":"Pod","templateName":"deploy","displayName":"deploy","phase":"Running","startedAt":"2026-05-01T10:02:00Z"}
		}}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := client.New(client.Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"pipeline": "myapp", "promotion": "p1"}
	res, err := listPromotionStepsHandler(c)(context.Background(), req)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected IsError: %+v", res.Content)
	}
	got, ok := res.StructuredContent.(listPromotionStepsResult)
	if !ok {
		t.Fatalf("structured content not listPromotionStepsResult: %T", res.StructuredContent)
	}
	if len(got.Steps) != 2 || got.Steps[0].NodeID != "p1-render" || got.Steps[1].NodeID != "p1-deploy" {
		t.Errorf("unexpected ordering/content: %+v", got)
	}
}

func TestListPromotionStepsHandler_MissingArg(t *testing.T) {
	c, _ := client.New(client.Config{BaseURL: "http://x", Username: "u", Password: "p"})
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"pipeline": "myapp"} // missing promotion
	res, err := listPromotionStepsHandler(c)(context.Background(), req)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true on missing promotion arg")
	}
}

func TestGetPromotionStepLogsHandler_Success(t *testing.T) {
	const wantLogs = "INFO starting render\nDEBUG resolved chart digest sha256:abc\n"
	mux := newAuthMux()
	mux.HandleFunc("GET /api/pipelines/myapp/promotions/p1/steps/p1-render/logs", func(w http.ResponseWriter, _ *http.Request) {
		// Encode through json so we exercise the same path the real API uses.
		_, _ = io.WriteString(w, `{"logs":"INFO starting render\nDEBUG resolved chart digest sha256:abc\n"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := client.New(client.Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"pipeline":  "myapp",
		"promotion": "p1",
		"step":      "p1-render",
	}
	res, err := getPromotionStepLogsHandler(c)(context.Background(), req)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected IsError: %+v", res.Content)
	}
	text := textContent(t, res)
	if text != wantLogs {
		t.Errorf("logs text mismatch.\ngot:  %q\nwant: %q", text, wantLogs)
	}
	// Logs must be plain text — explicitly NOT wrapped in JSON code fences
	// (agents reading logs shouldn't have to unwrap them).
	if strings.HasPrefix(text, "```") {
		t.Error("logs should not be code-fenced")
	}
}

func TestGetPromotionStepLogsHandler_MissingArg(t *testing.T) {
	c, _ := client.New(client.Config{BaseURL: "http://x", Username: "u", Password: "p"})
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"pipeline": "myapp", "promotion": "p1"} // missing step
	res, err := getPromotionStepLogsHandler(c)(context.Background(), req)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true on missing step arg")
	}
}
