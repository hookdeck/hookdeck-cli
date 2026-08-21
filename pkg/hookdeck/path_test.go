package hookdeck

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPathSegment(t *testing.T) {
	t.Parallel()

	t.Run("rejects values that would change the request target", func(t *testing.T) {
		for _, id := range []string{"", ".", "..", "a/b", "../evil", `a\b`} {
			_, err := pathSegment(id)
			require.Error(t, err, "%q must be rejected", id)
			assert.ErrorIs(t, err, ErrInvalidResourceID)
		}
	})

	t.Run("names the offending value", func(t *testing.T) {
		_, err := pathSegment("src_1/../des_2")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "src_1/../des_2")
	})

	t.Run("escapes everything else", func(t *testing.T) {
		got, err := pathSegment("acme prod?x")
		require.NoError(t, err)
		assert.Equal(t, "acme%20prod%3Fx", got)

		// Percent-encoded traversal must not survive a second decode.
		got, err = pathSegment("%2e%2e")
		require.NoError(t, err)
		assert.Equal(t, "%252e%252e", got)
	})
}

// validID is the stand-in for a well-formed identifier in the table below.
const validID = "res_1"

// otherID fills the identifier positions a case is not exercising.
const otherID = "other_1"

// byIDCall is one client method that interpolates a caller-supplied identifier
// into a request path.
type byIDCall struct {
	name string

	// call invokes the method, putting id in the identifier position under test.
	call func(context.Context, *Client, string) error

	// wantPath is the path the method must emit for a well-formed id.
	wantPath func(id string) string

	// skipEmpty marks the positions where an empty value legitimately means
	// "not supplied" and selects a differently-shaped endpoint.
	skipEmpty bool
}

// byIDCalls covers every client method that puts a caller-supplied identifier
// into a URL path. A method missing from here is a method whose identifier is
// not being checked, so add to this list when adding an endpoint.
func byIDCalls() []byIDCall {
	// at builds the expected path, substituting the id under test for "%s".
	at := func(segments ...string) func(string) string {
		return func(id string) string {
			parts := make([]string, 0, len(segments))
			for _, s := range segments {
				if s == "%s" {
					s = id
				}
				parts = append(parts, s)
			}
			return APIPathPrefix + "/" + strings.Join(parts, "/")
		}
	}

	return []byIDCall{
		// Sources
		{"GetSource", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetSource(ctx, id, nil)
			return err
		}, at("sources", "%s"), false},
		{"UpdateSource", func(ctx context.Context, c *Client, id string) error {
			_, err := c.UpdateSource(ctx, id, &SourceUpdateRequest{})
			return err
		}, at("sources", "%s"), false},
		{"DeleteSource", func(ctx context.Context, c *Client, id string) error {
			return c.DeleteSource(ctx, id)
		}, at("sources", "%s"), false},
		{"EnableSource", func(ctx context.Context, c *Client, id string) error {
			_, err := c.EnableSource(ctx, id)
			return err
		}, at("sources", "%s", "enable"), false},
		{"DisableSource", func(ctx context.Context, c *Client, id string) error {
			_, err := c.DisableSource(ctx, id)
			return err
		}, at("sources", "%s", "disable"), false},

		// Destinations
		{"GetDestination", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetDestination(ctx, id, nil)
			return err
		}, at("destinations", "%s"), false},
		{"UpdateDestination", func(ctx context.Context, c *Client, id string) error {
			_, err := c.UpdateDestination(ctx, id, &DestinationUpdateRequest{})
			return err
		}, at("destinations", "%s"), false},
		{"DeleteDestination", func(ctx context.Context, c *Client, id string) error {
			return c.DeleteDestination(ctx, id)
		}, at("destinations", "%s"), false},
		{"EnableDestination", func(ctx context.Context, c *Client, id string) error {
			_, err := c.EnableDestination(ctx, id)
			return err
		}, at("destinations", "%s", "enable"), false},
		{"DisableDestination", func(ctx context.Context, c *Client, id string) error {
			_, err := c.DisableDestination(ctx, id)
			return err
		}, at("destinations", "%s", "disable"), false},

		// Connections
		{"GetConnection", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetConnection(ctx, id)
			return err
		}, at("connections", "%s"), false},
		{"UpdateConnection", func(ctx context.Context, c *Client, id string) error {
			_, err := c.UpdateConnection(ctx, id, &ConnectionCreateRequest{})
			return err
		}, at("connections", "%s"), false},
		{"DeleteConnection", func(ctx context.Context, c *Client, id string) error {
			return c.DeleteConnection(ctx, id)
		}, at("connections", "%s"), false},
		{"EnableConnection", func(ctx context.Context, c *Client, id string) error {
			_, err := c.EnableConnection(ctx, id)
			return err
		}, at("connections", "%s", "enable"), false},
		{"DisableConnection", func(ctx context.Context, c *Client, id string) error {
			_, err := c.DisableConnection(ctx, id)
			return err
		}, at("connections", "%s", "disable"), false},
		{"PauseConnection", func(ctx context.Context, c *Client, id string) error {
			_, err := c.PauseConnection(ctx, id)
			return err
		}, at("connections", "%s", "pause"), false},
		{"UnpauseConnection", func(ctx context.Context, c *Client, id string) error {
			_, err := c.UnpauseConnection(ctx, id)
			return err
		}, at("connections", "%s", "unpause"), false},

		// Transformations
		{"GetTransformation", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetTransformation(ctx, id)
			return err
		}, at("transformations", "%s"), false},
		{"UpdateTransformation", func(ctx context.Context, c *Client, id string) error {
			_, err := c.UpdateTransformation(ctx, id, &TransformationUpdateRequest{})
			return err
		}, at("transformations", "%s"), false},
		{"DeleteTransformation", func(ctx context.Context, c *Client, id string) error {
			return c.DeleteTransformation(ctx, id)
		}, at("transformations", "%s"), false},
		{"ListTransformationExecutions", func(ctx context.Context, c *Client, id string) error {
			_, err := c.ListTransformationExecutions(ctx, id, nil)
			return err
		}, at("transformations", "%s", "executions"), false},
		{"GetTransformationExecution/transformation", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetTransformationExecution(ctx, id, otherID)
			return err
		}, at("transformations", "%s", "executions", otherID), false},
		{"GetTransformationExecution/execution", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetTransformationExecution(ctx, otherID, id)
			return err
		}, at("transformations", otherID, "executions", "%s"), false},

		// Issues
		{"GetIssue", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetIssue(ctx, id)
			return err
		}, at("issues", "%s"), false},
		{"UpdateIssue", func(ctx context.Context, c *Client, id string) error {
			_, err := c.UpdateIssue(ctx, id, &IssueUpdateRequest{})
			return err
		}, at("issues", "%s"), false},
		{"DismissIssue", func(ctx context.Context, c *Client, id string) error {
			_, err := c.DismissIssue(ctx, id)
			return err
		}, at("issues", "%s"), false},

		// Events
		{"GetEvent", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetEvent(ctx, id, nil)
			return err
		}, at("events", "%s"), false},
		{"RetryEvent", func(ctx context.Context, c *Client, id string) error {
			_, err := c.RetryEvent(ctx, id)
			return err
		}, at("events", "%s", "retry"), false},
		{"CancelEvent", func(ctx context.Context, c *Client, id string) error {
			_, err := c.CancelEvent(ctx, id)
			return err
		}, at("events", "%s", "cancel"), false},
		{"MuteEvent", func(ctx context.Context, c *Client, id string) error {
			_, err := c.MuteEvent(ctx, id)
			return err
		}, at("events", "%s", "mute"), false},
		{"GetEventRawBody", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetEventRawBody(ctx, id)
			return err
		}, at("events", "%s", "raw_body"), false},

		// Requests
		{"GetRequest", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetRequest(ctx, id, nil)
			return err
		}, at("requests", "%s"), false},
		{"RetryRequest", func(ctx context.Context, c *Client, id string) error {
			_, err := c.RetryRequest(ctx, id, nil)
			return err
		}, at("requests", "%s", "retry"), false},
		{"GetRequestEvents", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetRequestEvents(ctx, id, nil)
			return err
		}, at("requests", "%s", "events"), false},
		{"GetRequestIgnoredEvents", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetRequestIgnoredEvents(ctx, id, nil)
			return err
		}, at("requests", "%s", "ignored_events"), false},
		{"GetRequestRawBody", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetRequestRawBody(ctx, id)
			return err
		}, at("requests", "%s", "raw_body"), false},

		// Attempts
		{"GetAttempt", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetAttempt(ctx, id)
			return err
		}, at("attempts", "%s"), false},

		// CLI clients
		{"UpdateClient", func(_ context.Context, c *Client, id string) error {
			return c.UpdateClient(id, UpdateClientInput{})
		}, at("cli", "%s"), false},

		// Outpost tenants
		{"GetOutpostTenant", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetOutpostTenant(ctx, id)
			return err
		}, at("tenants", "%s"), false},
		{"UpsertOutpostTenant", func(ctx context.Context, c *Client, id string) error {
			_, err := c.UpsertOutpostTenant(ctx, id, &OutpostTenantUpsertRequest{})
			return err
		}, at("tenants", "%s"), false},
		{"DeleteOutpostTenant", func(ctx context.Context, c *Client, id string) error {
			return c.DeleteOutpostTenant(ctx, id)
		}, at("tenants", "%s"), false},
		{"GetOutpostTenantToken", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetOutpostTenantToken(ctx, id)
			return err
		}, at("tenants", "%s", "token"), false},
		{"GetOutpostTenantPortalURL", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetOutpostTenantPortalURL(ctx, id, "")
			return err
		}, at("tenants", "%s", "portal"), false},

		// Outpost destinations
		{"ListOutpostDestinations", func(ctx context.Context, c *Client, id string) error {
			_, err := c.ListOutpostDestinations(ctx, id, nil, nil)
			return err
		}, at("tenants", "%s", "destinations"), false},
		{"CreateOutpostDestination", func(ctx context.Context, c *Client, id string) error {
			_, err := c.CreateOutpostDestination(ctx, id, &OutpostDestinationCreateRequest{})
			return err
		}, at("tenants", "%s", "destinations"), false},
		{"GetOutpostDestination/tenant", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetOutpostDestination(ctx, id, otherID)
			return err
		}, at("tenants", "%s", "destinations", otherID), false},
		{"GetOutpostDestination/destination", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetOutpostDestination(ctx, otherID, id)
			return err
		}, at("tenants", otherID, "destinations", "%s"), false},
		{"UpdateOutpostDestination/tenant", func(ctx context.Context, c *Client, id string) error {
			_, err := c.UpdateOutpostDestination(ctx, id, otherID, &OutpostDestinationUpdateRequest{})
			return err
		}, at("tenants", "%s", "destinations", otherID), false},
		{"UpdateOutpostDestination/destination", func(ctx context.Context, c *Client, id string) error {
			_, err := c.UpdateOutpostDestination(ctx, otherID, id, &OutpostDestinationUpdateRequest{})
			return err
		}, at("tenants", otherID, "destinations", "%s"), false},
		{"DeleteOutpostDestination/tenant", func(ctx context.Context, c *Client, id string) error {
			return c.DeleteOutpostDestination(ctx, id, otherID)
		}, at("tenants", "%s", "destinations", otherID), false},
		{"DeleteOutpostDestination/destination", func(ctx context.Context, c *Client, id string) error {
			return c.DeleteOutpostDestination(ctx, otherID, id)
		}, at("tenants", otherID, "destinations", "%s"), false},
		{"EnableOutpostDestination", func(ctx context.Context, c *Client, id string) error {
			_, err := c.EnableOutpostDestination(ctx, otherID, id)
			return err
		}, at("tenants", otherID, "destinations", "%s", "enable"), false},
		{"DisableOutpostDestination", func(ctx context.Context, c *Client, id string) error {
			_, err := c.DisableOutpostDestination(ctx, otherID, id)
			return err
		}, at("tenants", otherID, "destinations", "%s", "disable"), false},

		// Outpost events, attempts, destination types, publish
		{"GetOutpostEvent", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetOutpostEvent(ctx, id, "")
			return err
		}, at("events", "%s"), false},
		{"GetOutpostAttempt", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetOutpostAttempt(ctx, id, OutpostAttemptGetParams{})
			return err
		}, at("attempts", "%s"), false},
		{"GetOutpostAttempt/tenant", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetOutpostAttempt(ctx, validID, OutpostAttemptGetParams{TenantID: id, DestinationID: otherID})
			return err
		}, at("tenants", "%s", "destinations", otherID, "attempts", validID), true},
		{"GetOutpostAttempt/destination", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetOutpostAttempt(ctx, validID, OutpostAttemptGetParams{TenantID: otherID, DestinationID: id})
			return err
		}, at("tenants", otherID, "destinations", "%s", "attempts", validID), true},
		{"ListOutpostAttempts/tenant", func(ctx context.Context, c *Client, id string) error {
			_, err := c.ListOutpostAttempts(ctx, OutpostAttemptListParams{TenantID: id, DestinationID: otherID})
			return err
		}, at("tenants", "%s", "destinations", otherID, "attempts"), true},
		{"ListOutpostAttempts/destination", func(ctx context.Context, c *Client, id string) error {
			_, err := c.ListOutpostAttempts(ctx, OutpostAttemptListParams{TenantID: otherID, DestinationID: id})
			return err
		}, at("tenants", otherID, "destinations", "%s", "attempts"), true},
		{"GetOutpostDestinationType", func(ctx context.Context, c *Client, id string) error {
			_, err := c.GetOutpostDestinationType(ctx, id)
			return err
		}, at("destination-types", "%s"), false},
		{"TenantExistsForPublish", func(ctx context.Context, c *Client, id string) error {
			_, err := c.TenantExistsForPublish(ctx, "project-api-key", id)
			return err
		}, at("tenants", "%s"), true},
	}
}

// TestByIDCallsRejectTraversingIdentifiers is the test that stops a call site
// regressing to raw string concatenation.
//
// Resolving a path against the base URL normalises "." and ".." segments, so an
// unvalidated identifier can move the request to a different resource — of a
// different type — while the caller reports the identifier it was handed. Every
// method that takes an identifier must refuse those values before any request
// is sent.
func TestByIDCallsRejectTraversingIdentifiers(t *testing.T) {
	t.Parallel()

	bad := []struct {
		name  string
		id    string
		empty bool
	}{
		{"parent directory", "..", false},
		{"embedded traversal", "a/../b", false},
		{"current directory", ".", false},
		{"backslash separator", `a\b`, false},
		{"empty", "", true},
	}

	for _, call := range byIDCalls() {
		for _, tc := range bad {
			if tc.empty && call.skipEmpty {
				continue
			}

			t.Run(call.name+"/"+tc.name, func(t *testing.T) {
				called := false
				client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
					called = true
					_, _ = w.Write([]byte(`{}`))
				})
				defer server.Close()

				err := call.call(context.Background(), client, tc.id)

				require.Error(t, err, "%q must be rejected", tc.id)
				assert.ErrorIs(t, err, ErrInvalidResourceID)
				assert.False(t, called, "no request may reach the API")
			})
		}
	}
}

// TestByIDCallsEmitTheExpectedPath is the other half of the table: rejecting
// bad identifiers is only useful if good ones still address the right endpoint.
func TestByIDCallsEmitTheExpectedPath(t *testing.T) {
	t.Parallel()

	for _, call := range byIDCalls() {
		t.Run(call.name, func(t *testing.T) {
			var gotPath string
			client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				_, _ = w.Write([]byte(`{}`))
			})
			defer server.Close()

			// Response bodies differ per endpoint, so a decode failure is not
			// interesting here — only the path, and that neither validation
			// layer fired, are.
			err := call.call(context.Background(), client, validID)
			assert.NotErrorIs(t, err, ErrInvalidResourceID)
			assert.NotErrorIs(t, err, ErrRequestPathRejected)

			assert.Equal(t, call.wantPath(validID), gotPath)
		})
	}
}

// TestRequestLayerBackstopRejectsRewrittenPaths proves the second layer works
// on its own, so a call site added later without apiPath still cannot send a
// request that addresses something other than what it names.
func TestRequestLayerBackstopRejectsRewrittenPaths(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		path string
	}{
		{"traversal retargets another resource type", APIPathPrefix + "/sources/src_1/../../destinations/des_2"},
		{"traversal truncates to the API root", APIPathPrefix + "/sources/.."},
		{"single dot segment", APIPathPrefix + "/sources/./src_1"},
		{"escapes the version prefix entirely", "/../admin"},
		{"outside the version prefix", "/2020-01-01/sources/src_1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
				called = true
			})
			defer server.Close()

			// Built by hand rather than through a client method, so only the
			// request layer can reject it.
			_, err := client.Get(context.Background(), tc.path, "", nil)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrRequestPathRejected)
			assert.NotErrorIs(t, err, ErrInvalidResourceID, "the two layers must be distinguishable")
			assert.False(t, called, "no request may reach the API")

			_, err = client.newRequest(context.Background(), http.MethodDelete, tc.path, nil)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrRequestPathRejected)
		})
	}

	t.Run("a well-formed path is left alone", func(t *testing.T) {
		var gotPath string
		client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			_, _ = w.Write([]byte(`{}`))
		})
		defer server.Close()

		_, err := client.Get(context.Background(), APIPathPrefix+"/sources/src_1", "", nil)
		require.NoError(t, err)
		assert.Equal(t, APIPathPrefix+"/sources/src_1", gotPath)
	})
}

// TestIdentifiersNeedingEscapingAreSentEscaped covers the identifiers that are
// chosen by the operator rather than generated by the API — tenant IDs above
// all — which may legitimately contain characters that have meaning in a URL.
func TestIdentifiersNeedingEscapingAreSentEscaped(t *testing.T) {
	t.Parallel()

	var gotEscaped, gotDecoded, gotQuery string
	client, server := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		gotEscaped, gotDecoded, gotQuery = r.URL.EscapedPath(), r.URL.Path, r.URL.RawQuery
		_, _ = w.Write([]byte(`{"id":"acme prod?x"}`))
	})
	defer server.Close()

	tenant, err := client.GetOutpostTenant(context.Background(), "acme prod?x")
	require.NoError(t, err)

	assert.Equal(t, APIPathPrefix+"/tenants/acme%20prod%3Fx", gotEscaped)
	// The server decodes back to exactly the id that was asked for, and the "?"
	// stays in the path rather than starting a query string.
	assert.Equal(t, APIPathPrefix+"/tenants/acme prod?x", gotDecoded)
	assert.Empty(t, gotQuery)
	assert.Equal(t, "acme prod?x", tenant.ID)
}

// TestBackstopErrorIsDistinctFromValidationError guards the property the other
// tests rely on to tell the layers apart.
func TestBackstopErrorIsDistinctFromValidationError(t *testing.T) {
	t.Parallel()

	assert.False(t, errors.Is(ErrRequestPathRejected, ErrInvalidResourceID))
	assert.False(t, errors.Is(ErrInvalidResourceID, ErrRequestPathRejected))
}
