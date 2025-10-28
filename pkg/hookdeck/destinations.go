package hookdeck

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

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

// DestinationCreateRequest represents the request to create a destination
type DestinationCreateRequest struct {
	Name        string                 `json:"name"`
	Description *string                `json:"description,omitempty"`
	URL         *string                `json:"url,omitempty"`
	CliPath     *string                `json:"cli_path,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// CreateDestination creates a new destination
func (c *Client) CreateDestination(ctx context.Context, req *DestinationCreateRequest) (*Destination, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal destination request: %w", err)
	}

	resp, err := c.Post(ctx, "/2024-03-01/destinations", data, nil)
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
