package hookdeck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Connection represents a Hookdeck connection
type Connection struct {
	ID          string       `json:"id"`
	Name        *string      `json:"name"`
	FullName    *string      `json:"full_name"`
	Description *string      `json:"description"`
	TeamID      string       `json:"team_id"`
	Destination *Destination `json:"destination"`
	Source      *Source      `json:"source"`
	Rules       []Rule       `json:"rules"`
	DisabledAt  *time.Time   `json:"disabled_at"`
	PausedAt    *time.Time   `json:"paused_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	CreatedAt   time.Time    `json:"created_at"`
}

// ConnectionCreateRequest represents the request to create a connection
type ConnectionCreateRequest struct {
	Name          *string                 `json:"name,omitempty"`
	Description   *string                 `json:"description,omitempty"`
	SourceID      *string                 `json:"source_id,omitempty"`
	DestinationID *string                 `json:"destination_id,omitempty"`
	Source        *SourceCreateInput      `json:"source,omitempty"`
	Destination   *DestinationCreateInput `json:"destination,omitempty"`
	Rules         []Rule                  `json:"rules,omitempty"`
}

// ConnectionListResponse represents the response from listing connections
type ConnectionListResponse struct {
	Models     []Connection       `json:"models"`
	Pagination PaginationResponse `json:"pagination"`
}

// ConnectionCountResponse represents the response from counting connections
type ConnectionCountResponse struct {
	Count int `json:"count"`
}

// PaginationResponse represents pagination metadata
type PaginationResponse struct {
	OrderBy string  `json:"order_by"`
	Dir     string  `json:"dir"`
	Limit   int     `json:"limit"`
	Next    *string `json:"next"`
	Prev    *string `json:"prev"`
}

// Rule represents a connection rule (union type)
type Rule map[string]interface{}

// ListConnections retrieves a list of connections with optional filters
func (c *Client) ListConnections(ctx context.Context, params map[string]string) (*ConnectionListResponse, error) {
	queryParams := url.Values{}
	for k, v := range params {
		queryParams.Add(k, v)
	}

	resp, err := c.Get(ctx, APIPathPrefix+"/connections", queryParams.Encode(), nil)
	if err != nil {
		return nil, err
	}

	var result ConnectionListResponse
	_, err = postprocessJsonResponse(resp, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection list response: %w", err)
	}

	return &result, nil
}

// GetConnection retrieves a single connection by ID
func (c *Client) GetConnection(ctx context.Context, id string) (*Connection, error) {
	path, err := apiPath("connections", id)
	if err != nil {
		return nil, err
	}

	resp, err := c.Get(ctx, path, "", nil)
	if err != nil {
		return nil, err
	}

	var connection Connection
	_, err = postprocessJsonResponse(resp, &connection)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection response: %w", err)
	}

	return &connection, nil
}

// CreateConnection creates a new connection
func (c *Client) CreateConnection(ctx context.Context, req *ConnectionCreateRequest) (*Connection, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal connection request: %w", err)
	}

	resp, err := c.Post(ctx, APIPathPrefix+"/connections", data, nil)
	if err != nil {
		return nil, err
	}

	var connection Connection
	_, err = postprocessJsonResponse(resp, &connection)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection response: %w", err)
	}

	return &connection, nil
}

// UpsertConnection creates or updates a connection by name
// Uses PUT /connections endpoint with name as the unique identifier
func (c *Client) UpsertConnection(ctx context.Context, req *ConnectionCreateRequest) (*Connection, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal connection upsert request: %w", err)
	}

	resp, err := c.Put(ctx, APIPathPrefix+"/connections", data, nil)
	if err != nil {
		return nil, err
	}

	var connection Connection
	_, err = postprocessJsonResponse(resp, &connection)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection response: %w", err)
	}

	return &connection, nil
}

// UpdateConnection updates an existing connection by ID
// Uses PUT /connections/{id} endpoint
func (c *Client) UpdateConnection(ctx context.Context, id string, req *ConnectionCreateRequest) (*Connection, error) {
	path, err := apiPath("connections", id)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal connection update request: %w", err)
	}

	resp, err := c.Put(ctx, path, data, nil)
	if err != nil {
		return nil, err
	}

	var connection Connection
	_, err = postprocessJsonResponse(resp, &connection)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection response: %w", err)
	}

	return &connection, nil
}

// DeleteConnection deletes a connection
func (c *Client) DeleteConnection(ctx context.Context, id string) error {
	path, err := apiPath("connections", id)
	if err != nil {
		return err
	}

	req, err := c.newRequest(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}

	resp, err := c.PerformRequest(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// EnableConnection enables a connection
func (c *Client) EnableConnection(ctx context.Context, id string) (*Connection, error) {
	return c.setConnectionState(ctx, id, "enable")
}

// DisableConnection disables a connection
func (c *Client) DisableConnection(ctx context.Context, id string) (*Connection, error) {
	return c.setConnectionState(ctx, id, "disable")
}

// PauseConnection pauses a connection
func (c *Client) PauseConnection(ctx context.Context, id string) (*Connection, error) {
	return c.setConnectionState(ctx, id, "pause")
}

// UnpauseConnection unpauses a connection
func (c *Client) UnpauseConnection(ctx context.Context, id string) (*Connection, error) {
	return c.setConnectionState(ctx, id, "unpause")
}

// setConnectionState applies one of the connection state-change actions, which
// differ only in the final path segment and all answer with the connection.
func (c *Client) setConnectionState(ctx context.Context, id, action string) (*Connection, error) {
	path, err := apiPath("connections", id, action)
	if err != nil {
		return nil, err
	}

	resp, err := c.Put(ctx, path, []byte("{}"), nil)
	if err != nil {
		return nil, err
	}

	var connection Connection
	if _, err := postprocessJsonResponse(resp, &connection); err != nil {
		return nil, fmt.Errorf("failed to parse connection response: %w", err)
	}

	return &connection, nil
}

// CountConnections counts connections matching the given filters
func (c *Client) CountConnections(ctx context.Context, params map[string]string) (*ConnectionCountResponse, error) {
	queryParams := url.Values{}
	for k, v := range params {
		queryParams.Add(k, v)
	}

	resp, err := c.Get(ctx, APIPathPrefix+"/connections/count", queryParams.Encode(), nil)
	if err != nil {
		return nil, err
	}

	var result ConnectionCountResponse
	_, err = postprocessJsonResponse(resp, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection count response: %w", err)
	}

	return &result, nil
}

// newRequest creates a new HTTP request (helper for DELETE and PATCH)
func (c *Client) newRequest(ctx context.Context, method, path string, body []byte) (*http.Request, error) {
	u, err := c.resolveRequestURL(path)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewBuffer(body)
	}

	return http.NewRequest(method, u.String(), bodyReader)
}
