package hookdeck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// Transformation represents a Hookdeck transformation
type Transformation struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Code      string                 `json:"code"`
	Env       map[string]string      `json:"env,omitempty"`
	UpdatedAt time.Time              `json:"updated_at"`
	CreatedAt time.Time              `json:"created_at"`
}

// TransformationCreateRequest is the request body for create and upsert (POST/PUT /transformations).
// API requires name and code for both.
type TransformationCreateRequest struct {
	Name string            `json:"name"`
	Code string            `json:"code"`
	Env  map[string]string `json:"env,omitempty"`
}

// TransformationUpdateRequest is the request body for update (PUT /transformations/{id}).
// API supports partial update; only include fields that are being updated.
type TransformationUpdateRequest struct {
	Name string            `json:"name,omitempty"`
	Code string            `json:"code,omitempty"`
	Env  map[string]string `json:"env,omitempty"`
}

// TransformationListResponse represents the response from listing transformations
type TransformationListResponse struct {
	Models     []Transformation   `json:"models"`
	Pagination PaginationResponse `json:"pagination"`
}

// TransformationCountResponse represents the response from counting transformations
type TransformationCountResponse struct {
	Count int `json:"count"`
}

// TransformationRunRequest is the request body for PUT /transformations/run.
// Either Code or TransformationID must be set. Request.Headers is required (can be empty object).
type TransformationRunRequest struct {
	Code             string                    `json:"code,omitempty"`
	TransformationID string                    `json:"transformation_id,omitempty"`
	WebhookID        string                    `json:"webhook_id,omitempty"`
	Env              map[string]string         `json:"env,omitempty"`
	Request          *TransformationRunRequestInput `json:"request,omitempty"`
}

// TransformationRunRequestInput is the "request" object for run (required headers; optional body, path, query).
type TransformationRunRequestInput struct {
	Headers    map[string]string      `json:"headers"`
	Body       interface{}            `json:"body,omitempty"`
	Path       string                 `json:"path,omitempty"`
	Query      string                 `json:"query,omitempty"`
	ParsedQuery map[string]interface{} `json:"parsed_query,omitempty"`
}

// TransformationRunResponse is the response from PUT /transformations/run
type TransformationRunResponse struct {
	Result interface{} `json:"result,omitempty"`
}

// TransformationExecution represents a single transformation execution
type TransformationExecution struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	// Additional fields may be present from API
}

// TransformationExecutionListResponse represents the response from listing executions
type TransformationExecutionListResponse struct {
	Models     []TransformationExecution `json:"models"`
	Pagination PaginationResponse        `json:"pagination"`
}

// ListTransformations retrieves a list of transformations with optional filters
func (c *Client) ListTransformations(ctx context.Context, params map[string]string) (*TransformationListResponse, error) {
	queryParams := url.Values{}
	for k, v := range params {
		queryParams.Add(k, v)
	}

	resp, err := c.Get(ctx, APIPathPrefix+"/transformations", queryParams.Encode(), nil)
	if err != nil {
		return nil, err
	}

	var result TransformationListResponse
	_, err = postprocessJsonResponse(resp, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transformation list response: %w", err)
	}

	return &result, nil
}

// GetTransformation retrieves a single transformation by ID
func (c *Client) GetTransformation(ctx context.Context, id string) (*Transformation, error) {
	resp, err := c.Get(ctx, APIPathPrefix+"/transformations/"+id, "", nil)
	if err != nil {
		return nil, err
	}

	var t Transformation
	_, err = postprocessJsonResponse(resp, &t)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transformation response: %w", err)
	}

	return &t, nil
}

// CreateTransformation creates a new transformation
func (c *Client) CreateTransformation(ctx context.Context, req *TransformationCreateRequest) (*Transformation, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transformation request: %w", err)
	}

	resp, err := c.Post(ctx, APIPathPrefix+"/transformations", data, nil)
	if err != nil {
		return nil, err
	}

	var t Transformation
	_, err = postprocessJsonResponse(resp, &t)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transformation response: %w", err)
	}

	return &t, nil
}

// UpsertTransformation creates or updates a transformation by name
func (c *Client) UpsertTransformation(ctx context.Context, req *TransformationCreateRequest) (*Transformation, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transformation upsert request: %w", err)
	}

	resp, err := c.Put(ctx, APIPathPrefix+"/transformations", data, nil)
	if err != nil {
		return nil, err
	}

	var t Transformation
	_, err = postprocessJsonResponse(resp, &t)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transformation response: %w", err)
	}

	return &t, nil
}

// UpdateTransformation updates an existing transformation by ID
func (c *Client) UpdateTransformation(ctx context.Context, id string, req *TransformationUpdateRequest) (*Transformation, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transformation update request: %w", err)
	}

	resp, err := c.Put(ctx, APIPathPrefix+"/transformations/"+id, data, nil)
	if err != nil {
		return nil, err
	}

	var t Transformation
	_, err = postprocessJsonResponse(resp, &t)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transformation response: %w", err)
	}

	return &t, nil
}

// DeleteTransformation deletes a transformation
func (c *Client) DeleteTransformation(ctx context.Context, id string) error {
	urlPath := APIPathPrefix + "/transformations/" + id
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

// CountTransformations counts transformations matching the given filters
func (c *Client) CountTransformations(ctx context.Context, params map[string]string) (*TransformationCountResponse, error) {
	queryParams := url.Values{}
	for k, v := range params {
		queryParams.Add(k, v)
	}

	resp, err := c.Get(ctx, APIPathPrefix+"/transformations/count", queryParams.Encode(), nil)
	if err != nil {
		return nil, err
	}

	var result TransformationCountResponse
	_, err = postprocessJsonResponse(resp, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transformation count response: %w", err)
	}

	return &result, nil
}

// RunTransformation runs transformation code (test run) via PUT /transformations/run
func (c *Client) RunTransformation(ctx context.Context, req *TransformationRunRequest) (*TransformationRunResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transformation run request: %w", err)
	}

	resp, err := c.Put(ctx, APIPathPrefix+"/transformations/run", data, nil)
	if err != nil {
		return nil, err
	}

	var result TransformationRunResponse
	_, err = postprocessJsonResponse(resp, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transformation run response: %w", err)
	}

	return &result, nil
}

// ListTransformationExecutions lists executions for a transformation
func (c *Client) ListTransformationExecutions(ctx context.Context, transformationID string, params map[string]string) (*TransformationExecutionListResponse, error) {
	queryParams := url.Values{}
	for k, v := range params {
		queryParams.Add(k, v)
	}

	resp, err := c.Get(ctx, APIPathPrefix+"/transformations/"+transformationID+"/executions", queryParams.Encode(), nil)
	if err != nil {
		return nil, err
	}

	var result TransformationExecutionListResponse
	_, err = postprocessJsonResponse(resp, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transformation executions list response: %w", err)
	}

	return &result, nil
}

// GetTransformationExecution retrieves a single execution by transformation ID and execution ID
func (c *Client) GetTransformationExecution(ctx context.Context, transformationID, executionID string) (*TransformationExecution, error) {
	resp, err := c.Get(ctx, APIPathPrefix+"/transformations/"+transformationID+"/executions/"+executionID, "", nil)
	if err != nil {
		return nil, err
	}

	var exec TransformationExecution
	_, err = postprocessJsonResponse(resp, &exec)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transformation execution response: %w", err)
	}

	return &exec, nil
}
