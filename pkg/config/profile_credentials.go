package config

import "github.com/hookdeck/hookdeck-cli/pkg/hookdeck"

// ApplyValidateAPIKeyResponse updates project fields from GET /cli-auth/validate.
// When clearGuestURL is true, GuestURL is cleared (e.g. hookdeck login re-verify).
// When false, GuestURL is left unchanged (e.g. gateway PreRun resolving type only).
func (p *Profile) ApplyValidateAPIKeyResponse(resp *hookdeck.ValidateAPIKeyResponse, clearGuestURL bool) {
	if resp == nil {
		return
	}
	p.ProjectId = resp.ProjectID
	p.ProjectMode = resp.ProjectMode
	p.ProjectType = ModeToProjectType(resp.ProjectMode)
	if clearGuestURL {
		p.GuestURL = ""
		p.GuestUserID = ""
	}
}

// ApplyPollAPIKeyResponse applies credentials from a completed CLI auth poll (browser or interactive login).
// guestURL is the guest upgrade URL when applicable; use "" for a normal account login.
// guestUserID is the Hookdeck user id when the session is a guest account.
func (p *Profile) ApplyPollAPIKeyResponse(resp *hookdeck.PollAPIKeyResponse, guestURL string, guestUserID string) {
	if resp == nil {
		return
	}
	p.APIKey = resp.APIKey
	p.ProjectId = resp.ProjectID
	p.ProjectMode = resp.ProjectMode
	p.ProjectType = ModeToProjectType(resp.ProjectMode)
	p.GuestURL = guestURL
	if guestUserID != "" {
		p.GuestUserID = guestUserID
	} else if guestURL == "" {
		p.GuestUserID = ""
	}
}

// ApplyCIClient applies credentials from hookdeck login --ci.
func (p *Profile) ApplyCIClient(ci hookdeck.CIClient) {
	p.APIKey = ci.APIKey
	p.ProjectId = ci.ProjectID
	p.ProjectMode = ci.ProjectMode
	p.ProjectType = ModeToProjectType(ci.ProjectMode)
	p.GuestURL = ""
	p.GuestUserID = ""
}
