package hookdeck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// Destination represents a Hookdeck destination
type Destination struct {
	ID          string                 `json:"id"`
	TeamID      string                 `json:"team_id"`
	Name        string                 `json:"name"`
	Description *string                `json:"description"`
	Type        string                 `json:"type"`
	Config      map[string]interface{} `json:"config"`
	DisabledAt  *time.Time             `json:"disabled_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	CreatedAt   time.Time              `json:"created_at"`
}

// GetCLIPath returns the CLI path from config for CLI-type destinations
// For CLI destinations, the path is stored in config.path according to the OpenAPI spec
func (d *Destination) GetCLIPath() *string {
	if d.Type != "CLI" || d.Config == nil {
		return nil
	}

	if path, ok := d.Config["path"].(string); ok {
		return &path
	}

	return nil
}

// GetHTTPURL returns the HTTP URL from config for HTTP-type destinations
// For HTTP destinations, the URL is stored in config.url according to the OpenAPI spec
func (d *Destination) GetHTTPURL() *string {
	if d.Type != "HTTP" || d.Config == nil {
		return nil
	}

	if url, ok := d.Config["url"].(string); ok {
		return &url
	}

	return nil
}

// SetCLIPath sets the CLI path in config for CLI-type destinations
func (d *Destination) SetCLIPath(path string) {
	if d.Type == "CLI" {
		if d.Config == nil {
			d.Config = make(map[string]interface{})
		}
		d.Config["path"] = path
	}
}

// ListDestinations retrieves a list of destinations with optional filters
func (c *Client) ListDestinations(ctx context.Context, params map[string]string) (*DestinationListResponse, error) {
	queryParams := url.Values{}
	for k, v := range params {
		queryParams.Add(k, v)
	}

	resp, err := c.Get(ctx, APIPathPrefix+"/destinations", queryParams.Encode(), nil)
	if err != nil {
		return nil, err
	}

	var result DestinationListResponse
	_, err = postprocessJsonResponse(resp, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse destination list response: %w", err)
	}

	return &result, nil
}

// GetDestination retrieves a single destination by ID
func (c *Client) GetDestination(ctx context.Context, id string, params map[string]string) (*Destination, error) {
	queryStr := ""
	if len(params) > 0 {
		queryParams := url.Values{}
		for k, v := range params {
			queryParams.Add(k, v)
		}
		queryStr = queryParams.Encode()
	}

	resp, err := c.Get(ctx, APIPathPrefix+"/destinations/"+id, queryStr, nil)
	if err != nil {
		return nil, err
	}

	var destination Destination
	_, err = postprocessJsonResponse(resp, &destination)
	if err != nil {
		return nil, fmt.Errorf("failed to parse destination response: %w", err)
	}

	return &destination, nil
}

// CreateDestination creates a new destination
func (c *Client) CreateDestination(ctx context.Context, req *DestinationCreateRequest) (*Destination, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal destination request: %w", err)
	}

	resp, err := c.Post(ctx, APIPathPrefix+"/destinations", data, nil)
	if err != nil {
		return nil, err
	}

	var destination Destination
	_, err = postprocessJsonResponse(resp, &destination)
	if err != nil {
		return nil, fmt.Errorf("failed to parse destination response: %w", err)
	}

	return &destination, nil
}

// UpsertDestination creates or updates a destination by name
func (c *Client) UpsertDestination(ctx context.Context, req *DestinationCreateRequest) (*Destination, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal destination upsert request: %w", err)
	}

	resp, err := c.Put(ctx, APIPathPrefix+"/destinations", data, nil)
	if err != nil {
		return nil, err
	}

	var destination Destination
	_, err = postprocessJsonResponse(resp, &destination)
	if err != nil {
		return nil, fmt.Errorf("failed to parse destination response: %w", err)
	}

	return &destination, nil
}

// UpdateDestination updates an existing destination by ID
func (c *Client) UpdateDestination(ctx context.Context, id string, req *DestinationUpdateRequest) (*Destination, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal destination update request: %w", err)
	}

	resp, err := c.Put(ctx, APIPathPrefix+"/destinations/"+id, data, nil)
	if err != nil {
		return nil, err
	}

	var destination Destination
	_, err = postprocessJsonResponse(resp, &destination)
	if err != nil {
		return nil, fmt.Errorf("failed to parse destination response: %w", err)
	}

	return &destination, nil
}

// DeleteDestination deletes a destination
func (c *Client) DeleteDestination(ctx context.Context, id string) error {
	urlPath := APIPathPrefix + "/destinations/" + id
	req, err := c.newRequest(ctx, "DELETE", urlPath, nil)
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

// EnableDestination enables a destination
func (c *Client) EnableDestination(ctx context.Context, id string) (*Destination, error) {
	resp, err := c.Put(ctx, APIPathPrefix+"/destinations/"+id+"/enable", []byte("{}"), nil)
	if err != nil {
		return nil, err
	}

	var destination Destination
	_, err = postprocessJsonResponse(resp, &destination)
	if err != nil {
		return nil, fmt.Errorf("failed to parse destination response: %w", err)
	}

	return &destination, nil
}

// DisableDestination disables a destination
func (c *Client) DisableDestination(ctx context.Context, id string) (*Destination, error) {
	resp, err := c.Put(ctx, APIPathPrefix+"/destinations/"+id+"/disable", []byte("{}"), nil)
	if err != nil {
		return nil, err
	}

	var destination Destination
	_, err = postprocessJsonResponse(resp, &destination)
	if err != nil {
		return nil, fmt.Errorf("failed to parse destination response: %w", err)
	}

	return &destination, nil
}

// CountDestinations counts destinations matching the given filters
func (c *Client) CountDestinations(ctx context.Context, params map[string]string) (*DestinationCountResponse, error) {
	queryParams := url.Values{}
	for k, v := range params {
		queryParams.Add(k, v)
	}

	resp, err := c.Get(ctx, APIPathPrefix+"/destinations/count", queryParams.Encode(), nil)
	if err != nil {
		return nil, err
	}

	var result DestinationCountResponse
	_, err = postprocessJsonResponse(resp, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse destination count response: %w", err)
	}

	return &result, nil
}

// DestinationCreateInput represents input for creating a destination inline
type DestinationCreateInput struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Description *string                `json:"description,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// DestinationCreateRequest is the request body for create and upsert (POST/PUT /destinations).
// API requires name. Type and Config are used for HTTP/CLI/MOCK_API destinations.
type DestinationCreateRequest struct {
	Name        string                 `json:"name"`
	Description *string                `json:"description,omitempty"`
	Type        string                 `json:"type,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// DestinationUpdateRequest is the request body for update (PUT /destinations/{id}).
// API has no required fields; only include fields that are being updated.
type DestinationUpdateRequest struct {
	Name        string                 `json:"name,omitempty"`
	Description *string                `json:"description,omitempty"`
	Type        string                 `json:"type,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// DestinationListResponse represents the response from listing destinations
type DestinationListResponse struct {
	Models     []Destination        `json:"models"`
	Pagination PaginationResponse   `json:"pagination"`
}

// DestinationCountResponse represents the response from counting destinations
type DestinationCountResponse struct {
	Count int `json:"count"`
}
