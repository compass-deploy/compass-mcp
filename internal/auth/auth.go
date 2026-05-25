// Copyright 2026.
package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// AcquireJWT is the single entry point client code calls at startup.
// Returns a JWT for baseURL: from the cache if available + unexpired,
// otherwise from a fresh browser flow (which prints a URL to stderr and
// blocks until the user signs in).
func AcquireJWT(ctx context.Context, baseURL string) (string, error) {
	if tok := LoadToken(baseURL); tok != "" {
		return tok, nil
	}
	tok, err := ListenForJWT(ctx, baseURL)
	if err != nil {
		return "", err
	}
	exp, err := tokenExpiry(tok)
	if err != nil {
		// Caller still gets a usable token; only the cache write is
		// skipped. They'll re-auth on the next process start.
		return tok, nil
	}
	_ = SaveToken(baseURL, tok, exp)
	return tok, nil
}

// tokenExpiry decodes the JWT's `exp` claim without verifying the
// signature. We trust compass-api to issue valid tokens — the MCP only
// needs `exp` to know when to drop the cache entry. The standard library
// has no built-in JWT parser; we hand-decode the base64url payload.
func tokenExpiry(jwt string) (time.Time, error) {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("auth: malformed JWT (parts=%d)", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("auth: decode JWT payload: %w", err)
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}, fmt.Errorf("auth: parse JWT claims: %w", err)
	}
	if claims.Exp == 0 {
		return time.Time{}, fmt.Errorf("auth: JWT has no exp claim")
	}
	return time.Unix(claims.Exp, 0), nil
}
