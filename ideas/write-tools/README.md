# Write tools (promote / approve / invalidate)

## Status
**Idea — design partially open.**

Tracked previously as a one-liner in CLAUDE.md's "Deferred from V1"
list. V1 is read-only by design: lets agents *answer questions*
without risk of unintended mutations. This doc covers turning the
read-only restriction off safely.

## Motivation

Today an agent can diagnose a failure ("the last promotion to staging
failed because the workflow timed out at the render step") but can't
take action ("retry the promotion"). The user has to drop out of the
agent context, switch to the UI / kubectl, and do the action by hand —
losing the agent's diagnostic context in the process.

Write tools close that loop, but they need a confirmation flow that
matches the read-only-by-default safety the V1 spec opted into.

## Tools to add

| Tool | Maps to | Confirmation requirement |
|---|---|---|
| `promote` | `POST /api/pipelines/{p}/promotions` | High — creates a deploy |
| `approve` | `POST /api/pipelines/{p}/approvals` (decision=Approved) | High — releases a downstream gate |
| `reject` | `POST /api/pipelines/{p}/approvals` (decision=Rejected) | High — closes a gate |
| `invalidate` | `POST /api/pipelines/{p}/invalidations` | High — blocks future promotions of named releases |
| `retry_promotion` | `POST /api/pipelines/{p}/promotions` (same env + release as a failed one) | Medium — re-runs an existing intent |

## Sketch: per-tool confirmation via MCP elicitation

The MCP protocol has an `elicitation/create` primitive
([spec](https://modelcontextprotocol.io/specification/server/utilities/elicitation))
that lets a server ask the agent's host (Claude Code / Cursor / etc.)
to prompt the user mid-tool-call. The flow for `promote`:

1. Agent calls `promote(pipeline="sampleapp", environment="prod", release="sampleapp-26.05.26.1")`.
2. MCP server validates args, looks up the env's gates (does it require
   approval? what's the current upstream-verified state?), and renders
   a summary.
3. MCP server sends `elicitation/create` with the summary + an
   `accept`/`cancel` choice.
4. Host displays it to the user inline ("Confirm: promote sampleapp-26.05.26.1
   to prod (requires approval after)?"). User clicks accept.
5. MCP server POSTs to `compass-api` and returns the new Promotion's
   name + initial status.

If the host doesn't support elicitation (older agents): fall back to a
"two-call" pattern — the first call returns `{requiresConfirmation: true,
confirmationToken: "..."}` and the agent has to call `promote_confirm(token)`
in a separate turn. The user sees the agent's "I'm about to do X — confirm?"
message and answers explicitly. Less ergonomic but works everywhere.

## Other design choices

- **Tool descriptions explicitly note the write side-effect** so agent
  models can pre-warn the user before even calling. The schema is the
  primary contract; descriptions are the secondary.
- **Audit attribution comes from auth.** Once SSO ships ([sso-auth](../sso-auth/README.md)),
  the Promotion CR gets stamped with the actual user's identity.
  Without SSO, write tools probably shouldn't ship — staying read-only
  is better than every action being attributed to `admin`.
- **No batching.** Each tool acts on one CR. "Promote 26.05.26.1 to all
  three prod regions" is three calls, not one — keeps the audit
  trail granular and prevents partial-failure ambiguity.
- **Idempotency.** Compass uses `metadata.generateName` for Promotions
  so each call creates a fresh CR even with identical inputs. That's
  the right default for "retry promotion."

## Open questions

- **Does MCP elicitation work in Claude Code yet?** Verify before
  committing to it as the primary path. (As of the protocol spec it
  exists; client support varies.)
- **Should `invalidate` always require confirmation, or only for
  release counts > N?** Invalidating one release is a small click;
  invalidating 20 releases at once is a potentially-destructive bulk
  op. Probably always confirm — the agent can be told to batch in
  one CR.
- **Polling after a write call.** Once `promote` returns the
  Promotion's name, should the tool block + poll until terminal,
  or return immediately and let the agent re-query? Lean toward
  returning immediately — keeps tools fast, lets the agent decide
  whether to wait.
- **Approve + Reject as one tool or two?** Spec says single CR with
  a `decision` field. The MCP tool surface could mirror that
  (`approve(decision=...)`) or split for clarity. Two tools reads
  better to agents and is explicit in their planning text.

## Out of scope

- A "rollback" verb. Compass models rollback as "promote the
  previous version" — same Promotion CR shape. No new mutation needed.
- Bulk operations across multiple Pipelines. Pipeline is the tenant
  boundary; cross-Pipeline bulk should require explicit per-Pipeline
  calls.
- Editing existing CRs (e.g. updating an Invalidation's `reason`).
  Compass CRs are mostly spec-immutable; if you got the spec wrong,
  delete + recreate.

## Depends on

- [sso-auth](../sso-auth/README.md) — without per-user identity, write
  tools degrade the audit story. Probably should not ship before SSO.
