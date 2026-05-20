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

// PromotionSummary is the projection of a Promotion CR exposed to the
// agent on list calls. Includes everything an agent needs to triage
// without burning context on managedFields / conditions / labels.
type PromotionSummary struct {
	Name              string `json:"name"`
	Environment       string `json:"environment"`
	Release           string `json:"release"`
	Phase             string `json:"phase,omitempty"`
	RequestedBy       string `json:"requestedBy,omitempty"`
	WorkflowRef       string `json:"workflowRef,omitempty"`
	StartedAt         string `json:"startedAt,omitempty"`
	CompletedAt       string `json:"completedAt,omitempty"`
	CreationTimestamp string `json:"creationTimestamp,omitempty"`
}

// promotionListResponse mirrors compass-api's PromotionList wrapper.
// promotionItem only pulls the fields PromotionSummary needs; the rest
// (system metadata, conditions) drop on decode.
type promotionListResponse struct {
	Items []promotionItem `json:"items"`
}

type promotionItem struct {
	Metadata objectMeta       `json:"metadata"`
	Spec     promotionSpec    `json:"spec"`
	Status   promotionStatus  `json:"status"`
}

type promotionSpec struct {
	EnvironmentRef   string `json:"environmentRef"`
	BundleReleaseRef string `json:"bundleReleaseRef"`
	RequestedBy      string `json:"requestedBy,omitempty"`
}

type promotionStatus struct {
	Phase       string `json:"phase,omitempty"`
	WorkflowRef string `json:"workflowRef,omitempty"`
	StartedAt   string `json:"startedAt,omitempty"`
	CompletedAt string `json:"completedAt,omitempty"`
}

func (p promotionItem) toSummary() PromotionSummary {
	return PromotionSummary{
		Name:              p.Metadata.Name,
		Environment:       p.Spec.EnvironmentRef,
		Release:           p.Spec.BundleReleaseRef,
		Phase:             p.Status.Phase,
		RequestedBy:       p.Spec.RequestedBy,
		WorkflowRef:       p.Status.WorkflowRef,
		StartedAt:         p.Status.StartedAt,
		CompletedAt:       p.Status.CompletedAt,
		CreationTimestamp: p.Metadata.CreationTimestamp,
	}
}

// ListPromotionsOpts carries the optional server-side filters compass-api
// supports on GET /api/pipelines/{p}/promotions. Empty strings are
// omitted from the query string.
type ListPromotionsOpts struct {
	Environment string
	Release     string
}

// StepSummary is one Pod-type node from an Argo Workflow projected for
// the agent. NodeID is the opaque identifier the agent passes back to
// get_promotion_step_logs; step is the friendlier displayName /
// templateName for the agent to refer to in prose.
type StepSummary struct {
	Step       string `json:"step"`
	NodeID     string `json:"nodeId"`
	Phase      string `json:"phase,omitempty"`
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
}

// workflowResponse mirrors the slice of the Argo Workflow object we
// care about — only status.nodes. The full workflow has hundreds of
// fields; this decode pattern drops everything else automatically.
type workflowResponse struct {
	Status struct {
		Nodes map[string]workflowNode `json:"nodes"`
	} `json:"status"`
}

type workflowNode struct {
	Type         string `json:"type"`
	DisplayName  string `json:"displayName"`
	TemplateName string `json:"templateName"`
	Phase        string `json:"phase"`
	StartedAt    string `json:"startedAt"`
	FinishedAt   string `json:"finishedAt"`
}

// stepLogsResponse is the body of GET .../steps/{node}/logs — a single
// "logs" string. Field name matches compass-api's getStepLogs handler.
type stepLogsResponse struct {
	Logs string `json:"logs"`
}
