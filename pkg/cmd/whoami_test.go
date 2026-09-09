package cmd

import (
	"errors"
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func TestResolveActiveProject(t *testing.T) {
	validateResponse := &hookdeck.ValidateAPIKeyResponse{
		ProjectID:        "tm_bound",
		ProjectName:      "Bound Project",
		ProjectProduct:   "event_gateway",
		OrganizationName: "Org A",
	}

	projects := []hookdeck.Project{
		{Id: "tm_bound", Name: "[Org A] Bound Project", Product: "event_gateway"},
		{Id: "tm_active", Name: "[Org B] Active Project", Product: "event_gateway"},
		{Id: "tm_unparsable", Name: "No Org Format", Product: "event_gateway"},
	}

	t.Run("no active project id uses validate response", func(t *testing.T) {
		called := false
		name, org, mode, note := resolveActiveProject(validateResponse, "", func() ([]hookdeck.Project, error) {
			called = true
			return projects, nil
		})
		if called {
			t.Error("listProjects should not be called when no active project id is set")
		}
		if name != "Bound Project" || org != "Org A" || mode != "event_gateway" || note != "" {
			t.Errorf("got (%q, %q, %q, %q)", name, org, mode, note)
		}
	})

	t.Run("active project matches key-bound project uses validate response", func(t *testing.T) {
		called := false
		name, org, mode, note := resolveActiveProject(validateResponse, "tm_bound", func() ([]hookdeck.Project, error) {
			called = true
			return projects, nil
		})
		if called {
			t.Error("listProjects should not be called when active project matches the key-bound project")
		}
		if name != "Bound Project" || org != "Org A" || mode != "event_gateway" || note != "" {
			t.Errorf("got (%q, %q, %q, %q)", name, org, mode, note)
		}
	})

	t.Run("active project differs and is resolved from project list", func(t *testing.T) {
		name, org, mode, note := resolveActiveProject(validateResponse, "tm_active", func() ([]hookdeck.Project, error) {
			return projects, nil
		})
		if name != "Active Project" || org != "Org B" || mode != "event_gateway" || note != "" {
			t.Errorf("got (%q, %q, %q, %q)", name, org, mode, note)
		}
	})

	t.Run("unparsable project name falls back to full name without org", func(t *testing.T) {
		name, org, mode, note := resolveActiveProject(validateResponse, "tm_unparsable", func() ([]hookdeck.Project, error) {
			return projects, nil
		})
		if name != "No Org Format" || org != "" || mode != "event_gateway" || note != "" {
			t.Errorf("got (%q, %q, %q, %q)", name, org, mode, note)
		}
	})

	t.Run("list error falls back to validate response with warning", func(t *testing.T) {
		name, org, mode, note := resolveActiveProject(validateResponse, "tm_active", func() ([]hookdeck.Project, error) {
			return nil, errors.New("boom")
		})
		if name != "Bound Project" || org != "Org A" || mode != "event_gateway" {
			t.Errorf("got (%q, %q, %q)", name, org, mode)
		}
		if note == "" {
			t.Error("expected a warning note when the project list cannot be fetched")
		}
	})

	t.Run("active project missing from list falls back with warning", func(t *testing.T) {
		name, org, mode, note := resolveActiveProject(validateResponse, "tm_deleted", func() ([]hookdeck.Project, error) {
			return projects, nil
		})
		if name != "Bound Project" || org != "Org A" || mode != "event_gateway" {
			t.Errorf("got (%q, %q, %q)", name, org, mode)
		}
		if note == "" {
			t.Error("expected a warning note when the active project is not in the list")
		}
	})
}
