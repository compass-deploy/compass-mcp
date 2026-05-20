package client

import "encoding/json"

// Capabilities mirrors compass-api's per-pipeline verb permissions in
// /api/me. We carry it through verbatim so future tools can decide
// whether to even attempt a write (M2+ adds promote/approve/invalidate).
type Capabilities struct {
	Promote    bool `json:"promote"`
	Approve    bool `json:"approve"`
	Invalidate bool `json:"invalidate"`
}

// Me is the JSON body of GET /api/me. Field names track the upstream
// `meResponse` in compass-deploy/backend/internal/api/server.go.
type Me struct {
	AuthEnabled   bool                    `json:"authEnabled"`
	SSOEnabled    bool                    `json:"ssoEnabled"`
	Authenticated bool                    `json:"authenticated"`
	User          string                  `json:"user,omitempty"`
	Groups        []string                `json:"groups,omitempty"`
	Can           map[string]Capabilities `json:"can,omitempty"`
}

// PipelineSummary is the projection of a Pipeline CR that we expose to
// the agent. The full CR carries metadata.managedFields and other k8s
// bookkeeping the agent has no use for; declaring only what we want
// makes the JSON decoder drop the rest at decode time.
type PipelineSummary struct {
	Name              string `json:"name"`
	Namespace         string `json:"namespace,omitempty"`
	CreationTimestamp string `json:"creationTimestamp,omitempty"`
}

// pipelineListResponse mirrors the k8s List wrapper compass-api returns
// for GET /api/pipelines, with each item projected via the same trim
// strategy. The intermediate item type pulls metadata.{name,
// creationTimestamp} and status.namespace out of the nested objects so
// the agent-visible summary stays flat.
type pipelineListResponse struct {
	Items []pipelineItem `json:"items"`
}

type pipelineItem struct {
	Metadata objectMeta `json:"metadata"`
	Status   struct {
		Namespace string `json:"namespace,omitempty"`
	} `json:"status"`
}

// objectMeta is the subset of k8s ObjectMeta we ever care about.
type objectMeta struct {
	Name              string `json:"name"`
	CreationTimestamp string `json:"creationTimestamp,omitempty"`
}

func (p pipelineItem) toSummary() PipelineSummary {
	return PipelineSummary{
		Name:              p.Metadata.Name,
		Namespace:         p.Status.Namespace,
		CreationTimestamp: p.Metadata.CreationTimestamp,
	}
}

// RawPromotion is the full Promotion CR forwarded verbatim. We use
// json.RawMessage so the client doesn't re-encode the bytes; the agent
// receives whatever compass-api returns (status conditions, audit
// fields, the full spec). Used by get_promotion where one record's
// fidelity matters.
type RawPromotion = json.RawMessage
