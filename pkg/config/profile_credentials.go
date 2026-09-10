package config

import "github.com/hookdeck/hookdeck-cli/pkg/hookdeck"

// resolveProduct returns the product to store. It prefers the current
// team_product field and falls back to the pre-2026-09-01 team_mode, so a
// response missing the new field leaves the profile usable rather than blank:
// an empty product makes IsGatewayProject false and fails every gateway command.
func resolveProduct(product, legacyMode string) string {
	if product != "" {
		return product
	}
	return ModeToProduct(legacyMode)
}

// ApplyValidateAPIKeyResponse updates project fields from GET /cli-auth/validate.
// When clearGuestURL is true, GuestURL is cleared (e.g. hookdeck login re-verify).
// When false, GuestURL is left unchanged (e.g. gateway PreRun resolving type only).
func (p *Profile) ApplyValidateAPIKeyResponse(resp *hookdeck.ValidateAPIKeyResponse, clearGuestURL bool) {
	if resp == nil {
		return
	}
	p.ProjectId = resp.ProjectID
	product := resolveProduct(resp.ProjectProduct, resp.ProjectMode)
	p.ProjectProduct = product
	p.ProjectMode = ProductToLegacyMode(product)
	p.ProjectType = ProductToProjectType(product)
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
	product := resolveProduct(resp.ProjectProduct, resp.ProjectMode)
	p.ProjectProduct = product
	p.ProjectMode = ProductToLegacyMode(product)
	p.ProjectType = ProductToProjectType(product)
	p.GuestURL = guestURL
}

// ApplyCIClient applies credentials from hookdeck login --ci.
func (p *Profile) ApplyCIClient(ci hookdeck.CIClient) {
	p.APIKey = ci.APIKey
	p.ProjectId = ci.ProjectID
	product := resolveProduct(ci.ProjectProduct, ci.ProjectMode)
	p.ProjectProduct = product
	p.ProjectMode = ProductToLegacyMode(product)
	p.ProjectType = ProductToProjectType(product)
	p.GuestURL = ""
}
