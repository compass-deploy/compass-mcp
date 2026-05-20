package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/compass-deploy/compass-mcp/internal/client"
)

func TestListPipelinesHandler_Integration(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/admin-login", func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "compass_session", Value: "ok", Path: "/"})
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/pipelines", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"items":[
			{"metadata":{"name":"myapp","creationTimestamp":"2026-05-01T10:00:00Z"},"status":{"namespace":"compass-myapp"}},
			{"metadata":{"name":"other","creationTimestamp":"2026-05-02T10:00:00Z"},"status":{"namespace":"compass-other"}}
		]}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := client.New(client.Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	res, err := listPipelinesHandler(c)(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected IsError=true: %+v", res.Content)
	}

	got, ok := res.StructuredContent.([]client.PipelineSummary)
	if !ok {
		t.Fatalf("structured content not []PipelineSummary: %T", res.StructuredContent)
	}
	if len(got) != 2 || got[0].Name != "myapp" || got[0].Namespace != "compass-myapp" {
		t.Errorf("unexpected structured content: %+v", got)
	}

	text := textContent(t, res)
	if !strings.Contains(text, `"name": "myapp"`) || !strings.Contains(text, `"namespace": "compass-myapp"`) {
		t.Errorf("text fallback missing expected fields:\n%s", text)
	}
	// The text fallback must also be valid JSON inside the fence so a host
	// without StructuredContent support can parse it.
	if i := strings.Index(text, "["); i >= 0 {
		if j := strings.LastIndex(text, "]"); j > i {
			var decoded []client.PipelineSummary
			if err := json.Unmarshal([]byte(text[i:j+1]), &decoded); err != nil {
				t.Errorf("text fallback not valid JSON: %v", err)
			}
		}
	}
}

func TestListPipelinesHandler_UpstreamError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/admin-login", func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "compass_session", Value: "ok", Path: "/"})
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/pipelines", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "k8s exploded", http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := client.New(client.Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	res, err := listPipelinesHandler(c)(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handler should not return Go error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true on upstream 500")
	}
}
