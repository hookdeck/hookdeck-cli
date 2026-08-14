package hookdeck

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// OutpostDestination represents a delivery destination belonging to a tenant.
//
// Config and Credentials are type-specific, so they stay untyped here and are
// validated against the schemas returned by ListOutpostDestinationTypes rather
// than against hand-written per-type structs.
type OutpostDestination struct {
	ID               string                 `json:"id"`
	Type             string                 `json:"type"`
	Topics           OutpostTopics          `json:"topics"`
	Config           map[string]interface{} `json:"config"`
	Credentials      map[string]interface{} `json:"credentials"`
	Filter           map[string]interface{} `json:"filter,omitempty"`
	DeliveryMetadata map[string]string      `json:"delivery_metadata,omitempty"`
	Metadata         map[string]string      `json:"metadata,omitempty"`
	Target           string                 `json:"target,omitempty"`
	TargetURL        string                 `json:"target_url,omitempty"`
	DisabledAt       *time.Time             `json:"disabled_at"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

// Disabled reports whether the destination is currently disabled.
func (d *OutpostDestination) Disabled() bool {
	return d != nil && d.DisabledAt != nil
}

// OutpostDestinationCreateRequest is the body for creating a destination.
// Type and Config are required; the rest depend on the destination type.
type OutpostDestinationCreateRequest struct {
	Type        string                 `json:"type"`
	Topics      OutpostTopics          `json:"topics,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Credentials map[string]interface{} `json:"credentials,omitempty"`
	Filter      map[string]interface{} `json:"filter,omitempty"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

// OutpostDestinationUpdateRequest is the body for updating a destination.
//
// The endpoint applies JSON merge-patch semantics, so omitted fields are left
// alone — hence omitempty on everything. Filter is the exception: the API
// replaces it wholesale rather than merging into it.
type OutpostDestinationUpdateRequest struct {
	Topics      OutpostTopics          `json:"topics,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Credentials map[string]interface{} `json:"credentials,omitempty"`
	Filter      map[string]interface{} `json:"filter,omitempty"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

// ListOutpostDestinations retrieves a tenant's destinations, optionally filtered
// by type and topic.
//
// This endpoint is not paginated: it returns a bare JSON array rather than the
// {models, pagination} envelope used elsewhere in this package.
func (c *Client) ListOutpostDestinations(ctx context.Context, tenantID string, types, topics []string) ([]OutpostDestination, error) {
	query := outpostQuery(nil, map[string][]string{
		"type":   types,
		"topics": topics,
	})

	resp, err := c.Get(ctx, outpostPath("tenants", tenantID, "destinations"), query, nil)
	if err != nil {
		return nil, err
	}

	var destinations []OutpostDestination
	if _, err := postprocessJsonResponse(resp, &destinations); err != nil {
		return nil, fmt.Errorf("failed to parse destination list response: %w", err)
	}

	return destinations, nil
}

// GetOutpostDestination retrieves a single destination.
func (c *Client) GetOutpostDestination(ctx context.Context, tenantID, destinationID string) (*OutpostDestination, error) {
	resp, err := c.Get(ctx, outpostPath("tenants", tenantID, "destinations", destinationID), "", nil)
	if err != nil {
		return nil, err
	}

	var destination OutpostDestination
	if _, err := postprocessJsonResponse(resp, &destination); err != nil {
		return nil, fmt.Errorf("failed to parse destination response: %w", err)
	}

	return &destination, nil
}

// CreateOutpostDestination creates a destination for a tenant.
func (c *Client) CreateOutpostDestination(ctx context.Context, tenantID string, req *OutpostDestinationCreateRequest) (*OutpostDestination, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal destination create request: %w", err)
	}

	resp, err := c.Post(ctx, outpostPath("tenants", tenantID, "destinations"), data, nil)
	if err != nil {
		return nil, err
	}

	var destination OutpostDestination
	if _, err := postprocessJsonResponse(resp, &destination); err != nil {
		return nil, fmt.Errorf("failed to parse destination response: %w", err)
	}

	return &destination, nil
}

// UpdateOutpostDestination applies a partial update to a destination.
func (c *Client) UpdateOutpostDestination(ctx context.Context, tenantID, destinationID string, req *OutpostDestinationUpdateRequest) (*OutpostDestination, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal destination update request: %w", err)
	}

	// The API uses PATCH here rather than PUT, so this goes through newRequest
	// instead of the Put helper.
	httpReq, err := c.newRequest(ctx, "PATCH", outpostPath("tenants", tenantID, "destinations", destinationID), data)
	if err != nil {
		return nil, err
	}

	resp, err := c.PerformRequest(ctx, httpReq)
	if err != nil {
		return nil, err
	}

	var destination OutpostDestination
	if _, err := postprocessJsonResponse(resp, &destination); err != nil {
		return nil, fmt.Errorf("failed to parse destination response: %w", err)
	}

	return &destination, nil
}

// DeleteOutpostDestination deletes a destination.
func (c *Client) DeleteOutpostDestination(ctx context.Context, tenantID, destinationID string) error {
	req, err := c.newRequest(ctx, "DELETE", outpostPath("tenants", tenantID, "destinations", destinationID), nil)
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

// EnableOutpostDestination re-enables a disabled destination.
func (c *Client) EnableOutpostDestination(ctx context.Context, tenantID, destinationID string) (*OutpostDestination, error) {
	return c.setOutpostDestinationEnabled(ctx, tenantID, destinationID, "enable")
}

// DisableOutpostDestination stops delivery to a destination without deleting it.
func (c *Client) DisableOutpostDestination(ctx context.Context, tenantID, destinationID string) (*OutpostDestination, error) {
	return c.setOutpostDestinationEnabled(ctx, tenantID, destinationID, "disable")
}

func (c *Client) setOutpostDestinationEnabled(ctx context.Context, tenantID, destinationID, action string) (*OutpostDestination, error) {
	resp, err := c.Put(ctx, outpostPath("tenants", tenantID, "destinations", destinationID, action), []byte("{}"), nil)
	if err != nil {
		return nil, err
	}

	var destination OutpostDestination
	if _, err := postprocessJsonResponse(resp, &destination); err != nil {
		return nil, fmt.Errorf("failed to parse destination response: %w", err)
	}

	return &destination, nil
}
