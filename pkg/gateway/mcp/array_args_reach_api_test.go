package mcp

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// A tool that declares an argument as an array must deliver a genuine array to
// the API. The input-shape guard makes every read that treats an argument as a
// list go through mcpcore.StringList; it cannot see the reverse -- a handler
// that declares an array but reads it with in.String(), which returns "" for an
// array and drops it without an error.
//
// So every array-typed argument is listed here with a valid call. The test
// sends a real JSON array and requires each value to reach the API. A new array
// argument fails until it has an entry.

type arrayArgCall struct {
	args map[string]any // a valid call, with the array argument set
	want []string       // strings that must appear in the request the API receives
}

var gatewayArrayArgs = map[string]arrayArgCall{
	"gateway_metrics_read.measures": {
		args: map[string]any{"action": "events", "start": "2025-01-01T00:00:00Z", "end": "2025-01-02T00:00:00Z",
			"measures": []any{"count", "successful_count"}},
		want: []string{"measures[]=count", "measures[]=successful_count"},
	},
	"gateway_metrics_read.dimensions": {
		args: map[string]any{"action": "events", "start": "2025-01-01T00:00:00Z", "end": "2025-01-02T00:00:00Z",
			"measures": []any{"count"}, "dimensions": []any{"status", "connection_id"}},
		// connection_id is sent in the API's spelling (#442).
		want: []string{"dimensions[]=status", "dimensions[]=webhook_id"},
	},
	"gateway_request_write.connection_ids": {
		args: map[string]any{"action": "retry", "id": "req_1", "connection_ids": []any{"web_qa_a", "web_qa_b"}},
		want: []string{"web_qa_a", "web_qa_b"},
	},
	"gateway_connections_write.rules": {
		args: map[string]any{"action": "update", "id": "web_1", "rules": []any{
			map[string]any{"type": "filter", "headers": map[string]any{"x-qa": "qa_marker_a"}},
			map[string]any{"type": "filter", "body": map[string]any{"qa": "qa_marker_b"}},
		}},
		want: []string{"qa_marker_a", "qa_marker_b"},
	},
}

// recordingAPI answers every API request and keeps its query and body.
func recordingAPI(t *testing.T) (map[string]http.HandlerFunc, func() string) {
	var mu sync.Mutex
	var seen []string
	handler := func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		q, _ := url.QueryUnescape(r.URL.RawQuery)
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path+"?"+q+" "+string(body))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"web_1","models":[],"pagination":{},"data":[]}`))
	}
	return map[string]http.HandlerFunc{hookdeck.APIPathPrefix + "/": handler}, func() string {
		mu.Lock()
		defer mu.Unlock()
		return strings.Join(seen, "\n")
	}
}

func arrayArgumentsOf(t *testing.T, tools []toolSchema) map[string]bool {
	found := map[string]bool{}
	for _, tool := range tools {
		for name, prop := range tool.props {
			if p, _ := prop.(map[string]any); p["type"] == "array" {
				found[tool.name+"."+name] = true
			}
		}
	}
	return found
}

type toolSchema struct {
	name  string
	props map[string]any
}

func TestGatewayArrayArgumentsReachTheAPI(t *testing.T) {
	handlers, requests := recordingAPI(t)
	session := mockAPIWithClientWriteEnabled(t, handlers)

	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	var tools []toolSchema
	for _, tool := range list.Tools {
		props, _ := tool.InputSchema.(map[string]any)["properties"].(map[string]any)
		tools = append(tools, toolSchema{tool.Name, props})
	}
	found := arrayArgumentsOf(t, tools)
	require.NotEmpty(t, found, "no array arguments found; the walk is broken and the guard would pass vacuously")

	for key := range found {
		call, ok := gatewayArrayArgs[key]
		if !assert.True(t, ok, "%s is declared as an array but has no entry in gatewayArrayArgs. "+
			"Add a valid call and check the values reach the API.", key) {
			continue
		}
		t.Run(key, func(t *testing.T) {
			before := requests()
			result := callTool(t, session, strings.SplitN(key, ".", 2)[0], call.args)
			sent := strings.TrimPrefix(requests(), before)
			require.NotEmpty(t, sent, "the call reached no API endpoint: %s", fmtResult(t, result))
			for _, w := range call.want {
				assert.Contains(t, sent, w, "an array value did not reach the API")
			}
		})
	}
	for key := range gatewayArrayArgs {
		assert.True(t, found[key], "%s is in gatewayArrayArgs but is no longer an array argument", key)
	}
}

func fmtResult(t *testing.T, r *mcpsdk.CallToolResult) string {
	t.Helper()
	if r == nil || len(r.Content) == 0 {
		return ""
	}
	return textContent(t, r)
}
