package hookdeck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Transformation represents a Hookdeck transformation
type Transformation struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Code      string            `json:"code"`
	Env       map[string]string `json:"env,omitempty"`
	UpdatedAt time.Time         `json:"updated_at"`
	CreatedAt time.Time         `json:"created_at"`
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
	Code             string                         `json:"code,omitempty"`
	TransformationID string                         `json:"transformation_id,omitempty"`
	WebhookID        string                         `json:"webhook_id,omitempty"`
	Env              map[string]string              `json:"env,omitempty"`
	Request          *TransformationRunRequestInput `json:"request,omitempty"`
}

// TransformationRunRequestInput is the "request" object for run (required headers; optional body, path, query).
type TransformationRunRequestInput struct {
	Headers     map[string]string      `json:"headers"`
	Body        interface{}            `json:"body,omitempty"`
	Path        string                 `json:"path,omitempty"`
	Query       string                 `json:"query,omitempty"`
	ParsedQuery map[string]interface{} `json:"parsed_query,omitempty"`
}

// TransformationRunResponse is the response from PUT /transformations/run.
// Matches OpenAPI schema TransformationExecutorOutput.
type TransformationRunResponse struct {
	RequestID        string                         `json:"request_id,omitempty"`
	TransformationID string                         `json:"transformation_id,omitempty"`
	ExecutionID      string                         `json:"execution_id,omitempty"`
	Request          *TransformationRunRequestInput `json:"request,omitempty"`

	// LogLevel is how the run ended, and the only signal that it failed: the
	// endpoint answers 200 for a throwing handler, a syntax error and a clean
	// run alike. "fatal" and "error" mean the code did not complete.
	//
	// It was omitted from this struct, so both surfaces reported success for a
	// transformation that threw — the CLI printed "✔ Transformation run
	// completed" and exited 0.
	LogLevel string `json:"log_level,omitempty"`

	// Console is everything the code printed, and where the failure reason
	// lives. A throwing handler answers with no Request at all and the error
	// only here:
	//
	//   {"log_level":"fatal","console":[{"type":"error","message":"Error: ..."}]}
	Console []TransformationConsoleLine `json:"console,omitempty"`
}

// TransformationConsoleLine is one line the transformation code emitted.
type TransformationConsoleLine struct {
	Type    string `json:"type"` // error, log, warn, info, debug
	Message string `json:"message"`
}

// Failed reports whether the run did not complete.
//
// Only "fatal" means that. log_level is the highest severity the run logged,
// not a completion flag — a handler that calls console.error and then returns a
// transformed request reports "error" and succeeded. Treating that as a failure
// discarded the result the caller asked for, which is the same shape of wrong
// answer this type was extended to prevent, inverted.
//
// Verified against the live API:
//
//	clean run                  log_level=info   request present
//	console.warn then returns  log_level=warn   request present
//	console.error then returns log_level=error  request present
//	handler returns nothing    log_level=fatal  no request
//	handler throws             log_level=fatal  no request
//
// A missing request is checked too: the two failing cases have no request, so a
// run that produced one completed however loudly it complained on the way.
func (r *TransformationRunResponse) Failed() bool {
	if r == nil {
		return false
	}
	return r.LogLevel == "fatal" || r.Request == nil
}

// ConsoleText renders the console output as lines, for an error message.
func (r *TransformationRunResponse) ConsoleText() string {
	if r == nil || len(r.Console) == 0 {
		return ""
	}
	lines := make([]string, 0, len(r.Console))
	for _, line := range r.Console {
		lines = append(lines, fmt.Sprintf("[%s] %s", line.Type, line.Message))
	}
	return strings.Join(lines, "\n")
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
	path, err := apiPath("transformations", id)
	if err != nil {
		return nil, err
	}

	resp, err := c.Get(ctx, path, "", nil)
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
	path, err := apiPath("transformations", id)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transformation update request: %w", err)
	}

	resp, err := c.Put(ctx, path, data, nil)
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
	path, err := apiPath("transformations", id)
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
	path, err := apiPath("transformations", transformationID, "executions")
	if err != nil {
		return nil, err
	}

	queryParams := url.Values{}
	for k, v := range params {
		queryParams.Add(k, v)
	}

	resp, err := c.Get(ctx, path, queryParams.Encode(), nil)
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
	path, err := apiPath("transformations", transformationID, "executions", executionID)
	if err != nil {
		return nil, err
	}

	resp, err := c.Get(ctx, path, "", nil)
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
