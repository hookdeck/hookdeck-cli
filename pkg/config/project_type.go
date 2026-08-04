package config

import "strings"

// Project type display values (user-facing and config).
const (
	ProjectTypeGateway = "Gateway"
	ProjectTypeOutpost = "Outpost"
	ProjectTypeConsole = "Console"
)

// OutboundMode is the API mode for outbound projects; treated as Gateway (same as inbound).
const OutboundMode = "outbound"

// ModeToProjectType maps API mode to display project type.
// Inbound and outbound both map to Gateway. Returns empty string only for unknown modes.
func ModeToProjectType(mode string) string {
	switch strings.ToLower(mode) {
	case "inbound":
		return ProjectTypeGateway
	case OutboundMode:
		return ProjectTypeGateway // same as inbound for gateway purposes
	case "console":
		return ProjectTypeConsole
	case "outpost":
		return ProjectTypeOutpost
	default:
		return ""
	}
}

// ProjectTypeToMode maps display type to API mode (for backward compat when only type is set).
func ProjectTypeToMode(projectType string) string {
	switch projectType {
	case ProjectTypeGateway:
		return "inbound"
	case ProjectTypeConsole:
		return "console"
	case ProjectTypeOutpost:
		return "outpost"
	default:
		return ""
	}
}

// IsGatewayProject returns true if the given type or mode represents a Gateway project (inbound, outbound, or console).
func IsGatewayProject(typeOrMode string) bool {
	switch typeOrMode {
	case ProjectTypeGateway, ProjectTypeConsole, "inbound", "outbound", "console":
		return true
	default:
		return false
	}
}

// ProjectTypeToJSON returns the lowercase type for JSON output (gateway, outpost, console).
func ProjectTypeToJSON(projectType string) string {
	switch projectType {
	case ProjectTypeGateway:
		return "gateway"
	case ProjectTypeOutpost:
		return "outpost"
	case ProjectTypeConsole:
		return "console"
	default:
		return strings.ToLower(projectType)
	}
}
