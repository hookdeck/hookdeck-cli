package hookdeck

import (
	"context"
	"encoding/json"
	"fmt"
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
	Type        string                 `json:"type,omitempty"`
	Description *string                `json:"description,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// SourceCreateRequest represents the request to create a source
type SourceCreateRequest struct {
	Name        string                 `json:"name"`
	Description *string                `json:"description,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// CreateSource creates a new source
func (c *Client) CreateSource(ctx context.Context, req *SourceCreateRequest) (*Source, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal source request: %w", err)
	}

	resp, err := c.Post(ctx, "/2024-03-01/sources", data, nil)
	if err != nil {
		return nil, err
	}

	var source Source
	_, err = postprocessJsonResponse(resp, &source)
	if err != nil {
		return nil, fmt.Errorf("failed to parse source response: %w", err)
	}

	return &source, nil
}
