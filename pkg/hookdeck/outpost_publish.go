package hookdeck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// withoutStoredAuth returns a shallow clone with the stored API key cleared, so
// that PerformRequest leaves the Authorization header alone. The underlying
// http.Client and its connection pool are shared.
func (c *Client) withoutStoredAuth() *Client {
	clone := *c
	clone.APIKey = ""
	return &clone
}

// OutpostPublishRequest is the body for publishing an event.
//
// ID is optional; supplying one makes the publish idempotent, and republishing
// the same ID reports Duplicate rather than creating a second event.
type OutpostPublishRequest struct {
	ID               string                 `json:"id,omitempty"`
	TenantID         string                 `json:"tenant_id"`
	Topic            string                 `json:"topic"`
	DestinationID    string                 `json:"destination_id,omitempty"`
	EligibleForRetry *bool                  `json:"eligible_for_retry,omitempty"`
	Time             *time.Time             `json:"time,omitempty"`
	Metadata         map[string]string      `json:"metadata,omitempty"`
	Data             map[string]interface{} `json:"data,omitempty"`
}

// OutpostPublishResponse is the acknowledgement returned by a publish.
//
// Publishing is asynchronous, so DestinationIDs records which destinations the
// event matched at publish time — not which have received it.
type OutpostPublishResponse struct {
	ID             string   `json:"id"`
	Duplicate      bool     `json:"duplicate"`
	DestinationIDs []string `json:"destination_ids"`
}

// PublishOutpostEvent publishes an event to a topic.
//
// This endpoint requires a Hookdeck Project API key supplied as a bearer token.
// The CLI key stored by `hookdeck login` is not accepted, which is why apiKey is
// an explicit argument here rather than being taken from the client.
func (c *Client) PublishOutpostEvent(ctx context.Context, apiKey string, req *OutpostPublishRequest) (*OutpostPublishResponse, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("a Hookdeck Project API key is required to publish")
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal publish request: %w", err)
	}

	// PerformRequest applies the client's stored key as basic auth whenever one
	// is set, which would overwrite the Authorization header below. Publishing
	// therefore goes out through a clone with no stored key, so the bearer token
	// is the only credential on the request. Everything else — base URL, project
	// scoping, telemetry, the shared HTTP client — is preserved.
	publishClient := c.withoutStoredAuth()

	httpReq, err := publishClient.newRequest(ctx, http.MethodPost, APIPathPrefix+"/publish", data)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := publishClient.PerformRequest(ctx, httpReq)
	if err != nil {
		return nil, err
	}

	var result OutpostPublishResponse
	if _, err := postprocessJsonResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse publish response: %w", err)
	}

	return &result, nil
}
