# Hardening backlog

Prod-quality gaps in **already-shipped** MCP code. Tracks polish,
correctness, audit, and security weaknesses that don't block usage
but bite real users at scale or in adversarial conditions. For
*features that haven't been implemented yet* (SSO auth, write tools,
streaming logs, Bundle tools) see [IDEAS.md](IDEAS.md).

## Test pattern — assert MCP-client-visible shape, not Go-struct shape

The `list_pipelines` / `list_promotions` / `list_promotion_steps`
shape bug fixed in `f605130` shipped past green tests because the
tests asserted `res.StructuredContent.([]client.PipelineSummary)` —
i.e. the in-process Go slice. The real MCP client (Claude Code's
validator) rejected the bare array per spec, but no test simulated
that. **The same shape of bug can recur on any other tool** until the
test harness exercises the MCP-client-visible JSON envelope.

- **Add a test layer that goes through the MCP protocol via stdio**
  and asserts the raw JSON shape of `structuredContent` — same path a
  real client takes. The existing `TestRealCompass_WhoamiRoundTrip`
  e2e demonstrates the stdio harness exists but only covers `whoami`.
  Expand to one per tool that exercises a typical-success response.
- **Per-tool integration tests should assert the marshalled JSON**
  (`json.Marshal(res.StructuredContent)` matches the expected envelope
  shape: `{"pipelines": [...]}` not `[...]`), in addition to the typed
  assertion. Cheap belt-and-suspenders against the regression class.

## Error messages bubbled up from compass-api are verbatim Go strings

When `compass-api` returns 401 / 404 / 500, the client wraps the raw
HTTP response into an error like:

```
list pipelines: GET /api/pipelines: status 401: unauthorized
```

An agent reading that has limited context. Two improvements:

- **Map common upstream statuses to actionable messages.** 401 →
  "Compass session expired or invalid; re-run `whoami` to force
  re-authentication, or check COMPASS_USERNAME/PASSWORD." 404 on a
  specific pipeline → "Pipeline `X` not found or you lack `get`
  permission on it." 5xx → "Compass server error; manager log may
  have details" (with no false claim that retry will fix it).
- **Tool error responses should carry a structured `code` field**
  (`upstream_auth_failed`, `pipeline_not_found`, `upstream_5xx`,
  `network_error`) so agents can route on the type without parsing
  the message string.

## Re-auth on 401 is one-shot

`client.doJSON` re-auths once when a request returns 401 and retries
(`client/client.go:173-183`). If the second attempt also returns 401 —
expected on a broken admin account, on a key rotation that invalidates
the old JWT before login can hand out a new one, or on a transient
auth service hiccup — the client surfaces the second 401 verbatim and
the next call starts the same dance over.

- **Track auth failures with a small backoff.** Two consecutive 401s
  within N seconds → return a tool error with code
  `upstream_auth_failed` and *don't* re-auth on the immediately-next
  call. Cap retries per minute.
- **Detect "stale-session loop"**: if re-auth succeeds but the
  subsequent request still 401s, the cookie wasn't actually issued
  (compass-api bug, proxy stripping cookies, etc.). Log this
  explicitly to stderr so operators can diagnose without reading
  Compass server logs.

## `COMPASS_PASSWORD` env var is plaintext in the MCP subprocess env

Per the README's recommended `claude mcp add` invocation, the password
is passed as an env var. It's visible to:

- Any process on the host that can `ps -E -ef` (most non-root processes
  on a multi-user host).
- The agent's own crash logs / core dumps.
- The MCP host process if it ever dumps subprocess env for debugging.

Not the MCP's bug per se — env-var creds are the agreed-on contract
— but worth a docs callout + a future credential-file fallback:

- **README warns** that `COMPASS_PASSWORD` is process-visible and
  recommends host-level mitigation (don't run on multi-user hosts,
  prefer SSO once it ships).
- **Add an optional `COMPASS_PASSWORD_FILE` env var** pointing at a
  mode-0600 file the MCP reads at startup. Avoids the env-var leak;
  matches common patterns (docker secrets, kubernetes file mounts,
  vault-injected files).
- **`~/.netrc` support** is the unix-traditional escape hatch but
  pulls in `golang.org/x/net/netrc`-style parsing; probably skip
  unless asked.
