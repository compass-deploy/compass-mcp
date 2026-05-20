package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// ListPromotions returns the trimmed promotion list for a single
// pipeline. environment / release in opts are forwarded as query params;
// compass-api handles the actual filtering server-side.
func (c *Client) ListPromotions(ctx context.Context, pipeline string, opts ListPromotionsOpts) ([]PromotionSummary, error) {
	if pipeline == "" {
		return nil, fmt.Errorf("ListPromotions: pipeline is required")
	}
	path := fmt.Sprintf("/api/pipelines/%s/promotions", url.PathEscape(pipeline))
	q := url.Values{}
	if opts.Environment != "" {
		q.Set("environment", opts.Environment)
	}
	if opts.Release != "" {
		q.Set("release", opts.Release)
	}
	if len(q) > 0 {
		path = path + "?" + q.Encode()
	}

	var raw promotionListResponse
	if err := c.doJSON(ctx, "GET", path, nil, &raw); err != nil {
		return nil, err
	}
	out := make([]PromotionSummary, 0, len(raw.Items))
	for _, item := range raw.Items {
		out = append(out, item.toSummary())
	}
	return out, nil
}

// GetPromotion returns the full Promotion CR JSON as compass-api
// returned it. Forwarded as RawMessage so the agent sees the same
// structure kubectl-get would, including conditions and audit fields.
func (c *Client) GetPromotion(ctx context.Context, pipeline, name string) (json.RawMessage, error) {
	if pipeline == "" || name == "" {
		return nil, fmt.Errorf("GetPromotion: pipeline and name are required")
	}
	path := fmt.Sprintf("/api/pipelines/%s/promotions/%s",
		url.PathEscape(pipeline), url.PathEscape(name))
	var raw json.RawMessage
	if err := c.doJSON(ctx, "GET", path, nil, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}
