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

// TenantExistsForPublish reports whether a tenant exists in the project the
// publish credential routes to.
//
// This matters because publishing follows the credential, not the client's
// active project. A publish for a tenant that does not exist there is accepted
// with a 202 and an event id, matches nothing, is never delivered, and does not
// appear in any event list — so the caller sees a success and no trace of it.
// Checking first turns that into an answerable error.
//
// The lookup deliberately uses the same credential and host as the publish, so
// it resolves to the same project the event would go to.
func (c *Client) TenantExistsForPublish(ctx context.Context, apiKey, tenantID string) (bool, error) {
	if apiKey == "" || tenantID == "" {
		return false, fmt.Errorf("an API key and tenant are required to check a tenant")
	}

	lookup := c.withoutStoredAuth()
	// Publishing resolves the project from the credential alone. Resource reads
	// additionally honour the project header, so leaving it set would check a
	// different project from the one the event goes to — and, when the key is not
	// valid for it, fail with a 401 that hides the answer entirely.
	lookup.ProjectID = ""

	req, err := lookup.newRequest(ctx, http.MethodGet, outpostPath("tenants", tenantID), nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := lookup.PerformRequest(ctx, req)
	if err != nil {
		if IsNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	defer resp.Body.Close()

	return true, nil
}
