package hookdeck

import (
	"context"
	"fmt"
	"time"
)

// Attempt status values returned by the API.
const (
	OutpostAttemptStatusSuccess = "success"
	OutpostAttemptStatusFailed  = "failed"
)

// OutpostAttempt represents a single delivery attempt of an event to a
// destination.
//
// Event and Destination are only populated when requested through the API's
// include parameter; otherwise they are nil.
type OutpostAttempt struct {
	ID            string                 `json:"id"`
	TenantID      string                 `json:"tenant_id"`
	EventID       string                 `json:"event_id"`
	DestinationID string                 `json:"destination_id"`
	Status        string                 `json:"status"`
	Code          string                 `json:"code"`
	AttemptNumber int                    `json:"attempt_number"`
	Manual        bool                   `json:"manual"`
	Time          time.Time              `json:"time"`
	ResponseData  map[string]interface{} `json:"response_data,omitempty"`
	Event         *OutpostEvent          `json:"event,omitempty"`
	Destination   *OutpostDestination    `json:"destination,omitempty"`
}

// Succeeded reports whether the attempt was delivered successfully.
func (a *OutpostAttempt) Succeeded() bool {
	return a != nil && a.Status == OutpostAttemptStatusSuccess
}

// OutpostAttemptListResponse is the paginated response from listing attempts.
type OutpostAttemptListResponse struct {
	Models     []OutpostAttempt   `json:"models"`
	Pagination PaginationResponse `json:"pagination"`
}

// OutpostAttemptListParams are the filters accepted by ListOutpostAttempts.
//
// The API exposes attempts both globally and scoped to a tenant's destination.
// When both TenantID and DestinationID are set, the tenant-scoped endpoint is
// used; the filters and response shape are the same either way.
type OutpostAttemptListParams struct {
	TenantID        string
	DestinationID   string
	TenantIDs       []string
	EventIDs        []string
	DestinationIDs  []string
	DestinationType []string
	Topics          []string
	Status          string
	TimeAfter       string
	TimeBefore      string
	Include         []string
	Limit           int
	OrderBy         string
	Dir             string
	Next            string
	Prev            string
}

// usesTenantScopedPath reports whether the request can address the
// tenant-scoped attempts endpoint.
func (p OutpostAttemptListParams) usesTenantScopedPath() bool {
	return p.TenantID != "" && p.DestinationID != ""
}

// ListOutpostAttempts retrieves a page of delivery attempts.
func (c *Client) ListOutpostAttempts(ctx context.Context, params OutpostAttemptListParams) (*OutpostAttemptListResponse, error) {
	scalar := map[string]string{
		"status":   params.Status,
		"order_by": params.OrderBy,
		"dir":      params.Dir,
		"next":     params.Next,
		"prev":     params.Prev,
	}
	if params.Limit > 0 {
		scalar["limit"] = fmt.Sprintf("%d", params.Limit)
	}
	setOutpostTimeRange(scalar, "time", params.TimeAfter, params.TimeBefore)

	lists := map[string][]string{
		"event_id": params.EventIDs,
		"topic":    params.Topics,
		"include":  params.Include,
	}

	path := APIPathPrefix + "/attempts"
	if params.usesTenantScopedPath() {
		scoped, err := apiPath("tenants", params.TenantID, "destinations", params.DestinationID, "attempts")
		if err != nil {
			return nil, err
		}
		path = scoped
	} else {
		// These filters are only meaningful on the global endpoint — the
		// tenant-scoped one already constrains both dimensions via the path.
		lists["tenant_id"] = params.TenantIDs
		lists["destination_id"] = params.DestinationIDs
		lists["destination_type"] = params.DestinationType
	}

	resp, err := c.Get(ctx, path, outpostQuery(scalar, lists), nil)
	if err != nil {
		return nil, err
	}

	var result OutpostAttemptListResponse
	if _, err := postprocessJsonResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse attempt list response: %w", err)
	}

	return &result, nil
}

// OutpostAttemptGetParams are the options accepted by GetOutpostAttempt.
type OutpostAttemptGetParams struct {
	TenantID      string
	DestinationID string
	Include       []string
}

// GetOutpostAttempt retrieves a single delivery attempt. As with the list
// endpoint, supplying both TenantID and DestinationID uses the tenant-scoped
// route.
func (c *Client) GetOutpostAttempt(ctx context.Context, attemptID string, params OutpostAttemptGetParams) (*OutpostAttempt, error) {
	scalar := map[string]string{}

	segments := []string{"attempts", attemptID}
	if params.TenantID != "" && params.DestinationID != "" {
		segments = []string{"tenants", params.TenantID, "destinations", params.DestinationID, "attempts", attemptID}
	} else {
		scalar["tenant_id"] = params.TenantID
	}

	path, err := apiPath(segments...)
	if err != nil {
		return nil, err
	}

	query := outpostQuery(scalar, map[string][]string{"include": params.Include})

	resp, err := c.Get(ctx, path, query, nil)
	if err != nil {
		return nil, err
	}

	var attempt OutpostAttempt
	if _, err := postprocessJsonResponse(resp, &attempt); err != nil {
		return nil, fmt.Errorf("failed to parse attempt response: %w", err)
	}

	return &attempt, nil
}
