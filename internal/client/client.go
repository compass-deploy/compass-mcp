// Package client is a thin HTTP wrapper around the compass-api endpoints
// the MCP server fronts. It owns admin-account login, the cookie jar that
// carries the resulting session, and the one-shot re-auth on 401 that
// keeps long-running agent sessions from breaking when the session JWT
// expires mid-conversation.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	adminLoginPath = "/api/auth/admin-login"
	mePath         = "/api/me"
	defaultTimeout = 15 * time.Second
)

// Config is the runtime input for a Client. NewFromEnv fills it from the
// COMPASS_URL/USERNAME/PASSWORD env vars an MCP host (Claude Code,
// Cursor, etc.) passes into the subprocess.
type Config struct {
	BaseURL  string
	Username string
	Password string
}

// Client talks to a single compass-api. It is safe for concurrent use:
// AuthN state is protected by a mutex, the underlying http.Client is
// already concurrent-safe, and the cookiejar internally guards itself.
type Client struct {
	cfg  Config
	http *http.Client

	mu        sync.Mutex
	loggedIn  bool
}

// New builds a Client. The HTTP client carries a cookie jar so the
// session cookie compass-api sets on admin-login is automatically
// attached to subsequent requests against the same host.
func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("client: BaseURL is required")
	}
	if cfg.Username == "" || cfg.Password == "" {
		return nil, errors.New("client: Username and Password are required")
	}
	if _, err := url.Parse(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("client: parse BaseURL: %w", err)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("client: cookiejar: %w", err)
	}
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Jar:     jar,
			Timeout: defaultTimeout,
		},
	}, nil
}

// NewFromEnv reads COMPASS_URL, COMPASS_USERNAME, COMPASS_PASSWORD and
// returns a configured Client. Missing values surface as a descriptive
// error so MCP-host log output points the operator at the right env var.
func NewFromEnv() (*Client, error) {
	cfg := Config{
		BaseURL:  os.Getenv("COMPASS_URL"),
		Username: os.Getenv("COMPASS_USERNAME"),
		Password: os.Getenv("COMPASS_PASSWORD"),
	}
	var missing []string
	if cfg.BaseURL == "" {
		missing = append(missing, "COMPASS_URL")
	}
	if cfg.Username == "" {
		missing = append(missing, "COMPASS_USERNAME")
	}
	if cfg.Password == "" {
		missing = append(missing, "COMPASS_PASSWORD")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("client: missing env vars: %s", strings.Join(missing, ", "))
	}
	return New(cfg)
}

// Me calls GET /api/me. First call triggers an admin-login; subsequent
// calls reuse the cached session cookie. A 401 along the way causes a
// single re-auth + retry so an agent doesn't see a stale-session error
// for a transient cookie expiry.
func (c *Client) Me(ctx context.Context) (*Me, error) {
	var out Me
	if err := c.doJSON(ctx, http.MethodGet, mePath, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ensureLoggedIn performs an admin-login if we haven't yet. The mutex
// serializes concurrent first-call races to a single login.
func (c *Client) ensureLoggedIn(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loggedIn {
		return nil
	}
	if err := c.login(ctx); err != nil {
		return err
	}
	c.loggedIn = true
	return nil
}

// invalidateSession drops the cached "we're logged in" flag so the next
// call re-auths. Called when we see a 401 from a request that we thought
// was authenticated.
func (c *Client) invalidateSession() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loggedIn = false
}

func (c *Client) login(ctx context.Context) error {
	body, _ := json.Marshal(map[string]string{
		"username": c.cfg.Username,
		"password": c.cfg.Password,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+adminLoginPath, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("login: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("login: invalid credentials")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("login: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// doJSON performs an authenticated request, decoding the JSON response
// into out. On 401 it re-auths once and retries the same request — the
// retry uses a fresh request value so a body reader doesn't end up at
// EOF (M1 has no bodied requests but M2's POSTs will need this).
func (c *Client) doJSON(ctx context.Context, method, path string, in any, out any) error {
	if err := c.ensureLoggedIn(ctx); err != nil {
		return err
	}
	resp, err := c.send(ctx, method, path, in)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		c.invalidateSession()
		if err := c.ensureLoggedIn(ctx); err != nil {
			return err
		}
		resp, err = c.send(ctx, method, path, in)
		if err != nil {
			return err
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%s %s: status %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("%s %s: decode body: %w", method, path, err)
	}
	return nil
}

func (c *Client) send(ctx context.Context, method, path string, in any) (*http.Response, error) {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return nil, fmt.Errorf("%s %s: encode body: %w", method, path, err)
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("%s %s: build request: %w", method, path, err)
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	return resp, nil
}
