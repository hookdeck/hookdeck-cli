package mcp

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// include=data puts each item's payload in the list response. Without it an
// agent that needs payloads calls the singular tool once per event, and a list
// of a hundred events becomes a hundred more calls.

// schemaProperty returns one property of a tool's input schema as decoded JSON.
func schemaProperty(t *testing.T, tool *mcpsdk.Tool, name string) map[string]any {
	t.Helper()
	require.NotNil(t, tool)
	raw, err := json.Marshal(tool.InputSchema)
	require.NoError(t, err)
	var schema struct {
		Properties map[string]map[string]any `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(raw, &schema))
	prop, ok := schema.Properties[name]
	require.True(t, ok, "%s does not advertise %q", tool.Name, name)
	return prop
}

// TestListToolsOfferIncludeData pins the schema: include exists, takes only
// "data" (the one value the API declares), and both the property and the tool
// description warn about response size where the agent decides to use it.
func TestListToolsOfferIncludeData(t *testing.T) {
	session := connectInMemory(t, newTestClient("https://api.hookdeck.com", "test-key"))
	tools := listTools(t, session)

	for _, name := range []string{eventsToolName, requestsToolName} {
		t.Run(name, func(t *testing.T) {
			tool := tools[name]
			prop := schemaProperty(t, tool, "include")
			assert.Equal(t, []any{"data"}, prop["enum"])

			desc, _ := prop["description"].(string)
			assert.Contains(t, desc, "limit", "the property must tell the agent to keep limit small")

			assert.Contains(t, tool.Description, `include: "data"`)
			assert.Contains(t, tool.Description, "much larger",
				"the tool description must warn that payloads make responses larger")
			assert.Contains(t, tool.Description, "keep limit small")
		})
	}
}

// TestRequestsListForwardsIncludeData: the requests collection route declares
// include, so it has to reach the wire. The events routes are covered by
// eventFilterForwarding.
func TestRequestsListForwardsIncludeData(t *testing.T) {
	var saw string
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/requests": func(w http.ResponseWriter, r *http.Request) {
			saw = r.URL.Query().Get("include")
			_ = json.NewEncoder(w).Encode(listResponse())
		},
	})
	result := callTool(t, session, requestsToolName, map[string]any{"action": "list", "include": "data"})
	require.False(t, result.IsError, textContent(t, result))
	assert.Equal(t, "data", saw)
}

// TestListOmitsIncludeWhenUnset: payloads are opt-in. Sending include by
// default would make every list call as heavy as the warning says.
func TestListOmitsIncludeWhenUnset(t *testing.T) {
	for _, tt := range []struct {
		tool, path string
	}{
		{eventsToolName, "/events"},
		{requestsToolName, "/requests"},
	} {
		t.Run(tt.tool, func(t *testing.T) {
			present := false
			called := false
			session := mockAPIWithClient(t, map[string]http.HandlerFunc{
				hookdeck.APIPathPrefix + tt.path: func(w http.ResponseWriter, r *http.Request) {
					called = true
					_, present = r.URL.Query()["include"]
					_ = json.NewEncoder(w).Encode(listResponse())
				},
			})
			result := callTool(t, session, tt.tool, map[string]any{"action": "list"})
			require.False(t, result.IsError, textContent(t, result))
			require.True(t, called)
			assert.False(t, present, "include must not be sent unless the caller asks for it")
		})
	}
}

// TestListReturnsIncludedData: forwarding the parameter is only half of it.
// The payload has to survive decoding into the response models and come back
// out in the tool result, or the agent asked for data and silently got none.
func TestListReturnsIncludedData(t *testing.T) {
	data := map[string]any{
		"headers":      map[string]any{"x-marker": "hdr-include-test"},
		"body":         map[string]any{"marker": "body-include-test"},
		"path":         "/include-test",
		"parsed_query": map[string]any{"q": "query-include-test"},
	}
	for _, tt := range []struct {
		tool, path, id string
	}{
		{eventsToolName, "/events", "evt_1"},
		{requestsToolName, "/requests", "req_1"},
	} {
		t.Run(tt.tool, func(t *testing.T) {
			session := mockAPIWithClient(t, map[string]http.HandlerFunc{
				hookdeck.APIPathPrefix + tt.path: func(w http.ResponseWriter, r *http.Request) {
					_ = json.NewEncoder(w).Encode(listResponse(map[string]any{"id": tt.id, "data": data}))
				},
			})
			result := callTool(t, session, tt.tool, map[string]any{"action": "list", "include": "data"})
			require.False(t, result.IsError, textContent(t, result))
			text := textContent(t, result)
			for _, marker := range []string{"hdr-include-test", "body-include-test", "/include-test", "query-include-test"} {
				assert.Contains(t, text, marker)
			}
		})
	}
}

// TestIncludeDataDefaultsTheLimit pins the backstop behind the size warning.
// Asking for payloads without a limit gets a small page; an explicit limit is
// honoured; and a list without include is untouched, so the API's own default
// still applies there. Run against both call sites, eventsListing and
// requestsList, because each sets the limit separately.
func TestIncludeDataDefaultsTheLimit(t *testing.T) {
	type sent struct {
		limit   string
		present bool
	}
	cases := []struct {
		name string
		args map[string]any
		want sent
	}{
		{"include without limit sends the default", map[string]any{"action": "list", "include": "data"}, sent{"10", true}},
		{"include with an explicit limit honours it", map[string]any{"action": "list", "include": "data", "limit": float64(25)}, sent{"25", true}},
		{"no include leaves the limit unset", map[string]any{"action": "list"}, sent{"", false}},
	}
	for _, route := range []struct{ tool, path string }{
		{eventsToolName, "/events"},
		{requestsToolName, "/requests"},
	} {
		for _, tc := range cases {
			t.Run(route.tool+"/"+tc.name, func(t *testing.T) {
				var got sent
				called := false
				session := mockAPIWithClient(t, map[string]http.HandlerFunc{
					hookdeck.APIPathPrefix + route.path: func(w http.ResponseWriter, r *http.Request) {
						called = true
						_, got.present = r.URL.Query()["limit"]
						got.limit = r.URL.Query().Get("limit")
						_ = json.NewEncoder(w).Encode(listResponse())
					},
				})
				result := callTool(t, session, route.tool, tc.args)
				require.False(t, result.IsError, textContent(t, result))
				require.True(t, called)
				assert.Equal(t, tc.want, got)
			})
		}
	}
}

// TestAgentIsToldAboutTheIncludeDataDefaultLimit: a default the agent does not
// know about looks like a short result. A list that stops at 10 reads as
// "there are only 10", so the default has to be stated everywhere an agent
// reads about include (the tool description it sees in tools/list, the
// property it fills in, and the help topic it asks for), along with how to get
// more.
func TestAgentIsToldAboutTheIncludeDataDefaultLimit(t *testing.T) {
	session := connectInMemory(t, newTestClient("https://api.hookdeck.com", "test-key"))
	tools := listTools(t, session)
	want := "defaults to " + strconv.Itoa(includeDataDefaultLimit)

	for _, name := range []string{eventsToolName, requestsToolName} {
		t.Run(name, func(t *testing.T) {
			assert.Contains(t, tools[name].Description, want, "tool description")

			desc, _ := schemaProperty(t, tools[name], "include")["description"].(string)
			assert.Contains(t, desc, want, "include property")
			assert.Contains(t, desc, "pass limit", "include property must say how to get more")

			result := callTool(t, session, helpToolName, map[string]any{"topic": name})
			require.False(t, result.IsError, textContent(t, result))
			help := textContent(t, result)
			assert.Contains(t, help, want, "help topic")
			assert.Contains(t, help, "pass limit", "help topic must say how to get more")
			t.Log(help)
		})
	}
}
