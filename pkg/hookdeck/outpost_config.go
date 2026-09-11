package hookdeck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// OutpostManagedConfig is the operator configuration for a project.
//
// The API models every value as a string, and a null clears a value back to its
// default, so this is a flat map rather than a struct: the key set is large and
// evolves independently of the CLI. Using a map means a newly added key works
// without a CLI release.
type OutpostManagedConfig map[string]*string

// OutpostDeploymentStatus reports the state of a project's Outpost deployment.
type OutpostDeploymentStatus struct {
	Status         string `json:"status"`
	Version        string `json:"version,omitempty"`
	PortalHostname string `json:"portal_hostname,omitempty"`
}

// OutpostCustomDomain describes the custom hostname serving a project's tenant
// portal.
type OutpostCustomDomain struct {
	Hostname string `json:"hostname,omitempty"`
	Status   string `json:"status,omitempty"`
	// Verification carries provider-specific DNS records to add. Its shape is
	// determined by the DNS provider, so it is left untyped.
	Verification []map[string]interface{} `json:"verification,omitempty"`
}

// GetOutpostConfig retrieves the project's operator configuration.
func (c *Client) GetOutpostConfig(ctx context.Context) (OutpostManagedConfig, error) {
	resp, err := c.Get(ctx, APIPathPrefix+"/config", "", nil)
	if err != nil {
		return nil, err
	}

	var config OutpostManagedConfig
	if _, err := postprocessJsonResponse(resp, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config response: %w", err)
	}

	return config, nil
}

// UpdateOutpostConfig applies a partial update to the operator configuration.
//
// Only the supplied keys are changed. A nil value clears a key back to its
// default; some keys instead accept an empty string to turn a behaviour off,
// which the API documents per key.
func (c *Client) UpdateOutpostConfig(ctx context.Context, update OutpostManagedConfig) (OutpostManagedConfig, error) {
	if len(update) == 0 {
		return nil, fmt.Errorf("no configuration values to update")
	}

	data, err := json.Marshal(update)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config update: %w", err)
	}

	req, err := c.newRequest(ctx, http.MethodPatch, APIPathPrefix+"/config", data)
	if err != nil {
		return nil, err
	}

	resp, err := c.PerformRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	var config OutpostManagedConfig
	if _, err := postprocessJsonResponse(resp, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config response: %w", err)
	}

	return config, nil
}

// GetOutpostStatus retrieves the deployment status for the project.
func (c *Client) GetOutpostStatus(ctx context.Context) (*OutpostDeploymentStatus, error) {
	resp, err := c.Get(ctx, APIPathPrefix+"/status", "", nil)
	if err != nil {
		return nil, err
	}

	var status OutpostDeploymentStatus
	if _, err := postprocessJsonResponse(resp, &status); err != nil {
		return nil, fmt.Errorf("failed to parse status response: %w", err)
	}

	return &status, nil
}

// GetOutpostCustomDomain retrieves the tenant portal's custom domain, if one is
// configured.
func (c *Client) GetOutpostCustomDomain(ctx context.Context) (*OutpostCustomDomain, error) {
	resp, err := c.Get(ctx, APIPathPrefix+"/config/custom_domain", "", nil)
	if err != nil {
		return nil, err
	}

	var domain OutpostCustomDomain
	if _, err := postprocessJsonResponse(resp, &domain); err != nil {
		return nil, fmt.Errorf("failed to parse custom domain response: %w", err)
	}

	return &domain, nil
}

// AddOutpostCustomDomain configures a custom hostname for the tenant portal.
func (c *Client) AddOutpostCustomDomain(ctx context.Context, hostname string) (*OutpostCustomDomain, error) {
	data, err := json.Marshal(map[string]string{"hostname": hostname})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal custom domain request: %w", err)
	}

	resp, err := c.Post(ctx, APIPathPrefix+"/config/custom_domain", data, nil)
	if err != nil {
		return nil, err
	}

	var domain OutpostCustomDomain
	if _, err := postprocessJsonResponse(resp, &domain); err != nil {
		return nil, fmt.Errorf("failed to parse custom domain response: %w", err)
	}

	return &domain, nil
}

// DeleteOutpostCustomDomain removes the tenant portal's custom domain.
func (c *Client) DeleteOutpostCustomDomain(ctx context.Context) error {
	req, err := c.newRequest(ctx, "DELETE", APIPathPrefix+"/config/custom_domain", nil)
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
