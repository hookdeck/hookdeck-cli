package hookdeck

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Event represents a Hookdeck event (processed webhook delivery)
type Event struct {
	ID             string     `json:"id"`
	Status         string     `json:"status"`
	WebhookID      string     `json:"webhook_id"`
	SourceID       string     `json:"source_id"`
	DestinationID  string     `json:"destination_id"`
	RequestID      string     `json:"request_id"`
	Attempts       int        `json:"attempts"`
	ResponseStatus *int       `json:"response_status,omitempty"`
	ErrorCode      *string    `json:"error_code,omitempty"`
	CliID          *string    `json:"cli_id,omitempty"`
	EventDataID    *string    `json:"event_data_id,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	SuccessfulAt   *time.Time `json:"successful_at,omitempty"`
	LastAttemptAt  *time.Time `json:"last_attempt_at,omitempty"`
	NextAttemptAt  *time.Time `json:"next_attempt_at,omitempty"`
	Data           *EventData `json:"data,omitempty"`
	TeamID         string     `json:"team_id"`
}

// EventData holds optional request snapshot on the event
type EventData struct {
	Headers     map[string]interface{} `json:"headers,omitempty"`
	Body        interface{}            `json:"body,omitempty"`
	Path        string                 `json:"path,omitempty"`
	ParsedQuery map[string]interface{} `json:"parsed_query,omitempty"`
}

// EventListResponse is the response from listing events
type EventListResponse struct {
	Models     []Event            `json:"models"`
	Pagination PaginationResponse `json:"pagination"`
}

// ListEvents retrieves events with optional filters (params: webhook_id, status, source_id, destination_id, limit, order_by, dir, next, prev, etc.)
func (c *Client) ListEvents(ctx context.Context, params map[string]string) (*EventListResponse, error) {
	queryParams := url.Values{}
	for k, v := range params {
		queryParams.Add(k, v)
	}
	resp, err := c.Get(ctx, APIPathPrefix+"/events", queryParams.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var result EventListResponse
	_, err = postprocessJsonResponse(resp, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse event list response: %w", err)
	}
	return &result, nil
}

// GetEvent retrieves a single event by ID
func (c *Client) GetEvent(ctx context.Context, id string, params map[string]string) (*Event, error) {
	path, err := apiPath("events", id)
	if err != nil {
		return nil, err
	}
	queryStr := ""
	if len(params) > 0 {
		q := url.Values{}
		for k, v := range params {
			q.Add(k, v)
		}
		queryStr = q.Encode()
	}
	resp, err := c.Get(ctx, path, queryStr, nil)
	if err != nil {
		return nil, err
	}
	var event Event
	_, err = postprocessJsonResponse(resp, &event)
	if err != nil {
		return nil, fmt.Errorf("failed to parse event response: %w", err)
	}
	return &event, nil
}

// RetryEvent retries an event by ID (POST /events/{id}/retry) and returns the
// event as it stands afterwards.
func (c *Client) RetryEvent(ctx context.Context, eventID string) (*Event, error) {
	return c.eventStateChange(ctx, eventID, "retry", http.MethodPost)
}

// CancelEvent cancels an event by ID (PUT /events/{id}/cancel) and returns the
// event as it stands afterwards.
func (c *Client) CancelEvent(ctx context.Context, eventID string) (*Event, error) {
	return c.eventStateChange(ctx, eventID, "cancel", http.MethodPut)
}

// MuteEvent mutes an event by ID (PUT /events/{id}/mute) and returns the event
// as it stands afterwards.
func (c *Client) MuteEvent(ctx context.Context, eventID string) (*Event, error) {
	return c.eventStateChange(ctx, eventID, "mute", http.MethodPut)
}

// eventStateChange applies one of the by-id event mutations and decodes the
// event the API answers with.
//
// These used to discard the response body and report success from the status
// code alone, which meant callers asserted an outcome nobody had checked. The
// API answers 200 for a no-op — cancelling an already-delivered event leaves it
// SUCCESSFUL — so "cancel" was reported for events that were never cancelled.
// The response says what actually happened; returning it lets the caller say so
// too.
func (c *Client) eventStateChange(ctx context.Context, eventID, action, method string) (*Event, error) {
	path, err := apiPath("events", eventID, action)
	if err != nil {
		return nil, err
	}

	var resp *http.Response
	if method == http.MethodPost {
		resp, err = c.Post(ctx, path, []byte("{}"), nil)
	} else {
		resp, err = c.Put(ctx, path, []byte("{}"), nil)
	}
	if err != nil {
		return nil, err
	}

	var event Event
	if _, err := postprocessJsonResponse(resp, &event); err != nil {
		return nil, fmt.Errorf("failed to parse event %s response: %w", action, err)
	}
	return &event, nil
}

// GetEventRawBody returns the raw body of an event (GET /events/{id}/raw_body)
func (c *Client) GetEventRawBody(ctx context.Context, eventID string) ([]byte, error) {
	path, err := apiPath("events", eventID, "raw_body")
	if err != nil {
		return nil, err
	}
	resp, err := c.Get(ctx, path, "", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := checkAndPrintError(resp); err != nil {
		return nil, err
	}
	return io.ReadAll(resp.Body)
}
