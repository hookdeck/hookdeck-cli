package project

import (
	"strings"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// ProjectListItem is a normalized project entry for list output, JSON, and project use selector.
type ProjectListItem struct {
	Id      string
	Org     string
	Project string
	Type    string // display type: Gateway, Outpost, Console
	Current bool
}

// NormalizeProjects converts API projects into a normalized list: parses name once, excludes outbound,
// sets type from mode. currentID is the profile's current project id for the Current flag.
func NormalizeProjects(projects []hookdeck.Project, currentID string) []ProjectListItem {
	var out []ProjectListItem
	for _, p := range projects {
		projectType := config.ModeToProjectType(p.Mode)
		if projectType == "" {
			// outbound or unknown: exclude from list
			continue
		}
		org, proj, err := ParseProjectName(p.Name)
		if err != nil {
			// fallback: use full name as project, empty org
			org = ""
			proj = p.Name
		}
		out = append(out, ProjectListItem{
			Id:      p.Id,
			Org:     org,
			Project: proj,
			Type:    projectType,
			Current: p.Id == currentID,
		})
	}
	return out
}

// FilterByType returns items whose Type (display) matches the given type filter (lowercase: gateway, outpost, console).
func FilterByType(items []ProjectListItem, typeFilter string) []ProjectListItem {
	if typeFilter == "" {
		return items
	}
	var out []ProjectListItem
	for _, it := range items {
		if config.ProjectTypeToJSON(it.Type) == typeFilter {
			out = append(out, it)
		}
	}
	return out
}

// DisplayLine returns the human-readable line for an item: "Org / Project (current?) | Type".
func (it *ProjectListItem) DisplayLine() string {
	namePart := it.Project
	if it.Org != "" {
		namePart = it.Org + " / " + it.Project
	}
	if it.Current {
		namePart += " (current)"
	}
	return namePart + " | " + it.Type
}

// FilterByOrgProject filters items by org and/or project name substrings (case-insensitive).
// If orgSubstr is non-empty, item must match. If projectSubstr is non-empty, item must match.
func FilterByOrgProject(items []ProjectListItem, orgSubstr, projectSubstr string) []ProjectListItem {
	orgLower := strings.ToLower(orgSubstr)
	projLower := strings.ToLower(projectSubstr)
	var out []ProjectListItem
	for _, it := range items {
		if orgSubstr != "" && !strings.Contains(strings.ToLower(it.Org), orgLower) {
			continue
		}
		if projectSubstr != "" && !strings.Contains(strings.ToLower(it.Project), projLower) {
			continue
		}
		out = append(out, it)
	}
	return out
}
