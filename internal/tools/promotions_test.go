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

func newAuthMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/admin-login", func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "compass_session", Value: "ok", Path: "/"})
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func TestListPromotionsHandler_Success(t *testing.T) {
	var gotQuery string
	mux := newAuthMux()
	mux.HandleFunc("GET /api/pipelines/myapp/promotions", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"items":[
			{
				"metadata":{"name":"p1","creationTimestamp":"2026-05-01T10:00:00Z"},
				"spec":{"environmentRef":"dev","bundleReleaseRef":"myapp-1.0.0","requestedBy":"alice"},
				"status":{"phase":"Succeeded"}
			}
		]}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := client.New(client.Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	req := mcp.CallToolRequest{}
	req.Params.Name = "list_promotions"
	req.Params.Arguments = map[string]any{
		"pipeline":    "myapp",
		"environment": "dev",
	}
	res, err := listPromotionsHandler(c)(context.Background(), req)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected IsError=true: %+v", res.Content)
	}
	got, ok := res.StructuredContent.(listPromotionsResult)
	if !ok {
		t.Fatalf("structured content not listPromotionsResult: %T", res.StructuredContent)
	}
	if len(got.Promotions) != 1 || got.Promotions[0].Environment != "dev" || got.Promotions[0].RequestedBy != "alice" {
		t.Errorf("unexpected structured: %+v", got)
	}
	if !strings.Contains(gotQuery, "environment=dev") {
		t.Errorf("expected environment filter forwarded, got query %q", gotQuery)
	}
}

func TestListPromotionsHandler_MissingPipelineArg(t *testing.T) {
	c, _ := client.New(client.Config{BaseURL: "http://x", Username: "u", Password: "p"})
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}
	res, err := listPromotionsHandler(c)(context.Background(), req)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true when pipeline arg missing")
	}
}

func TestGetPromotionHandler_RawFidelity(t *testing.T) {
	mux := newAuthMux()
	mux.HandleFunc("GET /api/pipelines/myapp/promotions/p1", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{
			"metadata":{"name":"p1","resourceVersion":"42"},
			"spec":{"environmentRef":"prod","bundleReleaseRef":"myapp-1.2.0","requestedBy":"bob"},
			"status":{"phase":"Failed","conditions":[{"type":"Verified","status":"False","reason":"NotYetTested"}]}
		}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := client.New(client.Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"pipeline": "myapp", "name": "p1"}
	res, err := getPromotionHandler(c)(context.Background(), req)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected IsError=true: %+v", res.Content)
	}
	// Raw fidelity means the text fallback should contain the conditions
	// block from the upstream — we promised to preserve diagnostic detail
	// on single-record gets.
	text := textContent(t, res)
	for _, want := range []string{`"resourceVersion": "42"`, `"reason": "NotYetTested"`, `"phase": "Failed"`} {
		if !strings.Contains(text, want) {
			t.Errorf("expected %q in text fallback, got:\n%s", want, text)
		}
	}
}

func TestGetPromotionHandler_NotFoundSurfacesAsToolError(t *testing.T) {
	mux := newAuthMux()
	mux.HandleFunc("GET /api/pipelines/myapp/promotions/missing", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := client.New(client.Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"pipeline": "myapp", "name": "missing"}
	res, err := getPromotionHandler(c)(context.Background(), req)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError=true on 404")
	}
}
