package config

import "github.com/hookdeck/hookdeck-cli/pkg/hookdeck"

// resolveType returns the project type to store. It prefers the current
// team_type field, then team_product (served briefly before the rename), then
// the pre-2026-09-01 team_mode. Without the fallbacks a response missing the
// newest field leaves the profile blank: an empty type makes IsGatewayProject
// false and fails every gateway command.
func resolveType(projectType, legacyProduct, legacyMode string) string {
	if t := NormalizeProjectType(projectType); t != "" {
		return t
	}
	if t := NormalizeProjectType(legacyProduct); t != "" {
		return t
	}
	return ModeToType(legacyMode)
}

// ApplyValidateAPIKeyResponse updates project fields from GET /cli-auth/validate.
// When clearGuestURL is true, GuestURL is cleared (e.g. hookdeck login re-verify).
// When false, GuestURL is left unchanged (e.g. gateway PreRun resolving type only).
func (p *Profile) ApplyValidateAPIKeyResponse(resp *hookdeck.ValidateAPIKeyResponse, clearGuestURL bool) {
	if resp == nil {
		return
	}
	p.ProjectId = resp.ProjectID
	projectType := resolveType(resp.ProjectType, resp.ProjectProduct, resp.ProjectMode)
	p.ProjectType = projectType
	p.ProjectMode = TypeToLegacyMode(projectType)
	if clearGuestURL {
		p.GuestURL = ""
	}
}

// ApplyPollAPIKeyResponse applies credentials from a completed CLI auth poll (browser or interactive login).
// guestURL is the guest upgrade URL when applicable; use "" for a normal account login.
func (p *Profile) ApplyPollAPIKeyResponse(resp *hookdeck.PollAPIKeyResponse, guestURL string) {
	if resp == nil {
		return
	}
	p.APIKey = resp.APIKey
	p.ProjectId = resp.ProjectID
	projectType := resolveType(resp.ProjectType, resp.ProjectProduct, resp.ProjectMode)
	p.ProjectType = projectType
	p.ProjectMode = TypeToLegacyMode(projectType)
	p.GuestURL = guestURL
}

// ApplyCIClient applies credentials from hookdeck login --ci.
func (p *Profile) ApplyCIClient(ci hookdeck.CIClient) {
	p.APIKey = ci.APIKey
	p.ProjectId = ci.ProjectID
	projectType := resolveType(ci.ProjectType, ci.ProjectProduct, ci.ProjectMode)
	p.ProjectType = projectType
	p.ProjectMode = TypeToLegacyMode(projectType)
	p.GuestURL = ""
}
