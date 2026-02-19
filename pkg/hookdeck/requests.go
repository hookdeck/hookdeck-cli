package hookdeck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"time"
)

// Request represents a raw inbound webhook received by a source
type Request struct {
	ID                   string                 `json:"id"`
	SourceID             string                 `json:"source_id"`
	Verified             bool                   `json:"verified"`
	RejectionCause       *string                `json:"rejection_cause,omitempty"`
	EventsCount          int                    `json:"events_count"`
	CliEventsCount       int                    `json:"cli_events_count"`
	IgnoredCount         int                    `json:"ignored_count"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
	IngestedAt           *time.Time             `json:"ingested_at,omitempty"`
	OriginalEventDataID  *string                `json:"original_event_data_id,omitempty"`
	Data                 *RequestData           `json:"data,omitempty"`
	TeamID               string                 `json:"team_id"`
}

// RequestData holds optional request snapshot
type RequestData struct {
	Headers     map[string]interface{} `json:"headers,omitempty"`
	Body        interface{}            `json:"body,omitempty"`
	Path        string                 `json:"path,omitempty"`
	ParsedQuery map[string]interface{} `json:"parsed_query,omitempty"`
}

// RequestListResponse is the response from listing requests
type RequestListResponse struct {
	Models     []Request            `json:"models"`
	Pagination PaginationResponse  `json:"pagination"`
}

// RequestRetryRequest is the body for POST /requests/{id}/retry. WebhookIDs limits retry to those connections; omit or empty for all.
type RequestRetryRequest struct {
	WebhookIDs []string `json:"webhook_ids,omitempty"`
}

// ListRequests retrieves requests with optional filters
func (c *Client) ListRequests(ctx context.Context, params map[string]string) (*RequestListResponse, error) {
	queryParams := url.Values{}
	for k, v := range params {
		queryParams.Add(k, v)
	}
	resp, err := c.Get(ctx, APIPathPrefix+"/requests", queryParams.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var result RequestListResponse
	_, err = postprocessJsonResponse(resp, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse request list response: %w", err)
	}
	return &result, nil
}

// GetRequest retrieves a single request by ID
func (c *Client) GetRequest(ctx context.Context, id string, params map[string]string) (*Request, error) {
	queryStr := ""
	if len(params) > 0 {
		q := url.Values{}
		for k, v := range params {
			q.Add(k, v)
		}
		queryStr = q.Encode()
	}
	resp, err := c.Get(ctx, APIPathPrefix+"/requests/"+id, queryStr, nil)
	if err != nil {
		return nil, err
	}
	var req Request
	_, err = postprocessJsonResponse(resp, &req)
	if err != nil {
		return nil, fmt.Errorf("failed to parse request response: %w", err)
	}
	return &req, nil
}

// RetryRequest retries a request by ID. Pass nil or empty WebhookIDs to retry on all connections; otherwise only for the given connection IDs.
func (c *Client) RetryRequest(ctx context.Context, requestID string, body *RequestRetryRequest) error {
	if body == nil {
		body = &RequestRetryRequest{}
	}
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request retry body: %w", err)
	}
	resp, err := c.Post(ctx, APIPathPrefix+"/requests/"+requestID+"/retry", data, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkAndPrintError(resp)
}

// GetRequestEvents returns the list of events for a request (GET /requests/{id}/events)
func (c *Client) GetRequestEvents(ctx context.Context, requestID string, params map[string]string) (*EventListResponse, error) {
	queryStr := ""
	if len(params) > 0 {
		q := url.Values{}
		for k, v := range params {
			q.Add(k, v)
		}
		queryStr = q.Encode()
	}
	resp, err := c.Get(ctx, APIPathPrefix+"/requests/"+requestID+"/events", queryStr, nil)
	if err != nil {
		return nil, err
	}
	var result EventListResponse
	_, err = postprocessJsonResponse(resp, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse request events response: %w", err)
	}
	return &result, nil
}

// GetRequestIgnoredEvents returns the list of ignored events for a request (GET /requests/{id}/ignored_events)
func (c *Client) GetRequestIgnoredEvents(ctx context.Context, requestID string, params map[string]string) (*EventListResponse, error) {
	queryStr := ""
	if len(params) > 0 {
		q := url.Values{}
		for k, v := range params {
			q.Add(k, v)
		}
		queryStr = q.Encode()
	}
	resp, err := c.Get(ctx, APIPathPrefix+"/requests/"+requestID+"/ignored_events", queryStr, nil)
	if err != nil {
		return nil, err
	}
	var result EventListResponse
	_, err = postprocessJsonResponse(resp, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse request ignored events response: %w", err)
	}
	return &result, nil
}

// GetRequestRawBody returns the raw body of a request (GET /requests/{id}/raw_body)
func (c *Client) GetRequestRawBody(ctx context.Context, requestID string) ([]byte, error) {
	resp, err := c.Get(ctx, APIPathPrefix+"/requests/"+requestID+"/raw_body", "", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := checkAndPrintError(resp); err != nil {
		return nil, err
	}
	return io.ReadAll(resp.Body)
}
