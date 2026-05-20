package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// promotionsFixture is a small builder so each test stays focused on
// the behaviour it cares about. Closures over flags / captured params
// are used by individual tests when they need to assert request shape.
func promotionsFixture() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/admin-login", func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "compass_session", Value: "ok", Path: "/"})
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func TestListPromotions_TrimAndProjection(t *testing.T) {
	mux := promotionsFixture()
	mux.HandleFunc("GET /api/pipelines/myapp/promotions", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{
			"items":[
				{
					"metadata":{"name":"myapp-1.0.0-dev-abc","creationTimestamp":"2026-05-01T10:00:00Z","resourceVersion":"x"},
					"spec":{"environmentRef":"dev","bundleReleaseRef":"myapp-1.0.0","requestedBy":"shabbir"},
					"status":{"phase":"Succeeded","workflowRef":"wf-1","startedAt":"2026-05-01T10:00:05Z","completedAt":"2026-05-01T10:01:00Z"}
				}
			]
		}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := New(Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	got, err := c.ListPromotions(context.Background(), "myapp", ListPromotionsOpts{})
	if err != nil {
		t.Fatalf("ListPromotions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 promotion, got %d", len(got))
	}
	want := PromotionSummary{
		Name: "myapp-1.0.0-dev-abc", Environment: "dev", Release: "myapp-1.0.0",
		Phase: "Succeeded", RequestedBy: "shabbir", WorkflowRef: "wf-1",
		StartedAt: "2026-05-01T10:00:05Z", CompletedAt: "2026-05-01T10:01:00Z",
		CreationTimestamp: "2026-05-01T10:00:00Z",
	}
	if got[0] != want {
		t.Errorf("got %+v, want %+v", got[0], want)
	}
}

func TestListPromotions_FiltersForwardedAsQuery(t *testing.T) {
	var gotQuery string
	mux := promotionsFixture()
	mux.HandleFunc("GET /api/pipelines/myapp/promotions", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"items":[]}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := New(Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	_, err := c.ListPromotions(context.Background(), "myapp", ListPromotionsOpts{
		Environment: "staging",
		Release:     "myapp-1.0.0",
	})
	if err != nil {
		t.Fatalf("ListPromotions: %v", err)
	}
	if !strings.Contains(gotQuery, "environment=staging") || !strings.Contains(gotQuery, "release=myapp-1.0.0") {
		t.Errorf("expected env+release in query, got %q", gotQuery)
	}
}

func TestListPromotions_PipelineNameEscaped(t *testing.T) {
	var gotRequestURI string
	mux := promotionsFixture()
	// A pipeline with characters that need escaping shouldn't normally
	// exist (DNS-1123 forbids it), but the client must not blindly
	// concatenate user input into URLs. r.URL.Path is auto-decoded by
	// net/http; r.RequestURI carries the raw bytes off the wire.
	mux.HandleFunc("/api/pipelines/", func(w http.ResponseWriter, r *http.Request) {
		gotRequestURI = r.RequestURI
		_, _ = io.WriteString(w, `{"items":[]}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := New(Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	_, err := c.ListPromotions(context.Background(), "weird name", ListPromotionsOpts{})
	if err != nil {
		t.Fatalf("ListPromotions: %v", err)
	}
	if !strings.Contains(gotRequestURI, "weird%20name") {
		t.Errorf("expected wire URL to encode space, got %q", gotRequestURI)
	}
}

func TestListPromotions_MissingPipeline(t *testing.T) {
	c, _ := New(Config{BaseURL: "http://x", Username: "u", Password: "p"})
	_, err := c.ListPromotions(context.Background(), "", ListPromotionsOpts{})
	if err == nil {
		t.Fatal("expected error when pipeline is empty")
	}
}

func TestGetPromotion_RawPassthrough(t *testing.T) {
	mux := promotionsFixture()
	body := `{"metadata":{"name":"myapp-1.0.0-dev-abc","resourceVersion":"42"},"spec":{"environmentRef":"dev"},"status":{"phase":"Failed","conditions":[{"type":"Verified","status":"True"}]}}`
	mux.HandleFunc("GET /api/pipelines/myapp/promotions/myapp-1.0.0-dev-abc", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := New(Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	raw, err := c.GetPromotion(context.Background(), "myapp", "myapp-1.0.0-dev-abc")
	if err != nil {
		t.Fatalf("GetPromotion: %v", err)
	}
	// Raw passthrough means we still see resourceVersion and conditions —
	// the trimming behavior is intentionally absent here.
	var probe map[string]any
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatalf("returned JSON didn't decode: %v", err)
	}
	if meta, _ := probe["metadata"].(map[string]any); meta["resourceVersion"] != "42" {
		t.Errorf("expected raw resourceVersion=42, got %v", meta)
	}
	if status, _ := probe["status"].(map[string]any); status["conditions"] == nil {
		t.Error("expected raw conditions to pass through")
	}
}

func TestGetPromotion_NotFound(t *testing.T) {
	mux := promotionsFixture()
	mux.HandleFunc("GET /api/pipelines/myapp/promotions/missing", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := New(Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	_, err := c.GetPromotion(context.Background(), "myapp", "missing")
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected 404, got %v", err)
	}
}

func TestGetPromotion_MissingArgs(t *testing.T) {
	c, _ := New(Config{BaseURL: "http://x", Username: "u", Password: "p"})
	if _, err := c.GetPromotion(context.Background(), "", "x"); err == nil {
		t.Error("expected error on empty pipeline")
	}
	if _, err := c.GetPromotion(context.Background(), "p", ""); err == nil {
		t.Error("expected error on empty name")
	}
}
