package hookdeck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// `--id evt_A,evt_B` sent the comma-joined string as one scalar `id`, which the
// API matched against nothing: zero rows, exit 0, while each id on its own
// returned its row (#411). The API takes a list as repeated `id[]` values --
// verified against the live API on event, request and transformation list
// before this fix was written, rather than inferred from another endpoint.
func TestListQueryExpandsCommaSeparatedIDs(t *testing.T) {
	q := listQuery(map[string]string{"id": "evt_A,evt_B"})

	assert.Equal(t, []string{"evt_A", "evt_B"}, q["id[]"], "each id becomes its own id[] value")
	assert.Empty(t, q["id"], "the comma-joined scalar must not be sent")
}

func TestListQueryLeavesOtherShapesAlone(t *testing.T) {
	for _, tt := range []struct {
		name   string
		params map[string]string
		want   url.Values
	}{
		{"a single id is sent exactly as before", map[string]string{"id": "evt_A"},
			url.Values{"id": {"evt_A"}}},
		{"spaces around commas are trimmed", map[string]string{"id": "evt_A, evt_B"},
			url.Values{"id[]": {"evt_A", "evt_B"}}},
		{"empty items are dropped", map[string]string{"id": "evt_A,,evt_B,"},
			url.Values{"id[]": {"evt_A", "evt_B"}}},
		// Only filters documented as lists are expanded. A comma elsewhere is
		// data, and splitting it would be a different silent-wrong-answer bug.
		{"a comma in a non-list filter is passed through", map[string]string{"search_term": "a,b"},
			url.Values{"search_term": {"a,b"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, listQuery(tt.params))
		})
	}
}

// The unit tests above cover the builder; this covers ListEvents actually using
// it. Without a test at this level, reverting ListEvents to the old loop leaves
// the builder tests green while `--id A,B` goes back to returning nothing.
func TestListEventsSendsIDsAsAList(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{}, "pagination": map[string]any{}})
	}))
	defer srv.Close()

	base, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c := &Client{BaseURL: base}

	_, err = c.ListEvents(context.Background(), map[string]string{"id": "evt_A,evt_B"})
	require.NoError(t, err)

	assert.Equal(t, []string{"evt_A", "evt_B"}, got["id[]"])
	assert.Empty(t, got["id"])
}

// delivery_group is declared exactly like id in the API -- a single value or an
// array, no comma splitting -- so "a,b" as one value matched nothing and
// --delivery-group a,b returned zero rows with exit 0, the #411 shape. Found by
// the guard over flags documented as comma-separated.
func TestListQueryExpandsCommaSeparatedDeliveryGroups(t *testing.T) {
	q := listQuery(map[string]string{"delivery_group": "cus_1,cus_2"})
	assert.Equal(t, []string{"cus_1", "cus_2"}, q["delivery_group[]"])
	assert.Empty(t, q["delivery_group"])
}

// GetRequestEvents built its own query and so was missed by the #411 fix. This
// covers it using the shared builder, for both list-valued filters.
func TestGetRequestEventsSendsListValuedFiltersAsLists(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{}, "pagination": map[string]any{}})
	}))
	defer srv.Close()

	base, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c := &Client{BaseURL: base}

	_, err = c.GetRequestEvents(context.Background(), "req_1", map[string]string{
		"id":             "evt_A,evt_B",
		"delivery_group": "cus_1,cus_2",
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"evt_A", "evt_B"}, got["id[]"])
	assert.Equal(t, []string{"cus_1", "cus_2"}, got["delivery_group[]"])
	assert.Empty(t, got["delivery_group"])
}
