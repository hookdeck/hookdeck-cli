package hookdeck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// IssueStatus represents the status of an issue.
type IssueStatus string

const (
	IssueStatusOpened       IssueStatus = "OPENED"
	IssueStatusIgnored      IssueStatus = "IGNORED"
	IssueStatusAcknowledged IssueStatus = "ACKNOWLEDGED"
	IssueStatusResolved     IssueStatus = "RESOLVED"
)

// IssueType represents the type of an issue.
type IssueType string

const (
	IssueTypeDelivery       IssueType = "delivery"
	IssueTypeTransformation IssueType = "transformation"
	IssueTypeBackpressure   IssueType = "backpressure"
)

// Issue represents a Hookdeck issue.
type Issue struct {
	ID              string                 `json:"id"`
	TeamID          string                 `json:"team_id"`
	Status          IssueStatus            `json:"status"`
	Type            IssueType              `json:"type"`
	OpenedAt        time.Time              `json:"opened_at"`
	FirstSeenAt     time.Time              `json:"first_seen_at"`
	LastSeenAt      time.Time              `json:"last_seen_at"`
	DismissedAt     *time.Time             `json:"dismissed_at,omitempty"`
	AggregationKeys map[string]interface{} `json:"aggregation_keys"`
	Reference       map[string]interface{} `json:"reference"`
	Data            map[string]interface{} `json:"data,omitempty"`
	UpdatedAt       time.Time              `json:"updated_at"`
	CreatedAt       time.Time              `json:"created_at"`
}

// IssueUpdateRequest is the request body for PUT /issues/{id}.
type IssueUpdateRequest struct {
	Status IssueStatus `json:"status"`
}

// IssueListResponse represents the response from listing issues.
type IssueListResponse struct {
	Models     []Issue            `json:"models"`
	Pagination PaginationResponse `json:"pagination"`
	Count      *int               `json:"count,omitempty"`
}

// IssueCountResponse represents the response from counting issues.
type IssueCountResponse struct {
	Count int `json:"count"`
}

// ListIssues retrieves issues with optional filters.
func (c *Client) ListIssues(ctx context.Context, params map[string]string) (*IssueListResponse, error) {
	queryParams := url.Values{}
	for k, v := range params {
		queryParams.Add(k, v)
	}

	resp, err := c.Get(ctx, APIPathPrefix+"/issues", queryParams.Encode(), nil)
	if err != nil {
		return nil, err
	}

	var result IssueListResponse
	_, err = postprocessJsonResponse(resp, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse issue list response: %w", err)
	}

	return &result, nil
}

// GetIssue retrieves a single issue by ID.
func (c *Client) GetIssue(ctx context.Context, id string) (*Issue, error) {
	resp, err := c.Get(ctx, APIPathPrefix+"/issues/"+id, "", nil)
	if err != nil {
		return nil, err
	}

	var issue Issue
	_, err = postprocessJsonResponse(resp, &issue)
	if err != nil {
		return nil, fmt.Errorf("failed to parse issue response: %w", err)
	}

	return &issue, nil
}

// UpdateIssue updates an issue's status.
func (c *Client) UpdateIssue(ctx context.Context, id string, req *IssueUpdateRequest) (*Issue, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal issue update request: %w", err)
	}

	resp, err := c.Put(ctx, APIPathPrefix+"/issues/"+id, data, nil)
	if err != nil {
		return nil, err
	}

	var issue Issue
	_, err = postprocessJsonResponse(resp, &issue)
	if err != nil {
		return nil, fmt.Errorf("failed to parse issue response: %w", err)
	}

	return &issue, nil
}

// DismissIssue dismisses an issue (DELETE /issues/{id}).
func (c *Client) DismissIssue(ctx context.Context, id string) (*Issue, error) {
	urlPath := APIPathPrefix + "/issues/" + id
	req, err := c.newRequest(ctx, "DELETE", urlPath, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.PerformRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var issue Issue
	_, err = postprocessJsonResponse(resp, &issue)
	if err != nil {
		return nil, fmt.Errorf("failed to parse issue response: %w", err)
	}

	return &issue, nil
}

// CountIssues counts issues matching the given filters.
func (c *Client) CountIssues(ctx context.Context, params map[string]string) (*IssueCountResponse, error) {
	queryParams := url.Values{}
	for k, v := range params {
		queryParams.Add(k, v)
	}

	resp, err := c.Get(ctx, APIPathPrefix+"/issues/count", queryParams.Encode(), nil)
	if err != nil {
		return nil, err
	}

	var result IssueCountResponse
	_, err = postprocessJsonResponse(resp, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse issue count response: %w", err)
	}

	return &result, nil
}
