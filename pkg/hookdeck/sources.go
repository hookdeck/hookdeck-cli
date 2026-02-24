package hookdeck

import (
	"context"
	"encoding/json"
	"fmt"
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

// SourceCreateInput is the payload for a source when nested inside another request
// (e.g. ConnectionCreateRequest.Source). Single responsibility: inline source definition.
// Source has type and config.auth (same shape as standalone source create).
type SourceCreateInput struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Description *string                `json:"description,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// SourceCreateRequest is the request body for create and upsert (POST/PUT /sources).
// API requires name for both. Same shape as SourceCreateInput but for direct /sources endpoints.
type SourceCreateRequest struct {
	Name        string                 `json:"name"`
	Description *string                `json:"description,omitempty"`
	Type        string                 `json:"type,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// SourceUpdateRequest is the request body for update (PUT /sources/{id}).
// API has no required fields; only include fields that are being updated.
type SourceUpdateRequest struct {
	Name        string                 `json:"name,omitempty"`
	Description *string                `json:"description,omitempty"`
	Type        string                 `json:"type,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// SourceListResponse represents the response from listing sources
type SourceListResponse struct {
	Models     []Source             `json:"models"`
	Pagination PaginationResponse   `json:"pagination"`
}

// SourceCountResponse represents the response from counting sources
type SourceCountResponse struct {
	Count int `json:"count"`
}

// ListSources retrieves a list of sources with optional filters
func (c *Client) ListSources(ctx context.Context, params map[string]string) (*SourceListResponse, error) {
	queryParams := url.Values{}
	for k, v := range params {
		queryParams.Add(k, v)
	}

	resp, err := c.Get(ctx, APIPathPrefix+"/sources", queryParams.Encode(), nil)
	if err != nil {
		return nil, err
	}

	var result SourceListResponse
	_, err = postprocessJsonResponse(resp, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse source list response: %w", err)
	}

	return &result, nil
}

// GetSource retrieves a single source by ID
func (c *Client) GetSource(ctx context.Context, id string, params map[string]string) (*Source, error) {
	queryStr := ""
	if len(params) > 0 {
		queryParams := url.Values{}
		for k, v := range params {
			queryParams.Add(k, v)
		}
		queryStr = queryParams.Encode()
	}

	resp, err := c.Get(ctx, APIPathPrefix+"/sources/"+id, queryStr, nil)
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

// CreateSource creates a new source
func (c *Client) CreateSource(ctx context.Context, req *SourceCreateRequest) (*Source, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal source request: %w", err)
	}

	resp, err := c.Post(ctx, APIPathPrefix+"/sources", data, nil)
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

// UpsertSource creates or updates a source by name
func (c *Client) UpsertSource(ctx context.Context, req *SourceCreateRequest) (*Source, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal source upsert request: %w", err)
	}

	resp, err := c.Put(ctx, APIPathPrefix+"/sources", data, nil)
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

// UpdateSource updates an existing source by ID
func (c *Client) UpdateSource(ctx context.Context, id string, req *SourceUpdateRequest) (*Source, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal source update request: %w", err)
	}

	resp, err := c.Put(ctx, APIPathPrefix+"/sources/"+id, data, nil)
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

// DeleteSource deletes a source
func (c *Client) DeleteSource(ctx context.Context, id string) error {
	urlPath := APIPathPrefix + "/sources/" + id
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

// EnableSource enables a source
func (c *Client) EnableSource(ctx context.Context, id string) (*Source, error) {
	resp, err := c.Put(ctx, APIPathPrefix+"/sources/"+id+"/enable", []byte("{}"), nil)
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

// DisableSource disables a source
func (c *Client) DisableSource(ctx context.Context, id string) (*Source, error) {
	resp, err := c.Put(ctx, APIPathPrefix+"/sources/"+id+"/disable", []byte("{}"), nil)
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

// CountSources counts sources matching the given filters
func (c *Client) CountSources(ctx context.Context, params map[string]string) (*SourceCountResponse, error) {
	queryParams := url.Values{}
	for k, v := range params {
		queryParams.Add(k, v)
	}

	resp, err := c.Get(ctx, APIPathPrefix+"/sources/count", queryParams.Encode(), nil)
	if err != nil {
		return nil, err
	}

	var result SourceCountResponse
	_, err = postprocessJsonResponse(resp, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse source count response: %w", err)
	}

	return &result, nil
}
