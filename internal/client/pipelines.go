package client

import "context"

// ListPipelines returns the trimmed summary list of every Pipeline the
// caller can see. Compass-api enforces RBAC via k8s impersonation, so a
// pipeline the user can't List is just absent from the response (not an
// error).
func (c *Client) ListPipelines(ctx context.Context) ([]PipelineSummary, error) {
	var raw pipelineListResponse
	if err := c.doJSON(ctx, "GET", "/api/pipelines", nil, &raw); err != nil {
		return nil, err
	}
	out := make([]PipelineSummary, 0, len(raw.Items))
	for _, item := range raw.Items {
		out = append(out, item.toSummary())
	}
	return out, nil
}
