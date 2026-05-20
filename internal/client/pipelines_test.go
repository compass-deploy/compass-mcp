package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListPipelines_TrimsMetadata(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/admin-login", func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "compass_session", Value: "ok", Path: "/"})
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/pipelines", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{
			"kind":"PipelineList",
			"items":[
				{
					"metadata":{
						"name":"myapp",
						"creationTimestamp":"2026-05-01T10:00:00Z",
						"resourceVersion":"abc123",
						"managedFields":[{"manager":"compass-manager"}]
					},
					"status":{"namespace":"compass-myapp"}
				},
				{
					"metadata":{"name":"other","creationTimestamp":"2026-05-02T10:00:00Z"},
					"status":{"namespace":"compass-other"}
				}
			]
		}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := New(Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	got, err := c.ListPipelines(context.Background())
	if err != nil {
		t.Fatalf("ListPipelines: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 pipelines, got %d", len(got))
	}
	want := PipelineSummary{Name: "myapp", Namespace: "compass-myapp", CreationTimestamp: "2026-05-01T10:00:00Z"}
	if got[0] != want {
		t.Errorf("first pipeline: got %+v, want %+v", got[0], want)
	}
}

func TestListPipelines_EmptyList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/admin-login", func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "compass_session", Value: "ok", Path: "/"})
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/pipelines", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"items":[]}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := New(Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	got, err := c.ListPipelines(context.Background())
	if err != nil {
		t.Fatalf("ListPipelines: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty list, got %d items", len(got))
	}
}

func TestListPipelines_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/admin-login", func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "compass_session", Value: "ok", Path: "/"})
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/pipelines", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := New(Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	_, err := c.ListPipelines(context.Background())
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected 500 error, got %v", err)
	}
}
