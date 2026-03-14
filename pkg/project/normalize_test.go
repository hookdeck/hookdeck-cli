package project

import (
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeProjects(t *testing.T) {
	projects := []hookdeck.Project{
		{Id: "p1", Name: "[Acme] Prod", Mode: "inbound"},
		{Id: "p2", Name: "[Acme] Staging", Mode: "console"},
		{Id: "p3", Name: "[Org2] Outpost", Mode: "outpost"},
		{Id: "p4", Name: "[Org] Outbound", Mode: "outbound"},
		{Id: "p5", Name: "No brackets", Mode: "inbound"},
	}
	items := NormalizeProjects(projects, "p2")
	// outbound excluded, so 4 items (p4 excluded)
	require.Len(t, items, 4)

	// p1: Gateway, Acme, Prod
	assert.Equal(t, "p1", items[0].Id)
	assert.Equal(t, "Acme", items[0].Org)
	assert.Equal(t, "Prod", items[0].Project)
	assert.Equal(t, "Gateway", items[0].Type)
	assert.False(t, items[0].Current)

	// p2: current, console mode -> Console type
	assert.True(t, items[1].Current)
	assert.Equal(t, "Console", items[1].Type)

	// p3: Outpost
	assert.Equal(t, "Outpost", items[2].Type)

	// p5: unparseable name -> org "", project "No brackets"
	assert.Equal(t, "", items[3].Org)
	assert.Equal(t, "No brackets", items[3].Project)
}

func TestNormalizeProjects_EmptyList(t *testing.T) {
	items := NormalizeProjects(nil, "")
	assert.Empty(t, items)
}

func TestNormalizeProjects_AllOutbound(t *testing.T) {
	projects := []hookdeck.Project{
		{Id: "p1", Name: "[A] P", Mode: "outbound"},
	}
	items := NormalizeProjects(projects, "p1")
	assert.Empty(t, items)
}

func TestFilterByType(t *testing.T) {
	items := []ProjectListItem{
		{Type: "Gateway"},
		{Type: "Outpost"},
		{Type: "Gateway"},
		{Type: "Console"},
	}
	got := FilterByType(items, "gateway")
	require.Len(t, got, 2)
	assert.Equal(t, "Gateway", got[0].Type)
	assert.Equal(t, "Gateway", got[1].Type)

	got = FilterByType(items, "")
	require.Len(t, got, 4)

	got = FilterByType(items, "console")
	require.Len(t, got, 1)
	assert.Equal(t, "Console", got[0].Type)
}

func TestFilterByOrgProject(t *testing.T) {
	items := []ProjectListItem{
		{Org: "Acme", Project: "Prod"},
		{Org: "Acme", Project: "Staging"},
		{Org: "Other", Project: "Prod"},
	}
	got := FilterByOrgProject(items, "acme", "")
	require.Len(t, got, 2)
	got = FilterByOrgProject(items, "", "prod")
	require.Len(t, got, 2)
	got = FilterByOrgProject(items, "acme", "stag")
	require.Len(t, got, 1)
	assert.Equal(t, "Staging", got[0].Project)
}

func TestProjectListItem_DisplayLine(t *testing.T) {
	it := ProjectListItem{Org: "Acme", Project: "Prod", Type: "Gateway", Current: false}
	assert.Equal(t, "Acme / Prod | Gateway", it.DisplayLine())

	it.Current = true
	assert.Equal(t, "Acme / Prod (current) | Gateway", it.DisplayLine())

	it.Org = ""
	it.Project = "Solo"
	assert.Equal(t, "Solo (current) | Gateway", it.DisplayLine())
}
