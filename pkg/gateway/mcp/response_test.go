package mcp

import (
	"encoding/json"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func firstText(t *testing.T, res *mcpsdk.CallToolResult) string {
	t.Helper()
	require.NotEmpty(t, res.Content)
	tc, ok := res.Content[0].(*mcpsdk.TextContent)
	require.True(t, ok, "expected TextContent, got %T", res.Content[0])
	return tc.Text
}

func TestJSONResultEnvelope_NoProject_MetaEmptyObject(t *testing.T) {
	res, err := JSONResultEnvelope(map[string]any{"items": []int{1}}, "", "", "")
	require.NoError(t, err)
	text := firstText(t, res)
	var root map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(text), &root))
	require.Contains(t, root, "data")
	require.Contains(t, root, "meta")
	var inner map[string]any
	require.NoError(t, json.Unmarshal(root["data"], &inner))
	items := inner["items"].([]any)
	require.Equal(t, float64(1), items[0])
	var meta map[string]any
	require.NoError(t, json.Unmarshal(root["meta"], &meta))
	require.NotContains(t, meta, "active_project_id")
	require.NotContains(t, meta, "active_project_name")
	require.NotContains(t, meta, "active_project_org")
}

func TestJSONResultEnvelope_WithProject_FlatMetaFields(t *testing.T) {
	res, err := JSONResultEnvelope(
		map[string]any{"count": 2},
		"tm_Mcf7DGlOQmds",
		"Demos",
		"trigger-dev-github",
	)
	require.NoError(t, err)
	var root map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(firstText(t, res)), &root))
	var dataObj struct {
		Count int `json:"count"`
	}
	require.NoError(t, json.Unmarshal(root["data"], &dataObj))
	require.Equal(t, 2, dataObj.Count)

	var meta struct {
		ActiveProjectID   string `json:"active_project_id"`
		ActiveProjectOrg  string `json:"active_project_org"`
		ActiveProjectName string `json:"active_project_name"`
	}
	require.NoError(t, json.Unmarshal(root["meta"], &meta))
	require.Equal(t, "tm_Mcf7DGlOQmds", meta.ActiveProjectID)
	require.Equal(t, "Demos", meta.ActiveProjectOrg)
	require.Equal(t, "trigger-dev-github", meta.ActiveProjectName)
}

func TestJSONResultEnvelope_DataCanBeArray(t *testing.T) {
	res, err := JSONResultEnvelope([]int{1, 2}, "proj_x", "", "Name")
	require.NoError(t, err)
	var root map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(firstText(t, res)), &root))
	var arr []int
	require.NoError(t, json.Unmarshal(root["data"], &arr))
	require.Equal(t, []int{1, 2}, arr)
	var meta map[string]any
	require.NoError(t, json.Unmarshal(root["meta"], &meta))
	require.Equal(t, "proj_x", meta["active_project_id"])
	require.Equal(t, "Name", meta["active_project_name"])
	require.NotContains(t, meta, "active_project_org")
}

func TestJSONResultEnvelope_IDOnly_IncludesEmptyName(t *testing.T) {
	res, err := JSONResultEnvelope(map[string]int{"n": 1}, "proj_only", "", "")
	require.NoError(t, err)
	var root map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(firstText(t, res)), &root))
	var meta map[string]any
	require.NoError(t, json.Unmarshal(root["meta"], &meta))
	require.Equal(t, "proj_only", meta["active_project_id"])
	require.Contains(t, meta, "active_project_name")
	require.Equal(t, "", meta["active_project_name"])
	require.NotContains(t, meta, "active_project_org")
}
