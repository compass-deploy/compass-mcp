package client

import (
	"context"
	"fmt"
	"net/url"
	"sort"
)

// ListPromotionSteps fetches the Promotion's underlying Argo Workflow
// and projects each Pod-type node into a StepSummary. Steps are sorted
// by start time (oldest first) — the natural execution order — with
// pending steps (no startedAt) at the end. Returns an empty slice if
// the workflow has materialized but has no Pod nodes yet.
func (c *Client) ListPromotionSteps(ctx context.Context, pipeline, promotion string) ([]StepSummary, error) {
	if pipeline == "" || promotion == "" {
		return nil, fmt.Errorf("ListPromotionSteps: pipeline and promotion are required")
	}
	path := fmt.Sprintf("/api/pipelines/%s/promotions/%s/workflow",
		url.PathEscape(pipeline), url.PathEscape(promotion))

	var wf workflowResponse
	if err := c.doJSON(ctx, "GET", path, nil, &wf); err != nil {
		return nil, err
	}

	out := make([]StepSummary, 0, len(wf.Status.Nodes))
	for id, node := range wf.Status.Nodes {
		if node.Type != "Pod" {
			continue
		}
		step := node.DisplayName
		if step == "" {
			step = node.TemplateName
		}
		out = append(out, StepSummary{
			Step:       step,
			NodeID:     id,
			Phase:      node.Phase,
			StartedAt:  node.StartedAt,
			FinishedAt: node.FinishedAt,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		ai, bi := out[i].StartedAt, out[j].StartedAt
		// Zero/empty timestamps sort last so the agent sees executed
		// steps first and pending ones at the bottom.
		switch {
		case ai == "" && bi == "":
			return out[i].NodeID < out[j].NodeID
		case ai == "":
			return false
		case bi == "":
			return true
		default:
			return ai < bi
		}
	})
	return out, nil
}

// GetPromotionStepLogs returns the last 500 lines of the "main"
// container log for the pod backing the given workflow node. The
// `node` argument is the opaque NodeID returned by ListPromotionSteps;
// compass-api translates it into the pod name using Argo's naming
// convention.
func (c *Client) GetPromotionStepLogs(ctx context.Context, pipeline, promotion, node string) (string, error) {
	if pipeline == "" || promotion == "" || node == "" {
		return "", fmt.Errorf("GetPromotionStepLogs: pipeline, promotion, and node are required")
	}
	path := fmt.Sprintf("/api/pipelines/%s/promotions/%s/steps/%s/logs",
		url.PathEscape(pipeline), url.PathEscape(promotion), url.PathEscape(node))

	var resp stepLogsResponse
	if err := c.doJSON(ctx, "GET", path, nil, &resp); err != nil {
		return "", err
	}
	return resp.Logs, nil
}
