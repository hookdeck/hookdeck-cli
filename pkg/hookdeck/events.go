package hookdeck

import (
	"context"
	"fmt"
	"io"
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
	Models     []Event             `json:"models"`
	Pagination PaginationResponse  `json:"pagination"`
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
	queryStr := ""
	if len(params) > 0 {
		q := url.Values{}
		for k, v := range params {
			q.Add(k, v)
		}
		queryStr = q.Encode()
	}
	resp, err := c.Get(ctx, APIPathPrefix+"/events/"+id, queryStr, nil)
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

// RetryEvent retries an event by ID (POST /events/{id}/retry; no request body)
func (c *Client) RetryEvent(ctx context.Context, eventID string) error {
	resp, err := c.Post(ctx, APIPathPrefix+"/events/"+eventID+"/retry", []byte("{}"), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkAndPrintError(resp)
}

// CancelEvent cancels an event by ID (PUT /events/{id}/cancel; no request body)
func (c *Client) CancelEvent(ctx context.Context, eventID string) error {
	resp, err := c.Put(ctx, APIPathPrefix+"/events/"+eventID+"/cancel", []byte("{}"), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkAndPrintError(resp)
}

// MuteEvent mutes an event by ID (PUT /events/{id}/mute; no request body)
func (c *Client) MuteEvent(ctx context.Context, eventID string) error {
	resp, err := c.Put(ctx, APIPathPrefix+"/events/"+eventID+"/mute", []byte("{}"), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkAndPrintError(resp)
}

// GetEventRawBody returns the raw body of an event (GET /events/{id}/raw_body)
func (c *Client) GetEventRawBody(ctx context.Context, eventID string) ([]byte, error) {
	resp, err := c.Get(ctx, APIPathPrefix+"/events/"+eventID+"/raw_body", "", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := checkAndPrintError(resp); err != nil {
		return nil, err
	}
	return io.ReadAll(resp.Body)
}
