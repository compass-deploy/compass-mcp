// Package auth handles the loopback-redirect SSO flow for compass-mcp.
//
// When COMPASS_USERNAME is unset, the client takes the SSO path: read the
// JWT cache for the current COMPASS_URL; on miss, start a local listener
// on 127.0.0.1:<random-port>, open the user's browser to the matching
// /api/auth/cli/login endpoint on compass-api, capture the JWT delivered
// to the loopback, cache it, and seed it into the http client's
// cookiejar. On expiry, the next process start re-runs the browser flow —
// we deliberately do NOT re-open the browser mid-session.
//
// This file owns the on-disk cache. One JSON file under the OS's user
// config directory, keyed by compass URL so a single MCP install can
// target multiple compass instances without stomping each other.
package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// configDirEnv overrides the OS user-config-dir lookup for tests. Set this
// to a tempdir in tests so the developer's real session cache isn't
// touched. Honoured by configDir() below.
const configDirEnv = "COMPASS_MCP_CONFIG_DIR"

// CachedToken is one entry in the on-disk cache.
type CachedToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// cacheFile is `{COMPASS_MCP_CONFIG_DIR or os.UserConfigDir()}/compass-mcp/session.json`.
type cacheFile map[string]CachedToken

// LoadToken returns the cached non-expired JWT for baseURL, or "" if no
// such entry exists. Any read error (missing file, malformed JSON,
// permission denied) is treated as a cache miss — the caller will run the
// browser flow to refill it.
func LoadToken(baseURL string) string {
	path, err := cachePath()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var c cacheFile
	if err := json.Unmarshal(data, &c); err != nil {
		return ""
	}
	entry, ok := c[baseURL]
	if !ok || entry.Token == "" {
		return ""
	}
	if !time.Now().Before(entry.ExpiresAt) {
		return ""
	}
	return entry.Token
}

// SaveToken writes the JWT for baseURL into the on-disk cache. Creates the
// config directory (mode 0700) and writes the file (mode 0600). Existing
// entries for other baseURLs are preserved.
//
// The write is atomic via temp-file + rename so two MCP processes starting
// simultaneously against the same COMPASS_URL can't tear each other's
// entries — the loser wins (last writer takes the file) but the file is
// always self-consistent.
func SaveToken(baseURL, token string, expiresAt time.Time) error {
	path, err := cachePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	existing := cacheFile{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &existing)
	}
	existing[baseURL] = CachedToken{Token: token, ExpiresAt: expiresAt}
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "session.*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}

func cachePath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "compass-mcp", "session.json"), nil
}

func configDir() (string, error) {
	if v := os.Getenv(configDirEnv); v != "" {
		return v, nil
	}
	return os.UserConfigDir()
}
