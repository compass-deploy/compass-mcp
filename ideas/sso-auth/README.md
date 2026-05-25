# SSO auth for MCP (OIDC loopback-redirect)

## Status
**Idea — design partially open.**

Tracked previously as a one-liner in CLAUDE.md's "Deferred from V1"
list. Promoted to a real design doc because it's the largest single
blocker for production use.

## Motivation

V1 MCP authenticates to `compass-api` with a static admin account
(`COMPASS_USERNAME` / `COMPASS_PASSWORD`). That's fine for local dev
but two real problems in production:

1. **Wrong identity.** Every action an agent takes is stamped
   `requestedBy: admin` instead of the human driving the agent. The
   audit story collapses ("alice asked Claude to promote 1.0.2" reads
   as "admin promoted 1.0.2" in the Activity tab).
2. **Wrong authorization.** The admin account has whatever ClusterRole
   it's bound to — typically `compass-admin`. The agent inherits that
   regardless of what the actual user is allowed to do. RBAC at
   compass-api becomes useless for MCP-driven actions.

Production deployments of `compass-api` will typically have the admin
account disabled (`auth.adminAccount.enabled=false`) and only OIDC.
V1 MCP doesn't work against those at all.

## Sketch: loopback-redirect (the `gh auth login` / `gcloud` pattern)

```
┌─ MCP subprocess ────────┐         ┌─ User's browser ─┐
│                         │         │                  │
│ 1. start http://127.0.0.1:PORT/cli-callback          │
│ 2. open URL in browser ───────────►                  │
│                                   │                  │
│                                   │ 3. OIDC code flow│
│                                   │    completes on  │
│                                   │    compass-api   │
│                                   ◄──────────────────┤
│ 4. receive JWT at loopback callback                  │
│ 5. cache to ~/.config/compass-mcp/session.json       │
│ 6. send compass_session cookie on subsequent calls   │
└─────────────────────────┘         └──────────────────┘
```

Steps the MCP needs:

1. On first call, check the cache (`~/.config/compass-mcp/session.json`,
   mode 0600) for a non-expired JWT for this `COMPASS_URL`. If valid,
   skip to step 6.
2. Bind an ephemeral HTTP listener on a random port `127.0.0.1:PORT`.
3. Open the user's default browser to a new compass-api endpoint
   `GET /api/auth/cli/login?port=PORT&state=<random>`.
4. compass-api stores `{state → loopback URL}`, runs its normal OIDC
   code flow, and in the callback handler — if the state matches a
   CLI session — 302s the browser to `http://127.0.0.1:PORT/cli-callback?token=<jwt>`
   instead of setting a cookie + redirecting to `/`.
5. MCP's listener captures the JWT, stores it in the cache.
6. MCP sends the JWT as the `compass_session` cookie on every call
   (same wire format compass-api already expects).

## What needs to change in compass-api

This is the bigger backend ask:

- New endpoint `GET /api/auth/cli/login` that initiates the OIDC flow
  with a CLI-aware state payload (loopback URL + random nonce).
- `/api/auth/callback` learns to detect a CLI state and 302 to the
  loopback URL with the JWT in a query param (only allowed for
  `127.0.0.1` / `[::1]` URLs, port arbitrary — never redirect tokens
  to remote hosts).
- The CLI-issued JWT could carry a shorter TTL than the UI's 12h
  default, configurable via flag.

See the parallel discussion in compass-deploy `ideas/` (no doc yet —
would be `ideas/cli-auth/` over there).

## Alternative considered: OAuth 2.0 Device Authorization Flow (RFC 8628)

MCP prints `verification_uri` + `user_code`, user goes to browser,
authenticates, MCP polls the token endpoint. Pros: works without a
local browser (e.g. remote SSH session). Cons: bigger backend state
machine (device_code → pending/granted → JWT); needs a `/device`
page rendered by compass-api. **Loopback wins on smaller surface +
faster cold-start** for the desktop-agent use case that's our actual
target.

## Open questions

- **Where does the JWT cache live, exactly?** `~/.config/compass-mcp/session.json`
  follows XDG; macOS has Keychain as a better-protected alternative
  but adds platform-specific code. Lean toward XDG + 0600 for v1.
- **One cache file per `COMPASS_URL`?** Yes — agents may have multiple
  compass instances configured.
- **What happens when the cached JWT expires mid-session?** Same
  re-auth flow; agent sees one tool call return a "session expired,
  re-authenticate" error and the next call works after the user
  clicks the link MCP prints.
- **Revocation.** Compass JWTs are stateless (HMAC-signed); deleting
  the cache invalidates the local session but the JWT remains valid
  on the server until expiry. Probably acceptable for short TTLs.
- **CI / headless environments.** Loopback assumes a browser is
  available on the same machine. For CI, a long-lived "MCP token"
  pattern (HARDENING-tracked separately) would be the answer.

## Out of scope

- Refresh tokens. Compass-api itself doesn't ship refresh; v1 MCP
  matches that. Re-login is one click.
- Browser-cookie scraping (extracting `compass_session` from the
  user's already-logged-in browser). Brittle, platform-specific,
  doesn't survive cookie rotation.
- Mid-tool-call interactive prompts. The auth flow always happens
  before/between tool calls, never in the middle of one.
