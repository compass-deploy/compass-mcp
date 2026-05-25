// Copyright 2026.
package auth

import "testing"

// The full loopback round-trip (start listener → simulate browser →
// receive JWT) is exercised by the binary smoke test at
// cmd/compass-mcp/smoke_test.go::TestStdioSmoke_SSO, which can capture
// the listener's stderr to discover the ephemeral port. At the unit
// level we cover the pieces that don't require knowing that port.

func TestRandomState_HexAndUnique(t *testing.T) {
	a, err := randomState()
	if err != nil {
		t.Fatalf("randomState: %v", err)
	}
	b, err := randomState()
	if err != nil {
		t.Fatalf("randomState: %v", err)
	}
	if len(a) != 32 {
		t.Errorf("len(randomState()) = %d, want 32 hex chars", len(a))
	}
	if a == b {
		t.Errorf("two successive randomState() calls returned identical values")
	}
}

// TestOpenBrowser_NoCrash asserts openBrowser doesn't panic on any
// platform. It intentionally ignores errors from exec.Command, so the
// only failure mode this protects against is a panic from a future
// regression (e.g. nil command construction).
func TestOpenBrowser_NoCrash(t *testing.T) {
	openBrowser("about:blank")
}
