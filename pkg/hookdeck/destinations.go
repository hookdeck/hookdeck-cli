package hookdeck

import (
	"context"
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

// GetDestination retrieves a single destination by ID
func (c *Client) GetDestination(ctx context.Context, id string, params map[string]string) (*Destination, error) {
	queryParams := url.Values{}
	for k, v := range params {
		queryParams.Add(k, v)
	}

	resp, err := c.Get(ctx, APIPathPrefix+"/destinations/"+id, queryParams.Encode(), nil)
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

// DestinationCreateInput represents input for creating a destination inline
type DestinationCreateInput struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Description *string                `json:"description,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// DestinationCreateRequest represents the request to create a destination
type DestinationCreateRequest struct {
	Name        string                 `json:"name"`
	Description *string                `json:"description,omitempty"`
	URL         *string                `json:"url,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}
