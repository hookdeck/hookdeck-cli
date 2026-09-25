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
)

// The Outpost half of the Gateway test of the same name: every array-typed
// argument must deliver a genuine array to the API. A handler that declared an
// array but read it with in.String() would drop it without an error. A new
// array argument fails here until it has an entry with a valid call.
//
// The wanted strings are the encoding the client actually sends, captured from
// a recording server rather than assumed.
var outpostArrayArgs = map[string]struct {
	args map[string]any
	want []string
}{
	"outpost_attempts_read.include": {
		args: map[string]any{"action": "list", "tenant_id": "t1", "include": []any{"event", "destination"}},
		want: []string{"include[0]=event", "include[1]=destination"},
	},
	"outpost_config_write.unset": {
		args: map[string]any{"action": "set", "unset": []any{"MAX_RETRY_LIMIT", "RETRY_INTERVAL_SECONDS"}},
		want: []string{`"MAX_RETRY_LIMIT":null`, `"RETRY_INTERVAL_SECONDS":null`},
	},
	"outpost_destinations_read.topics": {
		args: map[string]any{"action": "list", "tenant_id": "t1", "topics": []any{"qa.a", "qa.b"}},
		want: []string{"topics[0]=qa.a", "topics[1]=qa.b"},
	},
	"outpost_destinations_write.topics": {
		args: map[string]any{"action": "create", "tenant_id": "t1", "type": "webhook",
			"config": map[string]any{"url": "https://example.com/hook"}, "topics": []any{"qa.a", "qa.b"}},
		want: []string{`"topics":["qa.a","qa.b"]`},
	},
	"outpost_metrics_read.measures": {
		args: map[string]any{"action": "events", "start": "2025-01-01T00:00:00Z", "end": "2025-01-02T00:00:00Z",
			"measures": []any{"count", "successful_count"}},
		want: []string{"measures[0]=count", "measures[1]=successful_count"},
	},
	"outpost_metrics_read.dimensions": {
		args: map[string]any{"action": "events", "start": "2025-01-01T00:00:00Z", "end": "2025-01-02T00:00:00Z",
			"measures": []any{"count"}, "dimensions": []any{"tenant_id", "topic"}},
		want: []string{"dimensions[0]=tenant_id", "dimensions[1]=topic"},
	},
}

func TestOutpostArrayArgumentsReachTheAPI(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	api := mockAPI(t, map[string]http.HandlerFunc{
		"/2026-09-01/": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			q, _ := url.QueryUnescape(r.URL.RawQuery)
			mu.Lock()
			seen = append(seen, r.Method+" "+r.URL.Path+"?"+q+" "+string(body))
			mu.Unlock()
			_, _ = w.Write([]byte(`{"id":"x","models":[],"data":[],"pagination":{}}`))
		},
	})
	requests := func() string { mu.Lock(); defer mu.Unlock(); return strings.Join(seen, "\n") }
	session := connect(t, ServerOptions{Client: newTestClient(t, api.URL), WriteEnabled: true, PublishAPIKey: "pk"})

	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	found := map[string]bool{}
	for _, tool := range list.Tools {
		props, _ := tool.InputSchema.(map[string]any)["properties"].(map[string]any)
		for name, prop := range props {
			if p, _ := prop.(map[string]any); p["type"] == "array" {
				found[tool.Name+"."+name] = true
			}
		}
	}
	require.NotEmpty(t, found, "no array arguments found; the walk is broken and the guard would pass vacuously")

	for key := range found {
		call, ok := outpostArrayArgs[key]
		if !assert.True(t, ok, "%s is declared as an array but has no entry in outpostArrayArgs. "+
			"Add a valid call and check the values reach the API.", key) {
			continue
		}
		t.Run(key, func(t *testing.T) {
			before := requests()
			callTool(t, session, strings.SplitN(key, ".", 2)[0], call.args)
			sent := strings.TrimPrefix(requests(), before)
			require.NotEmpty(t, sent, "the call reached no API endpoint")
			for _, w := range call.want {
				assert.Contains(t, sent, w, "an array value did not reach the API")
			}
		})
	}
	for key := range outpostArrayArgs {
		assert.True(t, found[key], "%s is in outpostArrayArgs but is no longer an array argument", key)
	}
}
