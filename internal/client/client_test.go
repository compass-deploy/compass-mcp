package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeAPI lets each test express the protocol it expects compass-api to
// follow with closures, without standing up the real router. We assert
// header/body invariants in handlers; status codes come from the script.
type fakeAPI struct {
	loginCalls atomic.Int32
	meCalls    atomic.Int32
	onLogin    http.HandlerFunc
	onMe       http.HandlerFunc
}

func (f *fakeAPI) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/admin-login", func(w http.ResponseWriter, r *http.Request) {
		f.loginCalls.Add(1)
		if f.onLogin != nil {
			f.onLogin(w, r)
		}
	})
	mux.HandleFunc("GET /api/me", func(w http.ResponseWriter, r *http.Request) {
		f.meCalls.Add(1)
		if f.onMe != nil {
			f.onMe(w, r)
		}
	})
	return mux
}

// issueCookie sets the same session cookie compass-api would set on
// successful admin-login. Helper so individual login handlers stay short.
func issueCookie(w http.ResponseWriter, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "compass_session",
		Value:    value,
		Path:     "/",
		HttpOnly: true,
	})
	w.WriteHeader(http.StatusNoContent)
}

func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c, err := New(Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestMe_Success(t *testing.T) {
	api := &fakeAPI{
		onLogin: func(w http.ResponseWriter, _ *http.Request) { issueCookie(w, "v1") },
		onMe: func(w http.ResponseWriter, r *http.Request) {
			if _, err := r.Cookie("compass_session"); err != nil {
				t.Errorf("expected session cookie on /api/me, got %v", err)
			}
			_ = json.NewEncoder(w).Encode(Me{
				AuthEnabled: true, Authenticated: true, User: "admin",
				Groups: []string{"platform-admins"},
			})
		},
	}
	srv := httptest.NewServer(api.handler())
	defer srv.Close()
	c := newTestClient(t, srv)

	me, err := c.Me(context.Background())
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if me.User != "admin" || !me.Authenticated {
		t.Errorf("unexpected Me: %+v", me)
	}
	if api.loginCalls.Load() != 1 {
		t.Errorf("expected 1 login, got %d", api.loginCalls.Load())
	}

	// Second call must reuse the session (no extra login).
	if _, err := c.Me(context.Background()); err != nil {
		t.Fatalf("Me #2: %v", err)
	}
	if api.loginCalls.Load() != 1 {
		t.Errorf("expected login cached, got %d logins", api.loginCalls.Load())
	}
}

func TestMe_ReauthOn401(t *testing.T) {
	var meAttempts atomic.Int32
	api := &fakeAPI{
		onLogin: func(w http.ResponseWriter, _ *http.Request) { issueCookie(w, "v1") },
		onMe: func(w http.ResponseWriter, _ *http.Request) {
			if meAttempts.Add(1) == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(Me{Authenticated: true, User: "admin"})
		},
	}
	srv := httptest.NewServer(api.handler())
	defer srv.Close()
	c := newTestClient(t, srv)

	me, err := c.Me(context.Background())
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if me.User != "admin" {
		t.Errorf("expected admin, got %q", me.User)
	}
	if api.loginCalls.Load() != 2 {
		t.Errorf("expected 2 logins (initial + re-auth), got %d", api.loginCalls.Load())
	}
	if meAttempts.Load() != 2 {
		t.Errorf("expected 2 /api/me calls, got %d", meAttempts.Load())
	}
}

func TestMe_ReauthThenStill401(t *testing.T) {
	api := &fakeAPI{
		onLogin: func(w http.ResponseWriter, _ *http.Request) { issueCookie(w, "v1") },
		onMe:    func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) },
	}
	srv := httptest.NewServer(api.handler())
	defer srv.Close()
	c := newTestClient(t, srv)

	_, err := c.Me(context.Background())
	if err == nil {
		t.Fatal("expected error after persistent 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention status 401, got %v", err)
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	api := &fakeAPI{
		onLogin: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) },
	}
	srv := httptest.NewServer(api.handler())
	defer srv.Close()
	c := newTestClient(t, srv)

	_, err := c.Me(context.Background())
	if err == nil || !strings.Contains(err.Error(), "invalid credentials") {
		t.Fatalf("expected invalid-credentials error, got %v", err)
	}
}

func TestMe_404(t *testing.T) {
	api := &fakeAPI{
		onLogin: func(w http.ResponseWriter, _ *http.Request) { issueCookie(w, "v1") },
		onMe:    func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "nope", http.StatusNotFound) },
	}
	srv := httptest.NewServer(api.handler())
	defer srv.Close()
	c := newTestClient(t, srv)

	_, err := c.Me(context.Background())
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected 404 error, got %v", err)
	}
}

func TestMe_5xx(t *testing.T) {
	api := &fakeAPI{
		onLogin: func(w http.ResponseWriter, _ *http.Request) { issueCookie(w, "v1") },
		onMe:    func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "boom", http.StatusInternalServerError) },
	}
	srv := httptest.NewServer(api.handler())
	defer srv.Close()
	c := newTestClient(t, srv)

	_, err := c.Me(context.Background())
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected 500 error, got %v", err)
	}
}

func TestMe_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.NewServeMux())
	srv.Close()
	c := newTestClient(t, srv)
	_, err := c.Me(context.Background())
	if err == nil {
		t.Fatal("expected network error")
	}
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		t.Errorf("expected *url.Error, got %T: %v", err, err)
	}
}

func TestMe_BadJSON(t *testing.T) {
	api := &fakeAPI{
		onLogin: func(w http.ResponseWriter, _ *http.Request) { issueCookie(w, "v1") },
		onMe: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "not-json")
		},
	}
	srv := httptest.NewServer(api.handler())
	defer srv.Close()
	c := newTestClient(t, srv)

	_, err := c.Me(context.Background())
	if err == nil || !strings.Contains(err.Error(), "decode body") {
		t.Fatalf("expected decode error, got %v", err)
	}
}

func TestNewFromEnv_MissingURL(t *testing.T) {
	t.Setenv("COMPASS_URL", "")
	t.Setenv("COMPASS_USERNAME", "")
	t.Setenv("COMPASS_PASSWORD", "")
	_, err := NewFromEnv()
	if err == nil || !strings.Contains(err.Error(), "COMPASS_URL") {
		t.Fatalf("expected COMPASS_URL error, got %v", err)
	}
}

func TestNewWithJWT_SeedsCookieAndSkipsAdminLogin(t *testing.T) {
	api := &fakeAPI{
		onLogin: func(w http.ResponseWriter, _ *http.Request) {
			t.Errorf("admin-login should not be called in SSO mode")
			w.WriteHeader(http.StatusInternalServerError)
		},
		onMe: func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie("compass_session")
			if err != nil {
				t.Errorf("expected compass_session cookie, got %v", err)
				return
			}
			if c.Value != "seed-jwt" {
				t.Errorf("cookie value = %q, want seed-jwt", c.Value)
			}
			_ = json.NewEncoder(w).Encode(Me{AuthEnabled: true, Authenticated: true, User: "alice"})
		},
	}
	srv := httptest.NewServer(api.handler())
	defer srv.Close()

	c, err := NewWithJWT(Config{BaseURL: srv.URL}, "seed-jwt")
	if err != nil {
		t.Fatalf("NewWithJWT: %v", err)
	}
	me, err := c.Me(context.Background())
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if me.User != "alice" {
		t.Errorf("Me().User = %q, want alice", me.User)
	}
	if api.loginCalls.Load() != 0 {
		t.Errorf("admin-login called %d times in SSO mode, want 0", api.loginCalls.Load())
	}
}

func TestSSOMode_401ReturnsRestartError(t *testing.T) {
	// In SSO mode a 401 must NOT trigger an admin-login retry — the
	// caller has no creds. It must surface a clear restart-required error.
	api := &fakeAPI{
		onLogin: func(w http.ResponseWriter, _ *http.Request) {
			t.Errorf("admin-login should not be called on 401 in SSO mode")
			w.WriteHeader(http.StatusInternalServerError)
		},
		onMe: func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		},
	}
	srv := httptest.NewServer(api.handler())
	defer srv.Close()

	c, err := NewWithJWT(Config{BaseURL: srv.URL}, "seed-jwt")
	if err != nil {
		t.Fatalf("NewWithJWT: %v", err)
	}
	_, err = c.Me(context.Background())
	if err == nil || !strings.Contains(err.Error(), "SSO session expired") {
		t.Fatalf("expected SSO-expired error, got %v", err)
	}
	if api.loginCalls.Load() != 0 {
		t.Errorf("admin-login called %d times in SSO mode, want 0", api.loginCalls.Load())
	}
}

func TestNewFromEnv_UsernameWithoutPassword(t *testing.T) {
	t.Setenv("COMPASS_URL", "https://compass.example.com")
	t.Setenv("COMPASS_USERNAME", "admin")
	t.Setenv("COMPASS_PASSWORD", "")
	_, err := NewFromEnv()
	if err == nil || !strings.Contains(err.Error(), "COMPASS_PASSWORD") {
		t.Fatalf("expected COMPASS_PASSWORD error, got %v", err)
	}
}

func TestNew_RequiredFields(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
	}{
		{"no url", Config{Username: "u", Password: "p"}},
		{"no user", Config{BaseURL: "http://x", Password: "p"}},
		{"no pass", Config{BaseURL: "http://x", Username: "u"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := New(tc.cfg); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
