package config

import "strings"

// Project types as the API names them: `type` on GET /projects, and `team_type`
// on the CLI auth endpoints. These are the values stored in config and passed
// around internally, so the CLI speaks the same vocabulary as the API it calls.
const (
	ProjectTypeEventGateway = "event_gateway"
	ProjectTypeOutpost      = "outpost"
	ProjectTypeConsole      = "console"
)

// Labels shown to the user. Presentation only: derived at print time, never
// stored, so there is one source of truth for what a project is.
const (
	ProjectLabelGateway = "Gateway"
	ProjectLabelOutpost = "Outpost"
	ProjectLabelConsole = "Console"
)

// OutboundMode is the legacy internal mode for outbound projects. The API folds
// inbound and outbound into event_gateway.
const OutboundMode = "outbound"

// TypeLabel returns the label shown to the user for an API project type.
func TypeLabel(projectType string) string {
	switch strings.ToLower(projectType) {
	case ProjectTypeEventGateway:
		return ProjectLabelGateway
	case ProjectTypeConsole:
		return ProjectLabelConsole
	case ProjectTypeOutpost:
		return ProjectLabelOutpost
	default:
		return ""
	}
}

// LabelToType maps a display label back to the API type. Needed for config files
// written before project_type held the API value, and for anything that only has
// the label a user was shown.
func LabelToType(label string) string {
	switch strings.ToLower(label) {
	case strings.ToLower(ProjectLabelGateway):
		return ProjectTypeEventGateway
	case strings.ToLower(ProjectLabelConsole):
		return ProjectTypeConsole
	case strings.ToLower(ProjectLabelOutpost):
		return ProjectTypeOutpost
	default:
		return ""
	}
}

// ModeToType maps a legacy internal mode to the API project type.
func ModeToType(mode string) string {
	switch strings.ToLower(mode) {
	case "inbound", OutboundMode:
		return ProjectTypeEventGateway
	case "console":
		return ProjectTypeConsole
	case "outpost":
		return ProjectTypeOutpost
	default:
		return ""
	}
}

// TypeToLegacyMode returns a representative legacy mode for a project type, kept
// so older CLIs reading the same config still resolve a project. The API folds
// inbound and outbound into event_gateway, so a round trip through the type
// normalizes outbound to inbound.
func TypeToLegacyMode(projectType string) string {
	switch strings.ToLower(projectType) {
	case ProjectTypeEventGateway:
		return "inbound"
	case ProjectTypeConsole:
		return "console"
	case ProjectTypeOutpost:
		return "outpost"
	default:
		return ""
	}
}

// NormalizeProjectType accepts an API type, a display label, or a legacy mode and
// returns the API type. Every value read from disk or handed in by a caller goes
// through here, so the three vocabularies converge in one place rather than at
// each call site.
func NormalizeProjectType(value string) string {
	lowered := strings.ToLower(strings.TrimSpace(value))
	if lowered == "" {
		return ""
	}
	if TypeLabel(lowered) != "" {
		return lowered
	}
	if t := LabelToType(lowered); t != "" {
		return t
	}
	return ModeToType(lowered)
}

// IsGatewayProject reports whether the value denotes a project the gateway
// commands can act on. Console projects count: they are Event Gateway projects
// with a different entry point.
func IsGatewayProject(value string) bool {
	switch NormalizeProjectType(value) {
	case ProjectTypeEventGateway, ProjectTypeConsole:
		return true
	default:
		return false
	}
}

// ProjectTypeToJSON returns the value used in `--output json` and accepted by the
// `--type` filter. Deliberately not the API type: `gateway` is what the CLI has
// always emitted, and changing it would break anyone parsing that output.
func ProjectTypeToJSON(projectType string) string {
	switch NormalizeProjectType(projectType) {
	case ProjectTypeEventGateway:
		return "gateway"
	case ProjectTypeOutpost:
		return "outpost"
	case ProjectTypeConsole:
		return "console"
	default:
		return strings.ToLower(projectType)
	}
}

// IsConsoleProject reports whether the first recognized value identifies a
// Console project. Values are given newest-field-first, matching the order the
// CLI reads team_type, team_product and team_mode from an auth response.
func IsConsoleProject(values ...string) bool {
	for _, v := range values {
		if t := NormalizeProjectType(v); t != "" {
			return t == ProjectTypeConsole
		}
	}
	return false
}
