// Copyright 2026.
package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func makeJWT(t *testing.T, exp int64) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadBytes, _ := json.Marshal(map[string]any{"exp": exp})
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	sig := base64.RawURLEncoding.EncodeToString([]byte("fake-sig"))
	return header + "." + payload + "." + sig
}

func TestTokenExpiry_ValidJWT(t *testing.T) {
	want := time.Now().Add(12 * time.Hour).Unix()
	jwt := makeJWT(t, want)
	got, err := tokenExpiry(jwt)
	if err != nil {
		t.Fatalf("tokenExpiry: %v", err)
	}
	if got.Unix() != want {
		t.Errorf("tokenExpiry().Unix() = %d, want %d", got.Unix(), want)
	}
}

func TestTokenExpiry_MalformedShapes(t *testing.T) {
	cases := map[string]string{
		"empty":              "",
		"one part":           "abc",
		"two parts":          "abc.def",
		"non-base64 payload": "abc.!!!.def",
		"non-JSON payload":   "abc." + base64.RawURLEncoding.EncodeToString([]byte("notjson")) + ".def",
	}
	for name, jwt := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := tokenExpiry(jwt); err == nil {
				t.Errorf("tokenExpiry(%q) returned nil error, want non-nil", jwt)
			}
		})
	}
}

func TestTokenExpiry_MissingExpClaim(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"alice"}`))
	jwt := header + "." + payload + ".sig"
	if _, err := tokenExpiry(jwt); err == nil {
		t.Errorf("expected error for JWT without exp claim")
	}
}

// TestAcquireJWT_CacheHit_SkipsBrowserFlow — when LoadToken returns a
// valid cached token, AcquireJWT must NOT call ListenForJWT. This is the
// "fast path on subsequent process starts" property the cache exists for.
func TestAcquireJWT_CacheHit_SkipsBrowserFlow(t *testing.T) {
	withTempConfigDir(t)
	expiry := time.Now().Add(time.Hour)
	if err := SaveToken("https://compass.example.com", "cached-tok", expiry); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Use a context that's immediately done — if AcquireJWT tries to run
	// the browser flow it'll error on the deadline; if it hits the cache
	// it returns instantly with the cached token.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := AcquireJWT(ctx, "https://compass.example.com")
	if err != nil {
		t.Fatalf("AcquireJWT on cache hit: %v", err)
	}
	if got != "cached-tok" {
		t.Errorf("AcquireJWT = %q, want cached-tok", got)
	}
}
