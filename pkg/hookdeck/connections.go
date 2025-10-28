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

// Source represents a Hookdeck source
type Source struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description *string                `json:"description"`
	URL         string                 `json:"url"`
	Type        string                 `json:"type"`
	Config      map[string]interface{} `json:"config"`
	DisabledAt  *time.Time             `json:"disabled_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	CreatedAt   time.Time              `json:"created_at"`
}

// SourceCreateInput represents input for creating a source inline
type SourceCreateInput struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Description *string                `json:"description,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// Destination represents a Hookdeck destination
type Destination struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description *string                `json:"description"`
	URL         *string                `json:"url"`
	Type        string                 `json:"type"`
	CliPath     *string                `json:"cli_path"`
	Config      map[string]interface{} `json:"config"`
	DisabledAt  *time.Time             `json:"disabled_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	CreatedAt   time.Time              `json:"created_at"`
}

// DestinationCreateInput represents input for creating a destination inline
type DestinationCreateInput struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Description *string                `json:"description,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

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

// ConnectionUpdateRequest represents the request to update a connection
type ConnectionUpdateRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
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

	resp, err := c.Get(ctx, "/2024-03-01/connections", queryParams.Encode(), nil)
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
	resp, err := c.Get(ctx, fmt.Sprintf("/2024-03-01/connections/%s", id), "", nil)
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

	resp, err := c.Post(ctx, "/2024-03-01/connections", data, nil)
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

// UpdateConnection updates an existing connection
func (c *Client) UpdateConnection(ctx context.Context, id string, req *ConnectionUpdateRequest) (*Connection, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal connection update request: %w", err)
	}

	resp, err := c.Put(ctx, fmt.Sprintf("/2024-03-01/connections/%s", id), data, nil)
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
	url := fmt.Sprintf("/2024-03-01/connections/%s", id)
	req, err := c.newRequest(ctx, "DELETE", url, nil)
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
	resp, err := c.Put(ctx, fmt.Sprintf("/2024-03-01/connections/%s/enable", id), []byte("{}"), nil)
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

// DisableConnection disables a connection
func (c *Client) DisableConnection(ctx context.Context, id string) (*Connection, error) {
	resp, err := c.Put(ctx, fmt.Sprintf("/2024-03-01/connections/%s/disable", id), []byte("{}"), nil)
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

// PauseConnection pauses a connection
func (c *Client) PauseConnection(ctx context.Context, id string) (*Connection, error) {
	resp, err := c.Put(ctx, fmt.Sprintf("/2024-03-01/connections/%s/pause", id), []byte("{}"), nil)
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

// UnpauseConnection unpauses a connection
func (c *Client) UnpauseConnection(ctx context.Context, id string) (*Connection, error) {
	resp, err := c.Put(ctx, fmt.Sprintf("/2024-03-01/connections/%s/unpause", id), []byte("{}"), nil)
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

// ArchiveConnection archives a connection
func (c *Client) ArchiveConnection(ctx context.Context, id string) (*Connection, error) {
	resp, err := c.Put(ctx, fmt.Sprintf("/2024-03-01/connections/%s/archive", id), []byte("{}"), nil)
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

// UnarchiveConnection unarchives a connection
func (c *Client) UnarchiveConnection(ctx context.Context, id string) (*Connection, error) {
	resp, err := c.Put(ctx, fmt.Sprintf("/2024-03-01/connections/%s/unarchive", id), []byte("{}"), nil)
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

// CountConnections counts connections matching the given filters
func (c *Client) CountConnections(ctx context.Context, params map[string]string) (*ConnectionCountResponse, error) {
	queryParams := url.Values{}
	for k, v := range params {
		queryParams.Add(k, v)
	}

	resp, err := c.Get(ctx, "/2024-03-01/connections/count", queryParams.Encode(), nil)
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

// newRequest creates a new HTTP request (helper for DELETE)
func (c *Client) newRequest(ctx context.Context, method, path string, body []byte) (*http.Request, error) {
	u, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	u = c.BaseURL.ResolveReference(u)

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewBuffer(body)
	}

	return http.NewRequest(method, u.String(), bodyReader)
}
