// Copyright 2026.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"
)

// listenerTimeout bounds how long ListenForJWT will wait for the user to
// complete the browser dance. Six minutes matches the OIDC state cookie's
// 10-minute server-side TTL with a safety margin — if the user is still
// faffing, they're past the practical window anyway.
const listenerTimeout = 6 * time.Minute

// ListenForJWT runs the loopback-redirect SSO flow:
//
//  1. Bind 127.0.0.1:0 — OS assigns an ephemeral port.
//  2. Generate a random `state` for CSRF protection.
//  3. Print the compass-api CLI-login URL to stderr (so it's visible
//     even when the browser-open fails or we're in a headless env).
//  4. Try to open the user's default browser to that URL.
//  5. Block until the loopback receives /cli-callback?token=...&state=...
//     OR the context times out.
//  6. Verify the returned state matches what we sent — defense-in-depth
//     on top of the ephemeral random port.
//  7. Return the JWT to the caller.
func ListenForJWT(ctx context.Context, baseURL string) (string, error) {
	state, err := randomState()
	if err != nil {
		return "", fmt.Errorf("auth: generate state: %w", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("auth: bind loopback: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	loginURL := fmt.Sprintf("%s/api/auth/cli/login?port=%d&state=%s", baseURL, port, state)

	// Buffer=1 lets the handler complete without blocking on a slow reader.
	// Errors flow back via a separate channel so the goroutine never blocks
	// on a closed/unread chan.
	tokenCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/cli-callback", func(w http.ResponseWriter, r *http.Request) {
		gotToken := r.URL.Query().Get("token")
		gotState := r.URL.Query().Get("state")
		if gotState != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		if gotToken == "" {
			http.Error(w, "missing token", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(successHTML))
		tokenCh <- gotToken
	})

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	// Print the URL to stderr regardless of whether we'll try to open a
	// browser. In a headless env the user can copy this URL elsewhere and
	// the listener still works as long as the redirect can reach this port.
	fmt.Fprintf(os.Stderr, "\ncompass-mcp: open this URL in any browser to sign in:\n\n  %s\n\nWaiting for callback on http://127.0.0.1:%d ...\n\n", loginURL, port)
	openBrowser(loginURL)

	timeoutCtx, cancel := context.WithTimeout(ctx, listenerTimeout)
	defer cancel()

	select {
	case tok := <-tokenCh:
		return tok, nil
	case err := <-errCh:
		return "", fmt.Errorf("auth: loopback server: %w", err)
	case <-timeoutCtx.Done():
		return "", fmt.Errorf("auth: timed out waiting for browser callback after %s", listenerTimeout)
	}
}

// openBrowser tries to launch the OS's default browser pointing at url.
// Errors are intentionally swallowed: the URL has already been printed to
// stderr, so a headless env or a missing `open`/`xdg-open` doesn't break
// the flow — the user can copy the URL by hand.
func openBrowser(target string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	_ = cmd.Start()
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// successHTML is what the user sees in their browser after the callback
// fires. Static text, no scripts, no external assets — keeps the loopback
// surface minimal.
const successHTML = `<!doctype html>
<html><head><title>compass-mcp signed in</title>
<style>body{font:14px/1.4 system-ui;max-width:480px;margin:80px auto;color:#222}</style>
</head><body>
<h2>Signed in to Compass</h2>
<p>You can close this tab and return to your terminal.</p>
</body></html>`
