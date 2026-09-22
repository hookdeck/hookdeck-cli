package config

import "github.com/hookdeck/hookdeck-cli/pkg/hookdeck"

// resolveType prefers team_type and falls back to the pre-2026-09-01 team_mode.
// Without the fallback a response missing team_type blanks the profile, which
// fails every gateway command.
func resolveType(projectType, legacyMode string) string {
	if t := NormalizeProjectType(projectType); t != "" {
		return t
	}
	return ModeToType(legacyMode)
}

// storeProjectIdentity records what the API said, not only what this CLI could
// resolve. Deriving both fields from a resolved type blanked them for an
// unrecognized type, leaving "current project type is ." on every gateway
// command. Recognized values are normalized; unrecognized ones kept verbatim.
func storeProjectIdentity(p *Profile, rawType, rawMode string) {
	resolved := resolveType(rawType, rawMode)

	// Recognized values are held normalized, for the same reason as on load;
	// an unrecognized one is kept verbatim so it survives to disk.
	p.ProjectType = resolved
	if p.ProjectType == "" {
		p.ProjectType = rawType
	}

	p.ProjectMode = rawMode
	if p.ProjectMode == "" {
		p.ProjectMode = TypeToLegacyMode(resolved)
	}
}

// ApplyValidateAPIKeyResponse updates project fields from GET /cli-auth/validate.
// When clearGuestURL is true, GuestURL is cleared (e.g. hookdeck login re-verify).
// When false, GuestURL is left unchanged (e.g. gateway PreRun resolving type only).
func (p *Profile) ApplyValidateAPIKeyResponse(resp *hookdeck.ValidateAPIKeyResponse, clearGuestURL bool) {
	if resp == nil {
		return
	}
	p.ProjectId = resp.ProjectID
	storeProjectIdentity(p, resp.ProjectType, resp.ProjectMode)
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
	storeProjectIdentity(p, resp.ProjectType, resp.ProjectMode)
	p.GuestURL = guestURL
}

// ApplyCIClient applies credentials from hookdeck login --ci.
func (p *Profile) ApplyCIClient(ci hookdeck.CIClient) {
	p.APIKey = ci.APIKey
	p.ProjectId = ci.ProjectID
	storeProjectIdentity(p, ci.ProjectType, ci.ProjectMode)
	p.GuestURL = ""
}
