package config

import "strings"

// Project type display values (user-facing and config).
const (
	ProjectTypeGateway = "Gateway"
	ProjectTypeOutpost = "Outpost"
	ProjectTypeConsole = "Console"

	ProjectProductEventGateway = "event_gateway"
	ProjectProductOutpost      = "outpost"
	ProjectProductConsole      = "console"
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

// ProductToProjectType maps the public API product to the CLI display type.
func ProductToProjectType(product string) string {
	switch strings.ToLower(product) {
	case ProjectProductEventGateway:
		return ProjectTypeGateway
	case ProjectProductConsole:
		return ProjectTypeConsole
	case ProjectProductOutpost:
		return ProjectTypeOutpost
	default:
		return ""
	}
}

// ProjectTypeToProduct maps the CLI display type to the public API product.
// Case-insensitive, like the other mappers in this file.
func ProjectTypeToProduct(projectType string) string {
	switch strings.ToLower(projectType) {
	case strings.ToLower(ProjectTypeGateway):
		return ProjectProductEventGateway
	case strings.ToLower(ProjectTypeConsole):
		return ProjectProductConsole
	case strings.ToLower(ProjectTypeOutpost):
		return ProjectProductOutpost
	default:
		return ""
	}
}

// ProductToLegacyMode returns a representative legacy mode for local config
// compatibility. The public API intentionally combines inbound and outbound
// projects under the event_gateway product.
func ProductToLegacyMode(product string) string {
	switch strings.ToLower(product) {
	case ProjectProductEventGateway:
		return "inbound"
	case ProjectProductConsole:
		return "console"
	case ProjectProductOutpost:
		return "outpost"
	default:
		return ""
	}
}

// ModeToProduct maps a legacy internal API mode to the public product.
func ModeToProduct(mode string) string {
	switch strings.ToLower(mode) {
	case "inbound", OutboundMode:
		return ProjectProductEventGateway
	case "console":
		return ProjectProductConsole
	case "outpost":
		return ProjectProductOutpost
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

// IsGatewayProject returns true if the given type, product, or legacy mode represents a Gateway project.
// Case-insensitive, like the other mappers in this file.
func IsGatewayProject(typeProductOrMode string) bool {
	switch strings.ToLower(typeProductOrMode) {
	case strings.ToLower(ProjectTypeGateway), strings.ToLower(ProjectTypeConsole), ProjectProductEventGateway, "inbound", "outbound", "console":
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
