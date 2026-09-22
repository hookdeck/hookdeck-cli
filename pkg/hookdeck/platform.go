package hookdeck

import (
	"context"
	"encoding/json"
	"fmt"
)

// The platform API: the organization you are in, and the projects inside it.
//
// These arrived in the 2026-09-01 API. Before it, a project could only be
// listed and switched to; creating or renaming one meant the dashboard.
//
// Every organization route is /organizations/current — the API offers no way to
// name another, so there is no organization argument anywhere here.
//
// All of these are account-level, so every call goes through
// withoutProjectScope: ProjectID is sent as X-Team-ID / X-Project-ID, and these
// routes reject a project-scoped request with a bare 401 that reads as a bad
// credential rather than a wrong scope.

// Organization is the organization the current credential belongs to.
type Organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// OrganizationUpdateRequest is the body of PUT /organizations/current. Name is
// the only field the endpoint accepts.
type OrganizationUpdateRequest struct {
	Name *string `json:"name,omitempty"`
}

// ProjectCreateRequest is the body of POST /projects.
//
// The endpoint declares no required fields, so an empty body is a valid call.
// Callers should still insist on a name and a type: a nameless project of
// unspecified kind is not something anyone means to create.
type ProjectCreateRequest struct {
	Name           *string `json:"name,omitempty"`
	OrganizationID *string `json:"organization_id,omitempty"`
	Private        *bool   `json:"private,omitempty"`
	Type           *string `json:"type,omitempty"`
}

// ProjectUpdateRequest is the body of PUT /projects/{id}.
type ProjectUpdateRequest struct {
	Name                *string                `json:"name,omitempty"`
	Private             *bool                  `json:"private,omitempty"`
	Domain              *string                `json:"domain,omitempty"`
	HeadersPrefix       *string                `json:"headers_prefix,omitempty"`
	Context             map[string]interface{} `json:"context,omitempty"`
	NotificationMethods []string               `json:"notification_methods,omitempty"`
	WebhookTopics       []string               `json:"webhook_topics,omitempty"`
	WebhookSourceID     *string                `json:"webhook_source_id,omitempty"`
}

// CustomDomain is one custom hostname configured for a project.
type CustomDomain struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	Status   string `json:"status,omitempty"`
}

// GetOrganization returns the organization the credential belongs to.
func (c *Client) GetOrganization(ctx context.Context) (*Organization, error) {
	resp, err := c.withoutProjectScope().Get(ctx, APIPathPrefix+"/organizations/current", "", nil)
	if err != nil {
		return nil, err
	}
	var org Organization
	if _, err := postprocessJsonResponse(resp, &org); err != nil {
		return nil, fmt.Errorf("failed to parse organization response: %w", err)
	}
	return &org, nil
}

// UpdateOrganization renames the current organization.
func (c *Client) UpdateOrganization(ctx context.Context, req *OrganizationUpdateRequest) (*Organization, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal organization request: %w", err)
	}
	resp, err := c.withoutProjectScope().Put(ctx, APIPathPrefix+"/organizations/current", data, nil)
	if err != nil {
		return nil, err
	}
	var org Organization
	if _, err := postprocessJsonResponse(resp, &org); err != nil {
		return nil, fmt.Errorf("failed to parse organization response: %w", err)
	}
	return &org, nil
}

// GetProject returns one project by id.
func (c *Client) GetProject(ctx context.Context, id string) (*Project, error) {
	path, err := apiPath("projects", id)
	if err != nil {
		return nil, err
	}
	resp, err := c.withoutProjectScope().Get(ctx, path, "", nil)
	if err != nil {
		return nil, err
	}
	var project Project
	if _, err := postprocessJsonResponse(resp, &project); err != nil {
		return nil, fmt.Errorf("failed to parse project response: %w", err)
	}
	return &project, nil
}

// CreateProject creates a project in the current organization.
func (c *Client) CreateProject(ctx context.Context, req *ProjectCreateRequest) (*Project, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal project request: %w", err)
	}
	resp, err := c.withoutProjectScope().Post(ctx, APIPathPrefix+"/projects", data, nil)
	if err != nil {
		return nil, err
	}
	var project Project
	if _, err := postprocessJsonResponse(resp, &project); err != nil {
		return nil, fmt.Errorf("failed to parse project response: %w", err)
	}
	return &project, nil
}

// UpdateProject changes a project's settings.
func (c *Client) UpdateProject(ctx context.Context, id string, req *ProjectUpdateRequest) (*Project, error) {
	path, err := apiPath("projects", id)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal project request: %w", err)
	}
	resp, err := c.withoutProjectScope().Put(ctx, path, data, nil)
	if err != nil {
		return nil, err
	}
	var project Project
	if _, err := postprocessJsonResponse(resp, &project); err != nil {
		return nil, fmt.Errorf("failed to parse project response: %w", err)
	}
	return &project, nil
}

// DeleteProject removes a project. Everything in it goes with it.
func (c *Client) DeleteProject(ctx context.Context, id string) error {
	path, err := apiPath("projects", id)
	if err != nil {
		return err
	}
	req, err := c.withoutProjectScope().newRequest(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}
	resp, err := c.withoutProjectScope().PerformRequest(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// ListCustomDomains returns the custom domains configured for a project.
//
// Not to be confused with Outpost's tenant-portal custom domain, which is a
// different feature on a different route (/config/custom_domain).
func (c *Client) ListCustomDomains(ctx context.Context, projectID string) ([]CustomDomain, error) {
	path, err := apiPath("projects", projectID, "custom_domains")
	if err != nil {
		return nil, err
	}
	resp, err := c.withoutProjectScope().Get(ctx, path, "", nil)
	if err != nil {
		return nil, err
	}
	domains := []CustomDomain{}
	if _, err := postprocessJsonResponse(resp, &domains); err != nil {
		return nil, fmt.Errorf("failed to parse custom domain response: %w", err)
	}
	return domains, nil
}

// AddCustomDomain adds a hostname to a project.
//
// The endpoint echoes back only the hostname — not the new domain's id, and not
// its verification status. To act on the domain afterwards (to remove it, say)
// the caller has to list the project's domains and match on hostname.
func (c *Client) AddCustomDomain(ctx context.Context, projectID, hostname string) (*CustomDomain, error) {
	path, err := apiPath("projects", projectID, "custom_domains")
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(map[string]string{"hostname": hostname})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal custom domain request: %w", err)
	}
	resp, err := c.withoutProjectScope().Post(ctx, path, data, nil)
	if err != nil {
		return nil, err
	}
	var domain CustomDomain
	if _, err := postprocessJsonResponse(resp, &domain); err != nil {
		return nil, fmt.Errorf("failed to parse custom domain response: %w", err)
	}
	return &domain, nil
}

// DeleteCustomDomain removes a hostname from a project.
func (c *Client) DeleteCustomDomain(ctx context.Context, projectID, domainID string) error {
	// apiPath prepends the version prefix, so the whole route has to be built
	// in one call — joining two apiPath results yields
	// /2026-09-01/projects/x/2026-09-01/custom_domains/y, which compiles and
	// only fails against the API.
	path, err := apiPath("projects", projectID, "custom_domains", domainID)
	if err != nil {
		return err
	}
	req, err := c.withoutProjectScope().newRequest(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}
	resp, err := c.withoutProjectScope().PerformRequest(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// APIKey is an organization or project API key.
//
// Key is the bearer secret and is returned on create and roll. Everywhere else
// the non-secret KeyFingerprint is what identifies it — a renderer that prints
// Key by habit leaks a live credential into a terminal, a log or a CI artifact.
type APIKey struct {
	ID             string              `json:"id"`
	Label          string              `json:"label"`
	Key            string              `json:"key,omitempty"`
	TeamID         *string             `json:"team_id"`
	OrganizationID string              `json:"organization_id"`
	KeyFingerprint *string             `json:"key_fingerprint"`
	Scopes         []string            `json:"scopes,omitempty"`
	Grants         map[string]APIGrant `json:"grants,omitempty"`
	ExpiresAt      *string             `json:"expires_at"`
	UpdatedAt      string              `json:"updated_at,omitempty"`
}

// APIGrant is one entry of an API key's grants: the projects an organization
// key is limited to, or the per-resource scope overrides of a project key.
type APIGrant struct {
	Scopes []string `json:"scopes,omitempty"`
}

// APIKeyCreateRequest is the body of POST /organizations/current/api-keys.
//
// Scopes are free-form: the OpenAPI document declares no enum for them, only
// examples of the shape (gateway.events.read). There is nothing to validate
// against locally, so they are passed through and the API is the authority.
type APIKeyCreateRequest struct {
	Label  string              `json:"label"`
	Type   string              `json:"type"`
	TeamID *string             `json:"team_id,omitempty"`
	Scopes []string            `json:"scopes,omitempty"`
	Grants map[string]APIGrant `json:"grants,omitempty"`
}

// APIKeyUpdateRequest changes an existing key's permissions. The secret is
// unchanged.
type APIKeyUpdateRequest struct {
	Scopes []string            `json:"scopes,omitempty"`
	Grants map[string]APIGrant `json:"grants,omitempty"`
}

// ListAPIKeys returns the organization and project keys of the current
// organization.
func (c *Client) ListAPIKeys(ctx context.Context) ([]APIKey, error) {
	resp, err := c.withoutProjectScope().Get(ctx, APIPathPrefix+"/organizations/current/api-keys", "", nil)
	if err != nil {
		return nil, err
	}
	keys := []APIKey{}
	if _, err := postprocessJsonResponse(resp, &keys); err != nil {
		return nil, fmt.Errorf("failed to parse API key list response: %w", err)
	}
	return keys, nil
}

// CreateAPIKey issues a key. The response carries the secret, once.
func (c *Client) CreateAPIKey(ctx context.Context, req *APIKeyCreateRequest) (*APIKey, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal API key request: %w", err)
	}
	resp, err := c.withoutProjectScope().Post(ctx, APIPathPrefix+"/organizations/current/api-keys", data, nil)
	if err != nil {
		return nil, err
	}
	var key APIKey
	if _, err := postprocessJsonResponse(resp, &key); err != nil {
		return nil, fmt.Errorf("failed to parse API key response: %w", err)
	}
	return &key, nil
}

// UpdateAPIKey changes a key's scopes and grants, leaving the secret alone.
func (c *Client) UpdateAPIKey(ctx context.Context, id string, req *APIKeyUpdateRequest) (*APIKey, error) {
	path, err := apiPath("organizations", "current", "api-keys", id)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal API key request: %w", err)
	}
	resp, err := c.withoutProjectScope().Put(ctx, path, data, nil)
	if err != nil {
		return nil, err
	}
	var key APIKey
	if _, err := postprocessJsonResponse(resp, &key); err != nil {
		return nil, fmt.Errorf("failed to parse API key response: %w", err)
	}
	return &key, nil
}

// RollAPIKey returns a replacement key; the current one expires after delaySec.
func (c *Client) RollAPIKey(ctx context.Context, id string, delaySec int) (*APIKey, error) {
	path, err := apiPath("organizations", "current", "api-keys", id, "roll")
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(map[string]int{"delay_sec": delaySec})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal roll request: %w", err)
	}
	resp, err := c.withoutProjectScope().Post(ctx, path, data, nil)
	if err != nil {
		return nil, err
	}
	var key APIKey
	if _, err := postprocessJsonResponse(resp, &key); err != nil {
		return nil, fmt.Errorf("failed to parse API key response: %w", err)
	}
	return &key, nil
}

// DeleteAPIKey removes a key. It stops authenticating immediately.
func (c *Client) DeleteAPIKey(ctx context.Context, id string) error {
	path, err := apiPath("organizations", "current", "api-keys", id)
	if err != nil {
		return err
	}
	req, err := c.withoutProjectScope().newRequest(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}
	resp, err := c.withoutProjectScope().PerformRequest(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
