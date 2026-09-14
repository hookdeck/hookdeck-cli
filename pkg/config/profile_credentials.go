package config

import "github.com/hookdeck/hookdeck-cli/pkg/hookdeck"

// resolveType returns the project type to store: the current team_type field,
// falling back to the pre-2026-09-01 team_mode. Without a fallback a response
// missing the newer field leaves the profile blank, and an empty type makes
// IsGatewayProject false and fails every gateway command.
func resolveType(projectType, legacyMode string) string {
	if t := NormalizeProjectType(projectType); t != "" {
		return t
	}
	return ModeToType(legacyMode)
}

// storeProjectIdentity records what the API said rather than only what this CLI
// could resolve.
//
// Deriving both fields from a resolved type blanked them whenever the type was
// unrecognized - a project type this CLI has not heard of - leaving a profile
// with no type and no mode. Every gateway command then failed with "current
// project type is ." and the value another CLI version could have used was gone.
//
// So each field keeps its raw value when one arrived, and is derived only to
// fill a gap. Reads already normalize, so an unknown value is inert here and
// meaningful to a CLI that understands it. Config.setProjectIdentity takes the
// same position for values supplied on the command line.
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
