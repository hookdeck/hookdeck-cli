package hookdeck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// With include=data a list carries one payload per record, decoded into
// EventData / RequestData. The spec allows headers and parsed_query to be a
// string as well as an object, so a record in the string form must decode
// rather than fail the whole page. is_large_payload must survive too: it is how
// a caller tells a withheld body from an empty one.
const listWithMixedDataShapes = `{"models":[
  {"id":"rec_object","data":{"headers":{"x-a":"1"},"parsed_query":{"q":"1"},"body":{"k":"v"},"path":"/a"}},
  {"id":"rec_string","data":{"headers":"x-a: 1","parsed_query":"q=1","body":"raw","path":"/b","query":"q=1","is_large_payload":true}}
],"pagination":{"limit":10}}`

func listDataServer(t *testing.T) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(listWithMixedDataShapes))
	}))
	t.Cleanup(srv.Close)
	base, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return &Client{BaseURL: base}
}

func TestListEventsDecodesEveryDataShape(t *testing.T) {
	resp, err := listDataServer(t).ListEvents(context.Background(), map[string]string{"include": "data"})
	require.NoError(t, err, "a string-form headers or parsed_query must not fail the page")
	require.Len(t, resp.Models, 2)

	assert.Equal(t, map[string]any{"x-a": "1"}, resp.Models[0].Data.Headers)
	assert.Equal(t, "x-a: 1", resp.Models[1].Data.Headers)
	assert.Equal(t, "q=1", resp.Models[1].Data.ParsedQuery)
	require.NotNil(t, resp.Models[1].Data.IsLargePayload)
	assert.True(t, *resp.Models[1].Data.IsLargePayload)
}

func TestListRequestsDecodesEveryDataShape(t *testing.T) {
	resp, err := listDataServer(t).ListRequests(context.Background(), map[string]string{"include": "data"})
	require.NoError(t, err, "a string-form headers or parsed_query must not fail the page")
	require.Len(t, resp.Models, 2)

	assert.Equal(t, map[string]any{"x-a": "1"}, resp.Models[0].Data.Headers)
	assert.Equal(t, "x-a: 1", resp.Models[1].Data.Headers)
	assert.Equal(t, "q=1", resp.Models[1].Data.ParsedQuery)
	require.NotNil(t, resp.Models[1].Data.IsLargePayload)
	assert.True(t, *resp.Models[1].Data.IsLargePayload)
}
