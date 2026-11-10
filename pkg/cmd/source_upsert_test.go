package cmd

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// PUT /sources validates with the API's source create schema, where a missing type defaults to
// WEBHOOK (hookdeck/core server/api/schemas/request/source.ts). So `source upsert` without
// --type must send an existing source's stored type, or the upsert converts it to WEBHOOK and
// drops its verification.

// upsertStub answers the name lookup with existing (nil: no source has the name) and the PUT
// with the type the request carried, defaulting to WEBHOOK as the API does. It records the
// lookup's query string.
func upsertStub(existing map[string]interface{}, lookups *[]url.Values) func(r *http.Request) interface{} {
	return func(r *http.Request) interface{} {
		if r.Method == http.MethodGet {
			*lookups = append(*lookups, r.URL.Query())
			models := []interface{}{}
			if existing != nil {
				models = append(models, existing)
			}
			return map[string]interface{}{"models": models, "pagination": map[string]interface{}{"limit": 100}}
		}
		return map[string]interface{}{"id": "src_1", "name": "my-stripe", "type": "WEBHOOK"}
	}
}

func existingStripeSource() map[string]interface{} {
	return map[string]interface{}{"id": "src_1", "name": "my-stripe", "type": "STRIPE"}
}

func TestSourceUpsertWithoutTypeKeepsExistingType(t *testing.T) {
	var lookups []url.Values
	requests, err := runGatewayAgainstStub(t, upsertStub(existingStripeSource(), &lookups),
		"source", "upsert", "my-stripe", "--description", "x")
	require.NoError(t, err)

	require.Len(t, lookups, 1, "the existing source must be looked up when --type is omitted")
	assert.Equal(t, "my-stripe", lookups[0].Get("name"))
	// A disabled source is still upserted by name, so the lookup must see it.
	assert.Equal(t, "true", lookups[0].Get("disabled"))

	put := findRequest(t, requests, http.MethodPut, hookdeck.APIPathPrefix+"/sources")
	assert.Equal(t, "STRIPE", put.Body["type"])
	assert.Equal(t, "x", put.Body["description"])
	// No config.auth: the API keeps the stored verification only when the type is unchanged
	// and the request carries no auth.
	assert.NotContains(t, put.Body, "config")
}

// Auth flags without --type are shaped for the stored type: a STRIPE secret is a bare
// webhook_secret_key, not the WEBHOOK HMAC shape built for a new source.
func TestSourceUpsertWithoutTypeShapesAuthForExistingType(t *testing.T) {
	var lookups []url.Values
	requests, err := runGatewayAgainstStub(t, upsertStub(existingStripeSource(), &lookups),
		"source", "upsert", "my-stripe", "--webhook-secret", "whsec_new")
	require.NoError(t, err)

	put := findRequest(t, requests, http.MethodPut, hookdeck.APIPathPrefix+"/sources")
	assert.Equal(t, "STRIPE", put.Body["type"])
	cfg := bodyMap(t, put.Body, "config")
	assert.Equal(t, map[string]interface{}{"webhook_secret_key": "whsec_new"}, cfg["auth"])
}

// No source has the name: send no type, so the API creates a WEBHOOK source as before.
func TestSourceUpsertWithoutTypeCreatesWithoutType(t *testing.T) {
	var lookups []url.Values
	requests, err := runGatewayAgainstStub(t, upsertStub(nil, &lookups), "source", "upsert", "new-source")
	require.NoError(t, err)

	require.Len(t, lookups, 1)
	put := findRequest(t, requests, http.MethodPut, hookdeck.APIPathPrefix+"/sources")
	assert.NotContains(t, put.Body, "type")
}

// The name is matched exactly on the client too: a source listed under another name is not
// the one being upserted.
func TestSourceUpsertWithoutTypeIgnoresOtherNames(t *testing.T) {
	var lookups []url.Values
	other := map[string]interface{}{"id": "src_2", "name": "my-stripe-2", "type": "STRIPE"}
	requests, err := runGatewayAgainstStub(t, upsertStub(other, &lookups), "source", "upsert", "my-stripe")
	require.NoError(t, err)

	put := findRequest(t, requests, http.MethodPut, hookdeck.APIPathPrefix+"/sources")
	assert.NotContains(t, put.Body, "type")
}

// An explicit --type is sent as given (upper-cased), with no lookup.
func TestSourceUpsertWithTypeSkipsLookup(t *testing.T) {
	var lookups []url.Values
	requests, err := runGatewayAgainstStub(t, upsertStub(existingStripeSource(), &lookups),
		"source", "upsert", "my-stripe", "--type", "stripe")
	require.NoError(t, err)

	assert.Empty(t, lookups)
	put := findRequest(t, requests, http.MethodPut, hookdeck.APIPathPrefix+"/sources")
	assert.Equal(t, "STRIPE", put.Body["type"])
}

// If the lookup fails, do not fall back to a typeless upsert: that is the request that
// converts the source.
func TestSourceUpsertWithoutTypeFailsWhenLookupFails(t *testing.T) {
	requests, err := runGatewayAgainstStub(t,
		func(r *http.Request) interface{} {
			return stubStatus{Code: http.StatusInternalServerError, Body: map[string]interface{}{"message": "stub failure"}}
		},
		"source", "upsert", "my-stripe", "--description", "x",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--type")
	for _, r := range requests {
		assert.NotEqual(t, http.MethodPut, r.Method, "no upsert may be sent after a failed lookup")
	}
}
