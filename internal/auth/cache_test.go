// Copyright 2026.
package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func withTempConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(configDirEnv, dir)
	return dir
}

func TestLoadToken_MissingFile(t *testing.T) {
	withTempConfigDir(t)
	if got := LoadToken("https://compass.example.com"); got != "" {
		t.Fatalf("LoadToken on missing file = %q, want \"\"", got)
	}
}

func TestSaveAndLoadToken_RoundTrip(t *testing.T) {
	withTempConfigDir(t)
	if err := SaveToken("https://compass.example.com", "tok-xyz", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}
	if got := LoadToken("https://compass.example.com"); got != "tok-xyz" {
		t.Fatalf("LoadToken after save = %q, want tok-xyz", got)
	}
}

func TestLoadToken_ExpiredEntryTreatedAsMiss(t *testing.T) {
	withTempConfigDir(t)
	if err := SaveToken("https://compass.example.com", "tok-old", time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}
	if got := LoadToken("https://compass.example.com"); got != "" {
		t.Fatalf("LoadToken on expired = %q, want \"\"", got)
	}
}

func TestLoadToken_DifferentURLReturnsEmpty(t *testing.T) {
	withTempConfigDir(t)
	if err := SaveToken("https://a.example.com", "tok-a", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}
	if got := LoadToken("https://b.example.com"); got != "" {
		t.Fatalf("LoadToken for unrelated URL = %q, want \"\"", got)
	}
}

func TestSaveToken_PreservesOtherEntries(t *testing.T) {
	withTempConfigDir(t)
	expiry := time.Now().Add(time.Hour)
	if err := SaveToken("https://a.example.com", "tok-a", expiry); err != nil {
		t.Fatalf("save a: %v", err)
	}
	if err := SaveToken("https://b.example.com", "tok-b", expiry); err != nil {
		t.Fatalf("save b: %v", err)
	}
	if got := LoadToken("https://a.example.com"); got != "tok-a" {
		t.Fatalf("entry a lost: %q", got)
	}
	if got := LoadToken("https://b.example.com"); got != "tok-b" {
		t.Fatalf("entry b lost: %q", got)
	}
}

func TestLoadToken_CorruptFileTreatedAsMiss(t *testing.T) {
	dir := withTempConfigDir(t)
	path := filepath.Join(dir, "compass-mcp", "session.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := LoadToken("https://compass.example.com"); got != "" {
		t.Fatalf("LoadToken on corrupt = %q, want \"\"", got)
	}
}

func TestSaveToken_FileIsMode0600(t *testing.T) {
	dir := withTempConfigDir(t)
	if err := SaveToken("https://compass.example.com", "x", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("save: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "compass-mcp", "session.json"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %v, want 0600", info.Mode().Perm())
	}
}
