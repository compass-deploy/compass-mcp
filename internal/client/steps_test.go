package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func stepsFixture() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/admin-login", func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "compass_session", Value: "ok", Path: "/"})
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func TestListPromotionSteps_FiltersPodNodesAndSortsByStart(t *testing.T) {
	mux := stepsFixture()
	mux.HandleFunc("GET /api/pipelines/myapp/promotions/p1/workflow", func(w http.ResponseWriter, _ *http.Request) {
		// Three nodes: a DAG entry (filtered), an early-started Pod, a
		// late-started Pod. Expect: 2 pods, early first.
		_, _ = io.WriteString(w, `{
			"status":{
				"nodes":{
					"p1-dag-1": {"type":"DAG","displayName":"main"},
					"p1-pod-late": {"type":"Pod","templateName":"deploy","displayName":"deploy","phase":"Running","startedAt":"2026-05-01T10:05:00Z"},
					"p1-pod-early": {"type":"Pod","templateName":"render","displayName":"render","phase":"Succeeded","startedAt":"2026-05-01T10:00:00Z","finishedAt":"2026-05-01T10:01:00Z"}
				}
			}
		}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := New(Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	got, err := c.ListPromotionSteps(context.Background(), "myapp", "p1")
	if err != nil {
		t.Fatalf("ListPromotionSteps: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 Pod nodes, got %d: %+v", len(got), got)
	}
	if got[0].NodeID != "p1-pod-early" || got[0].Step != "render" {
		t.Errorf("expected early Pod first, got %+v", got[0])
	}
	if got[1].NodeID != "p1-pod-late" || got[1].Phase != "Running" {
		t.Errorf("expected late Pod second, got %+v", got[1])
	}
}

func TestListPromotionSteps_PendingStepsSortLast(t *testing.T) {
	mux := stepsFixture()
	mux.HandleFunc("GET /api/pipelines/myapp/promotions/p1/workflow", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{
			"status":{
				"nodes":{
					"p1-pending":{"type":"Pod","templateName":"validate","phase":"Pending"},
					"p1-done":{"type":"Pod","templateName":"build","phase":"Succeeded","startedAt":"2026-05-01T09:00:00Z","finishedAt":"2026-05-01T09:01:00Z"}
				}
			}
		}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := New(Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	got, _ := c.ListPromotionSteps(context.Background(), "myapp", "p1")
	if len(got) != 2 || got[0].NodeID != "p1-done" || got[1].NodeID != "p1-pending" {
		t.Errorf("expected pending step last, got %+v", got)
	}
}

func TestListPromotionSteps_NoWorkflowYet(t *testing.T) {
	mux := stepsFixture()
	mux.HandleFunc("GET /api/pipelines/myapp/promotions/p1/workflow", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"no workflow has been materialized yet"}`, http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := New(Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	_, err := c.ListPromotionSteps(context.Background(), "myapp", "p1")
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected 404, got %v", err)
	}
}

func TestListPromotionSteps_MissingArgs(t *testing.T) {
	c, _ := New(Config{BaseURL: "http://x", Username: "u", Password: "p"})
	if _, err := c.ListPromotionSteps(context.Background(), "", "p"); err == nil {
		t.Error("expected error on empty pipeline")
	}
	if _, err := c.ListPromotionSteps(context.Background(), "p", ""); err == nil {
		t.Error("expected error on empty promotion")
	}
}

func TestGetPromotionStepLogs_Success(t *testing.T) {
	mux := stepsFixture()
	mux.HandleFunc("GET /api/pipelines/myapp/promotions/p1/steps/p1-pod-early/logs", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"logs":"line1\nline2\nline3"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := New(Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	logs, err := c.GetPromotionStepLogs(context.Background(), "myapp", "p1", "p1-pod-early")
	if err != nil {
		t.Fatalf("GetPromotionStepLogs: %v", err)
	}
	if logs != "line1\nline2\nline3" {
		t.Errorf("unexpected logs: %q", logs)
	}
}

func TestGetPromotionStepLogs_MissingArgs(t *testing.T) {
	c, _ := New(Config{BaseURL: "http://x", Username: "u", Password: "p"})
	if _, err := c.GetPromotionStepLogs(context.Background(), "", "p", "n"); err == nil {
		t.Error("expected error on empty pipeline")
	}
	if _, err := c.GetPromotionStepLogs(context.Background(), "p", "", "n"); err == nil {
		t.Error("expected error on empty promotion")
	}
	if _, err := c.GetPromotionStepLogs(context.Background(), "p", "pr", ""); err == nil {
		t.Error("expected error on empty node")
	}
}
