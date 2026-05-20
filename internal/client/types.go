package client

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
