package hookdeck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// Bulk operations: retry, cancel and replay applied to everything matching a
// filter, rather than to one record.
//
// Five families, all the same shape — POST {query} to start one, GET /plan to
// estimate what it would touch first, GET to list, GET /{id} to follow, and
// (with one exception) POST /{id}/cancel to stop it.
//
// The families do NOT accept the same filters, which is the trap here. An
// events family takes 23; ignored-events takes 3. A filter the family does not
// declare is ignored by the API, so the operation runs across everything the
// remaining filters match — an unfiltered bulk retry that reads as a filtered
// one, with real deliveries behind it. BulkFilters is the matrix that prevents
// that, and it is checked before anything is sent.

// Bulk operation families, spelled as the route spells them.
const (
	BulkEventsRetry        = "events_retry"
	BulkEventsCancel       = "events_cancel"
	BulkIgnoredEventsRetry = "ignored_events_retry"
	BulkRequestsRetry      = "requests_retry"
	BulkRequestsReplay     = "requests_replay"
)

// bulkRoutes maps a family onto its path segment.
var bulkRoutes = map[string]string{
	BulkEventsRetry:        "events/retry",
	BulkEventsCancel:       "events/cancel",
	BulkIgnoredEventsRetry: "ignored-events/retry",
	BulkRequestsRetry:      "requests/retry",
	BulkRequestsReplay:     "requests/replay",
}

// BulkFilters is the query filter each family declares, taken from the
// 2026-09-01 OpenAPI document. Pinned by TestBulkFiltersMatchTheSpec.
var BulkFilters = map[string][]string{
	BulkEventsRetry: eventBulkFilters,
	// The same collection, so the same filters.
	BulkEventsCancel: eventBulkFilters,
	// Far narrower than its siblings, and the reason this matrix exists.
	BulkIgnoredEventsRetry: {"cause", "webhook_id", "transformation_id"},
	BulkRequestsRetry:      requestBulkFilters,
	// The request filters plus target, which selects where to replay.
	BulkRequestsReplay: append(append([]string{}, requestBulkFilters...), "target"),
}

var eventBulkFilters = []string{
	"id", "status", "webhook_id", "destination_id", "delivery_group", "source_id",
	"attempts", "response_status", "successful_at", "created_at", "error_code",
	"cli_id", "last_attempt_at", "next_attempt_at", "search_term", "headers",
	"body", "parsed_query", "path", "cli_user_id", "issue_id", "event_data_id",
	"bulk_retry_id",
}

var requestBulkFilters = []string{
	"id", "status", "rejection_cause", "source_id", "verified", "search_term",
	"headers", "body", "parsed_query", "path", "ignored_count", "events_count",
	"cli_events_count", "created_at", "ingested_at", "bulk_retry_id",
}

// BulkFamilies returns the family names, sorted, for an enum or an error.
func BulkFamilies() []string {
	out := make([]string, 0, len(bulkRoutes))
	for k := range bulkRoutes {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// BulkCancellable reports whether a family's jobs can be stopped once started.
//
// events_cancel cannot: it is the one family the API gives no /{id}/cancel.
// Saying so is more useful than a 404.
func BulkCancellable(family string) bool {
	return family != BulkEventsCancel && bulkRoutes[family] != ""
}

// RejectUnsupportedBulkFilters reports the filters this family does not declare.
func RejectUnsupportedBulkFilters(family string, query map[string]interface{}) error {
	allowed, ok := BulkFilters[family]
	if !ok {
		return fmt.Errorf("unknown bulk operation %q; expected one of: %s",
			family, strings.Join(BulkFamilies(), ", "))
	}
	set := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		set[a] = true
	}

	var bad []string
	for k := range query {
		if !set[k] {
			bad = append(bad, k)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	sort.Strings(bad)
	sorted := append([]string{}, allowed...)
	sort.Strings(sorted)
	return fmt.Errorf(
		"%s is not a filter of the %s operation; it accepts: %s. "+
			"The API would ignore it and run across everything the remaining filters match",
		strings.Join(bad, ", "), family, strings.Join(sorted, ", "))
}

// BulkJob is a bulk operation, running or finished.
type BulkJob struct {
	ID             string      `json:"id"`
	TeamID         string      `json:"team_id,omitempty"`
	Type           string      `json:"type,omitempty"`
	Query          interface{} `json:"query,omitempty"`
	CreatedAt      string      `json:"created_at,omitempty"`
	UpdatedAt      string      `json:"updated_at,omitempty"`
	CancelledAt    *string     `json:"cancelled_at"`
	CompletedAt    *string     `json:"completed_at"`
	EstimatedCount int         `json:"estimated_count,omitempty"`
	CompletedCount int         `json:"completed_count,omitempty"`
	FailedCount    int         `json:"failed_count,omitempty"`
	InProgress     bool        `json:"in_progress,omitempty"`
	Progress       float64     `json:"progress,omitempty"`
}

// BulkPlan is what a bulk operation would touch, without touching it.
type BulkPlan struct {
	EstimatedBatch int     `json:"estimated_batch,omitempty"`
	EstimatedCount int     `json:"estimated_count"`
	Progress       float64 `json:"progress,omitempty"`
}

func bulkPath(family string, extra ...string) (string, error) {
	route, ok := bulkRoutes[family]
	if !ok {
		return "", fmt.Errorf("unknown bulk operation %q; expected one of: %s",
			family, strings.Join(BulkFamilies(), ", "))
	}
	segments := append([]string{"bulk"}, strings.Split(route, "/")...)
	segments = append(segments, extra...)
	return apiPath(segments...)
}

// encodeBulkQuery renders the query object the way the API reads it: bracket
// notation, query[status]=FAILED, the same convention the metrics routes use
// for filters[...] and date_range[...].
//
// It is NOT a JSON string. Sending one is accepted by any hand-written mock and
// rejected by the API with "query must be of type object" — which is how this
// was found, by an acceptance test rather than a unit test.
func encodeBulkQuery(query map[string]interface{}) string {
	values := url.Values{}
	for key, raw := range query {
		switch v := raw.(type) {
		case nil:
			continue
		case string:
			values.Set("query["+key+"]", v)
		case bool:
			values.Set("query["+key+"]", strconv.FormatBool(v))
		case float64:
			values.Set("query["+key+"]", strconv.FormatFloat(v, 'f', -1, 64))
		case []interface{}:
			for _, item := range v {
				values.Add("query["+key+"][]", fmt.Sprintf("%v", item))
			}
		case map[string]interface{}:
			// Nested objects — target[source_id], and the range filters'
			// operator forms such as created_at[gte].
			for inner, iv := range v {
				values.Set("query["+key+"]["+inner+"]", fmt.Sprintf("%v", iv))
			}
		default:
			values.Set("query["+key+"]", fmt.Sprintf("%v", v))
		}
	}
	return values.Encode()
}

// PlanBulk estimates what a bulk operation would match, running nothing.
func (c *Client) PlanBulk(ctx context.Context, family string, query map[string]interface{}) (*BulkPlan, error) {
	if err := RejectUnsupportedBulkFilters(family, query); err != nil {
		return nil, err
	}
	path, err := bulkPath(family, "plan")
	if err != nil {
		return nil, err
	}
	resp, err := c.Get(ctx, path, encodeBulkQuery(query), nil)
	if err != nil {
		return nil, err
	}
	var plan BulkPlan
	if _, err := postprocessJsonResponse(resp, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse bulk plan response: %w", err)
	}
	return &plan, nil
}

// CreateBulk starts a bulk operation.
func (c *Client) CreateBulk(ctx context.Context, family string, query map[string]interface{}) (*BulkJob, error) {
	if err := RejectUnsupportedBulkFilters(family, query); err != nil {
		return nil, err
	}
	path, err := bulkPath(family)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(map[string]interface{}{"query": query})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal bulk request: %w", err)
	}
	resp, err := c.Post(ctx, path, data, nil)
	if err != nil {
		return nil, err
	}
	var job BulkJob
	if _, err := postprocessJsonResponse(resp, &job); err != nil {
		return nil, fmt.Errorf("failed to parse bulk response: %w", err)
	}
	return &job, nil
}

// ListBulk returns a family's jobs.
func (c *Client) ListBulk(ctx context.Context, family string, params map[string]string) ([]BulkJob, error) {
	path, err := bulkPath(family)
	if err != nil {
		return nil, err
	}
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	resp, err := c.Get(ctx, path, values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var listed struct {
		Models []BulkJob `json:"models"`
	}
	if _, err := postprocessJsonResponse(resp, &listed); err != nil {
		return nil, fmt.Errorf("failed to parse bulk list response: %w", err)
	}
	return listed.Models, nil
}

// GetBulk returns one job.
func (c *Client) GetBulk(ctx context.Context, family, id string) (*BulkJob, error) {
	path, err := bulkPath(family, id)
	if err != nil {
		return nil, err
	}
	resp, err := c.Get(ctx, path, "", nil)
	if err != nil {
		return nil, err
	}
	var job BulkJob
	if _, err := postprocessJsonResponse(resp, &job); err != nil {
		return nil, fmt.Errorf("failed to parse bulk response: %w", err)
	}
	return &job, nil
}

// CancelBulk stops a pending or in-progress job.
func (c *Client) CancelBulk(ctx context.Context, family, id string) (*BulkJob, error) {
	if !BulkCancellable(family) {
		return nil, fmt.Errorf(
			"a %s operation cannot be cancelled once started: the API offers no cancel route for it",
			family)
	}
	path, err := bulkPath(family, id, "cancel")
	if err != nil {
		return nil, err
	}
	resp, err := c.Post(ctx, path, nil, nil)
	if err != nil {
		return nil, err
	}
	var job BulkJob
	if _, err := postprocessJsonResponse(resp, &job); err != nil {
		return nil, fmt.Errorf("failed to parse bulk response: %w", err)
	}
	return &job, nil
}
