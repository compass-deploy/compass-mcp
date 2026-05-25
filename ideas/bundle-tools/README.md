# Bundle-aware tools (`list_bundles`, `get_bundle`, `list_bundlereleases`)

## Status
**Idea — design open.**

Surfaced while working on the generic-Bundle redesign in compass-deploy
(commits `e418be7` … `7632fce`). V1 MCP has no Bundle-aware tooling at
all — agents can list pipelines and walk promotions, but can't answer
basic Bundle questions like "what releases are available?" or "is
1.0.2 a valid release on this Pipeline?"

## Motivation

Real questions an agent should be able to answer that V1 can't:

- *"Which versions of `sampleapp` are available to deploy?"*
  Requires `list_bundlereleases(pipeline=sampleapp)`.
- *"Why isn't 1.0.2 showing up as a release?"* Requires
  `get_bundle(pipeline, name)` to surface
  `status.conditions[ReleaseDiscovery]` — the per-artifact tag
  breakdown we ship in `compass-deploy` already names exactly which
  artifact is behind.
- *"What artifacts is this Bundle watching?"* Requires `get_bundle` to
  return the `spec.artifacts` map.
- *"Is the registry credential set up correctly?"* When the
  `ArtifactsReachable` hardening item lands in compass-deploy, the
  Bundle will surface a status condition; MCP needs to expose it.

Without these, an agent investigating "the latest release didn't show
up" has to ask the user to run `kubectl describe bundle`. That's a
broken loop.

## Tools to add

| Tool | Returns | Notes |
|---|---|---|
| `list_bundles(pipeline)` | `[{name, artifactCount, conditions: [...]}]` | Compact summary; reflects `status.conditions` per Bundle for at-a-glance health |
| `get_bundle(pipeline, name)` | Full `Bundle` CR — spec.artifacts map + status.conditions | The detail call for "why isn't this working" |
| `list_bundlereleases(pipeline, opts?)` | `[{name, bundle, version, phase, createdAt}]` | Optional filter by `bundle`, `phase` (Valid / Invalidated), `version` substring |

API shape mirrors the existing per-CR-kind list endpoints in compass-api
(`/api/pipelines/{p}/bundles`, `/api/pipelines/{p}/bundlereleases`),
which already return full CRs.

## Schema sketch (Go)

```go
// types.go additions
type BundleSummary struct {
    Name          string                       `json:"name"`
    ArtifactNames []string                     `json:"artifacts"` // just the map keys
    Conditions    []metav1.Condition           `json:"conditions"`
}

type BundleReleaseSummary struct {
    Name      string `json:"name"`
    Bundle    string `json:"bundle"`
    Version   string `json:"version"`
    Phase     string `json:"phase,omitempty"` // Valid | Invalidated | "" (unset)
    CreatedAt string `json:"createdAt"`       // RFC3339
}
```

Tools return typed envelopes (per the f605130 fix —
`structuredContent` must be a JSON object at the root, not an array).

## Filter design for `list_bundlereleases`

Optional args, mirroring the existing `list_promotions` precedent:

- `bundle` — only releases of this Bundle (in Pipelines with multi-source
  Bundles like the sample-app tested/untested pattern, this is the
  primary filter).
- `phase` — `Valid` filters out Invalidated; useful for "what can I
  promote." `Invalidated` filters for incident-response queries.
- `version` — substring match on the version field. Powerful but
  vague; useful for "is there a 1.0.x release."

Server-side filtering matches the `list_promotions` env+release filter
pattern in `compass-api/internal/api/server.go`.

## Open questions

- **Tag-set discovery diagnostics surface area.** When `Bundle.status.conditions[ReleaseDiscovery]`
  reports `NoCompleteRelease` with a per-artifact tag breakdown, should
  the tool flatten that into a top-level summary, or pass the raw
  condition through? Lean toward passing through — agents read
  `condition.message` directly and parse the breakdown in natural language.
- **Inclusion in `whoami`?** Could surface "Bundles you can read" as
  part of `whoami` so the agent has a starting map of the Pipeline.
  Probably not — `whoami` should stay an identity/permission snapshot,
  not a data dump.
- **Cross-Pipeline `list_bundlereleases` by version**, e.g. "which
  Pipelines are running 1.0.2 anywhere?" Bundle of work bigger than v1;
  out of scope for now.

## Out of scope

- Creating Bundles via MCP. Bundles are platform-team config; not
  something an agent should mutate.
- BundleRelease metadata mutation. BundleReleases are immutable by
  design (CEL `self == oldSelf`).
- Direct OCI registry queries (e.g. "list all tags in this repo even
  if Compass hasn't materialized them as releases"). The Bundle
  controller is the source of truth; MCP shouldn't reach around it.

## Depends on

- Nothing blocking. Compass-api already serves the data; this is
  purely additive tool wiring. Could ship before [sso-auth](../sso-auth/README.md)
  since these are read-only.
