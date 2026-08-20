package hookdeck

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// OutpostTenant represents a tenant — the end customer destinations belong to.
// Unlike most Hookdeck resources the ID is supplied by the operator rather than
// generated, which is why tenants are created with an idempotent upsert.
type OutpostTenant struct {
	ID                string            `json:"id"`
	DestinationsCount int               `json:"destinations_count"`
	Topics            []string          `json:"topics"`
	Metadata          map[string]string `json:"metadata"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

// OutpostTenantListResponse is the paginated response from listing tenants.
type OutpostTenantListResponse struct {
	Models     []OutpostTenant    `json:"models"`
	Pagination PaginationResponse `json:"pagination"`
	Count      int                `json:"count"`
}

// OutpostTenantUpsertRequest is the body for PUT /tenants/{id}. Metadata is the
// only writable field; the ID comes from the path.
type OutpostTenantUpsertRequest struct {
	Metadata map[string]string `json:"metadata,omitempty"`
}

// OutpostTenantToken is a short-lived JWT scoped to a single tenant.
type OutpostTenantToken struct {
	Token    string `json:"token"`
	TenantID string `json:"tenant_id"`
}

// OutpostTenantPortalURL is a redirect URL granting access to a tenant's portal.
type OutpostTenantPortalURL struct {
	RedirectURL string `json:"redirect_url"`
	TenantID    string `json:"tenant_id"`
}

// OutpostTenantListParams are the filters accepted by ListOutpostTenants.
type OutpostTenantListParams struct {
	IDs   []string
	Limit int
	Dir   string
	Next  string
	Prev  string
}

// ListOutpostTenants retrieves a page of tenants.
func (c *Client) ListOutpostTenants(ctx context.Context, params OutpostTenantListParams) (*OutpostTenantListResponse, error) {
	scalar := map[string]string{
		"dir":  params.Dir,
		"next": params.Next,
		"prev": params.Prev,
	}
	if params.Limit > 0 {
		scalar["limit"] = fmt.Sprintf("%d", params.Limit)
	}

	resp, err := c.Get(ctx, APIPathPrefix+"/tenants", outpostQuery(scalar, map[string][]string{
		"id": params.IDs,
	}), nil)
	if err != nil {
		return nil, err
	}

	var result OutpostTenantListResponse
	if _, err := postprocessJsonResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tenant list response: %w", err)
	}

	return &result, nil
}

// GetOutpostTenant retrieves a single tenant by ID.
func (c *Client) GetOutpostTenant(ctx context.Context, tenantID string) (*OutpostTenant, error) {
	path, err := apiPath("tenants", tenantID)
	if err != nil {
		return nil, err
	}

	resp, err := c.Get(ctx, path, "", nil)
	if err != nil {
		return nil, err
	}

	var tenant OutpostTenant
	if _, err := postprocessJsonResponse(resp, &tenant); err != nil {
		return nil, fmt.Errorf("failed to parse tenant response: %w", err)
	}

	return &tenant, nil
}

// UpsertOutpostTenant creates a tenant or updates its metadata. The API is
// idempotent, returning 201 on create and 200 on update.
func (c *Client) UpsertOutpostTenant(ctx context.Context, tenantID string, req *OutpostTenantUpsertRequest) (*OutpostTenant, error) {
	path, err := apiPath("tenants", tenantID)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tenant upsert request: %w", err)
	}

	resp, err := c.Put(ctx, path, data, nil)
	if err != nil {
		return nil, err
	}

	var tenant OutpostTenant
	if _, err := postprocessJsonResponse(resp, &tenant); err != nil {
		return nil, fmt.Errorf("failed to parse tenant response: %w", err)
	}

	return &tenant, nil
}

// DeleteOutpostTenant deletes a tenant and everything belonging to it.
func (c *Client) DeleteOutpostTenant(ctx context.Context, tenantID string) error {
	path, err := apiPath("tenants", tenantID)
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

// GetOutpostTenantToken mints a JWT scoped to the tenant. The token is a
// credential in its own right — it grants access to that tenant's data.
func (c *Client) GetOutpostTenantToken(ctx context.Context, tenantID string) (*OutpostTenantToken, error) {
	path, err := apiPath("tenants", tenantID, "token")
	if err != nil {
		return nil, err
	}

	resp, err := c.Get(ctx, path, "", nil)
	if err != nil {
		return nil, err
	}

	var token OutpostTenantToken
	if _, err := postprocessJsonResponse(resp, &token); err != nil {
		return nil, fmt.Errorf("failed to parse tenant token response: %w", err)
	}

	return &token, nil
}

// GetOutpostTenantPortalURL returns a redirect URL for the tenant's portal.
// theme is optional and accepts "light" or "dark".
func (c *Client) GetOutpostTenantPortalURL(ctx context.Context, tenantID, theme string) (*OutpostTenantPortalURL, error) {
	path, err := apiPath("tenants", tenantID, "portal")
	if err != nil {
		return nil, err
	}

	query := outpostQuery(map[string]string{"theme": theme}, nil)

	resp, err := c.Get(ctx, path, query, nil)
	if err != nil {
		return nil, err
	}

	var portal OutpostTenantPortalURL
	if _, err := postprocessJsonResponse(resp, &portal); err != nil {
		return nil, fmt.Errorf("failed to parse tenant portal response: %w", err)
	}

	return &portal, nil
}
