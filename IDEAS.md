# Ideas backlog

Forward-looking enhancements that aren't committed work. Each idea has
its own directory under `ideas/<feature>/` so we can grow the design
context (sketches, decisions, links) without bloating any single file.

| Idea | Status | One-liner |
|---|---|---|
| [sso-auth](ideas/sso-auth/README.md) | Idea | OIDC loopback-redirect auth so MCP clients can sign in as the human user instead of a static admin account. |
| [write-tools](ideas/write-tools/README.md) | Idea | `promote` / `approve` / `invalidate` tools with explicit confirmation flow — currently V1 is read-only. |
| [streaming-log-tail](ideas/streaming-log-tail/README.md) | Idea | Live log tail for Running workflow steps via MCP progress notifications, so agents see step output as it streams. |
| [bundle-tools](ideas/bundle-tools/README.md) | Idea | `list_bundles` / `get_bundle` / `list_bundlereleases` tools so agents can answer "which envs run which version" without walking Promotions. |

For *prod-quality gaps in shipped tools* see [HARDENING.md](HARDENING.md);
for *implementation conventions and where-we-are* see [CLAUDE.md](CLAUDE.md).

## Adding a new idea

Create `ideas/<short-slug>/README.md` and link it in the table above.
Mirror the existing entries:

- **Status**: Idea / Designed / In progress / Implemented / Deferred.
- **Motivation**: what real problem this solves, in user terms.
- **Sketch**: the smallest concrete description to argue about.
- **Open questions**: what we'd have to decide when we revisit.
- **Out of scope** (when meaningful): reduces re-litigation later.

Each entry should be self-contained — future-you (or a future
contributor) should be able to pick up the directory cold and understand
the context without needing the chat history that produced it.
