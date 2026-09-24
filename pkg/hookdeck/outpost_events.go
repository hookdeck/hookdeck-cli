package hookdeck

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// OutpostEvent represents a published event. Events are created through Publish
// rather than a create endpoint, so this type is read-only.
type OutpostEvent struct {
	ID                    string                 `json:"id"`
	TenantID              string                 `json:"tenant_id"`
	Topic                 string                 `json:"topic"`
	MatchedDestinationIDs []string               `json:"matched_destination_ids"`
	Time                  time.Time              `json:"time"`
	EligibleForRetry      *bool                  `json:"eligible_for_retry,omitempty"`
	Metadata              map[string]string      `json:"metadata"`
	Data                  map[string]interface{} `json:"data"`
}

// OutpostEventListResponse is the paginated response from listing events.
type OutpostEventListResponse struct {
	Models     []OutpostEvent     `json:"models"`
	Pagination PaginationResponse `json:"pagination"`
}

// OutpostEventListParams are the filters accepted by ListOutpostEvents.
//
// TimeAfter and TimeBefore are ISO 8601 datetimes and map to the API's
// time[gte] / time[lte] comparison filters.
type OutpostEventListParams struct {
	IDs            []string
	TenantIDs      []string
	DestinationIDs []string
	Topics         []string
	TimeAfter      string
	TimeBefore     string
	Limit          int
	OrderBy        string
	Dir            string
	Next           string
	Prev           string
}

// ListOutpostEvents retrieves a page of events.
func (c *Client) ListOutpostEvents(ctx context.Context, params OutpostEventListParams) (*OutpostEventListResponse, error) {
	scalar := map[string]string{
		"order_by": params.OrderBy,
		"dir":      params.Dir,
		"next":     params.Next,
		"prev":     params.Prev,
	}
	if params.Limit > 0 {
		scalar["limit"] = fmt.Sprintf("%d", params.Limit)
	}
	setOutpostTimeRange(scalar, "time", params.TimeAfter, params.TimeBefore)

	query := outpostQuery(scalar, map[string][]string{
		"id":             params.IDs,
		"tenant_id":      params.TenantIDs,
		"destination_id": params.DestinationIDs,
		"topic":          params.Topics,
	})

	resp, err := c.Get(ctx, APIPathPrefix+"/events", query, nil)
	if err != nil {
		return nil, err
	}

	var result OutpostEventListResponse
	if _, err := postprocessJsonResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse event list response: %w", err)
	}

	return &result, nil
}

// GetOutpostEvent retrieves a single event. tenantID is optional.
func (c *Client) GetOutpostEvent(ctx context.Context, eventID, tenantID string) (*OutpostEvent, error) {
	path, err := apiPath("events", eventID)
	if err != nil {
		return nil, err
	}

	query := outpostQuery(map[string]string{"tenant_id": tenantID}, nil)

	resp, err := c.Get(ctx, path, query, nil)
	if err != nil {
		return nil, err
	}

	var event OutpostEvent
	if _, err := postprocessJsonResponse(resp, &event); err != nil {
		return nil, fmt.Errorf("failed to parse event response: %w", err)
	}

	return &event, nil
}

// OutpostRetryRequest is the body for retrying delivery of an event to a
// destination.
type OutpostRetryRequest struct {
	EventID       string `json:"event_id"`
	DestinationID string `json:"destination_id"`
}

// OutpostRetryResponse is the acknowledgement returned by a retry.
type OutpostRetryResponse struct {
	Success bool `json:"success"`
}

// RetryOutpostEvent asks the API to deliver an event to a destination again.
// The retry is queued rather than performed inline, so a successful response
// means accepted, not delivered.
func (c *Client) RetryOutpostEvent(ctx context.Context, req *OutpostRetryRequest) (*OutpostRetryResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal retry request: %w", err)
	}

	resp, err := c.Post(ctx, APIPathPrefix+"/retry", data, nil)
	if err != nil {
		return nil, err
	}

	var result OutpostRetryResponse
	if _, err := postprocessJsonResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse retry response: %w", err)
	}

	return &result, nil
}
