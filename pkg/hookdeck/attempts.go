package hookdeck

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// EventAttempt represents a single delivery attempt for an event
type EventAttempt struct {
	ID              string     `json:"id"`
	TeamID          string     `json:"team_id"`
	EventID         string     `json:"event_id"`
	DestinationID   string     `json:"destination_id"`
	ResponseStatus  *int       `json:"response_status,omitempty"`
	AttemptNumber   int        `json:"attempt_number"`
	Trigger         string     `json:"trigger"`
	ErrorCode       *string    `json:"error_code,omitempty"`
	Body            interface{} `json:"body,omitempty"` // API may return string or object
	RequestedURL    string     `json:"requested_url"`
	HTTPMethod      string     `json:"http_method"`
	BulkRetryID     *string    `json:"bulk_retry_id,omitempty"`
	Status          string     `json:"status"`
	SuccessfulAt    *time.Time `json:"successful_at,omitempty"`
	DeliveredAt     *time.Time `json:"delivered_at,omitempty"`
}

// EventAttemptListResponse is the response from listing attempts (EventAttemptPaginatedResult)
type EventAttemptListResponse struct {
	Models     []EventAttempt      `json:"models"`
	Pagination PaginationResponse   `json:"pagination"`
	Count      *int                `json:"count,omitempty"`
}

// ListAttempts retrieves attempts for an event (params: event_id required; order_by, dir, limit, next, prev)
func (c *Client) ListAttempts(ctx context.Context, params map[string]string) (*EventAttemptListResponse, error) {
	queryParams := url.Values{}
	for k, v := range params {
		queryParams.Add(k, v)
	}
	resp, err := c.Get(ctx, APIPathPrefix+"/attempts", queryParams.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var result EventAttemptListResponse
	_, err = postprocessJsonResponse(resp, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse attempt list response: %w", err)
	}
	return &result, nil
}

// GetAttempt retrieves a single attempt by ID
func (c *Client) GetAttempt(ctx context.Context, id string) (*EventAttempt, error) {
	resp, err := c.Get(ctx, APIPathPrefix+"/attempts/"+id, "", nil)
	if err != nil {
		return nil, err
	}
	var attempt EventAttempt
	_, err = postprocessJsonResponse(resp, &attempt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse attempt response: %w", err)
	}
	return &attempt, nil
}
