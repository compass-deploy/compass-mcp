# Streaming log tail for Running steps

## Status
**Idea — design open.**

Tracked previously as a one-liner in CLAUDE.md's "Deferred from V1"
list. V1's `get_promotion_step_logs` returns the last 500 lines as a
single snapshot — fine for completed steps, awkward for ones that are
still running.

## Motivation

When an agent is investigating an in-flight promotion ("why is the
deploy taking so long?") it currently has to call
`get_promotion_step_logs` repeatedly to see new lines, parse them,
decide whether to call again. That's polling at the agent layer,
which:

- Burns tokens re-reading lines the agent has already seen.
- Has no built-in "this step is done; stop polling" signal.
- Wastes time when the step is moving fast and the agent is sampling
  at a coarser cadence.

The UI already does live tailing (3s react-query refetch for Running
steps in `PromotionInspector`). MCP should expose the same.

## Sketch: MCP progress notifications

MCP has a [progress notification primitive](https://modelcontextprotocol.io/specification/server/utilities/progress)
that a server can emit during a long-running tool call. Agent hosts
that support it render the progress inline.

```
agent.callTool("tail_promotion_step_logs", { ..., follow: true })
  ↓
MCP server:
  while !ctx.Done() && stepPhaseRunning:
    delta = fetch(/logs?since=lastLine)
    if delta:
      emit progress notification with `{lines: delta}`
      lastLine += len(delta)
    sleep 3s
  return final result (entire log + terminal phase)
```

Design choices:

- **New tool** `tail_promotion_step_logs(pipeline, promotion, nodeId)`
  rather than extending the existing `get_promotion_step_logs` with a
  `follow=true` flag. Different tool, different return shape — the
  follow tool returns *deltas via notifications* and a final summary;
  the snapshot tool returns one body. Clearer in the agent's schema.
- **Always bounded.** The follow loop terminates when (a) the step
  reaches a terminal phase or (b) the agent host cancels the call or
  (c) a max-duration cap (default 10min) fires. Never streams forever.
- **Snapshot tool stays.** Many use cases (post-mortem, completed
  steps) don't need streaming; the existing snapshot is faster +
  simpler.

## Backend changes

None required. compass-api's existing `GET /api/pipelines/{p}/promotions/{pr}/steps/{node}/logs`
endpoint already returns the current log buffer; the MCP server polls
it and computes deltas client-side. If poll latency becomes a problem,
a server-sent-events variant of the endpoint could land later — purely
optimization, not blocking.

## Open questions

- **Delta computation: by line count, byte offset, or last-line hash?**
  Argo Workflows / k8s `pods/log` rewrites the tail on each fetch but
  the buffer grows monotonically, so simple `lines[lastSeen:]`
  slicing is correct. If logs get truncated mid-stream (Argo's
  rare retention path), the MCP should detect length-decrease and
  restart from the top with a warning.
- **Multiple followers.** If two agents (or the user via UI) are
  both watching the same step, do they each get their own poll cycle?
  Probably yes — independent state, doesn't matter at our scale.
- **Polling cadence.** 3s matches the UI. Configurable per-call?
  Probably not — adding knobs invites variance. One reasonable
  default; revisit if a real user feels it.
- **What if the agent host doesn't support progress notifications?**
  Fall back to returning periodic snapshots in the result body, or
  just delegate to the existing snapshot tool with guidance text.

## Out of scope

- Multi-step parallel tails. If an agent wants to watch 4 steps at
  once, that's 4 tool calls — keeps semantics simple.
- Log search / grep inside the tail. The agent can grep the snapshot.
- Persistent log archive. Compass deliberately doesn't store logs
  outside k8s; the workflow step's pod logs are the source of truth.
