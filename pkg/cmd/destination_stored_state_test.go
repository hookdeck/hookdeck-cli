package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// destinationAPI serves the list-by-name and get-by-id pair that upsert uses to
// find the stored destination, so these tests pin the wiring and not just a
// helper. status is applied to every response; 0 means 200.
func destinationAPI(t *testing.T, stored *hookdeck.Destination, status int) *hookdeck.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != 0 {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"message":"boom"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == hookdeck.APIPathPrefix+"/destinations" {
			models := []hookdeck.Destination{}
			if stored != nil {
				models = append(models, *stored)
			}
			_ = json.NewEncoder(w).Encode(hookdeck.DestinationListResponse{Models: models})
			return
		}
		_ = json.NewEncoder(w).Encode(stored)
	}))
	t.Cleanup(srv.Close)

	baseURL, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return &hookdeck.Client{BaseURL: baseURL, APIKey: "k"}
}

// TestDestinationTypeForPolicyCheck pins the guard that `destination update` and
// `destination upsert` were missing: both normally omit --type, so comparing
// rate-limit and delivery-group flags against the flag alone always passed and
// the flags reached a stored CLI destination, where the API discards them.
func TestDestinationTypeForPolicyCheck(t *testing.T) {
	cli := &hookdeck.Destination{ID: "des_1", Name: "local", Type: "CLI"}
	policyFlags := &destinationConfigFlags{RateLimit: 100, RateLimitPeriod: "minute"}
	lookup := func() (*hookdeck.Destination, error) { return cli, nil }

	t.Run("omitted --type resolves to the stored type", func(t *testing.T) {
		got, err := destinationTypeForPolicyCheck("", false, policyFlags, lookup)
		require.NoError(t, err)
		assert.Equal(t, "CLI", got)
	})

	t.Run("an explicit --type is trusted and no lookup happens", func(t *testing.T) {
		got, err := destinationTypeForPolicyCheck("HTTP", false, policyFlags, func() (*hookdeck.Destination, error) {
			t.Fatal("must not spend an API call when --type was given")
			return nil, nil
		})
		require.NoError(t, err)
		assert.Equal(t, "HTTP", got)
	})

	t.Run("no policy flags means no lookup", func(t *testing.T) {
		got, err := destinationTypeForPolicyCheck("", false, &destinationConfigFlags{URL: "https://x"}, func() (*hookdeck.Destination, error) {
			t.Fatal("must not spend an API call when no policy flag is set")
			return nil, nil
		})
		require.NoError(t, err)
		assert.Equal(t, "", got)
	})

	t.Run("--config takes precedence so the individual flags are ignored", func(t *testing.T) {
		got, err := destinationTypeForPolicyCheck("", true, policyFlags, func() (*hookdeck.Destination, error) {
			t.Fatal("must not spend an API call when --config wins anyway")
			return nil, nil
		})
		require.NoError(t, err)
		assert.Equal(t, "", got)
	})

	t.Run("no stored destination leaves the type to the API", func(t *testing.T) {
		got, err := destinationTypeForPolicyCheck("", false, policyFlags, func() (*hookdeck.Destination, error) {
			return nil, nil
		})
		require.NoError(t, err)
		assert.Equal(t, "", got)
	})

	t.Run("a failed lookup is an error, not a pass", func(t *testing.T) {
		_, err := destinationTypeForPolicyCheck("", false, policyFlags, func() (*hookdeck.Destination, error) {
			return nil, errors.New("network down")
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "network down")
	})
}

// TestUpsertRejectsDeliveryPolicyOnStoredCLIDestination walks the whole path the
// command takes: build the config from flags with --type omitted, resolve the
// stored type over the API, then apply the guard.
func TestUpsertRejectsDeliveryPolicyOnStoredCLIDestination(t *testing.T) {
	client := destinationAPI(t, &hookdeck.Destination{ID: "des_1", Name: "local", Type: "CLI"}, 0)
	flags := &destinationConfigFlags{RateLimit: 100, RateLimitPeriod: "minute"}

	// --type omitted, exactly as `destination upsert local --rate-limit 100 ...`.
	config, err := buildDestinationConfigFromFlags("", "", "", flags)
	require.NoError(t, err)
	require.Contains(t, config, "delivery_policy", "the flags do build a policy; the type is what makes it useless")

	policyType, err := destinationTypeForPolicyCheck("", false, flags, func() (*hookdeck.Destination, error) {
		return fetchDestinationByName(context.Background(), client, "local")
	})
	require.NoError(t, err)

	err = rejectDeliveryPolicyInConfigForCLI(policyType, config, "")
	require.Error(t, err, "a stored CLI destination must refuse delivery-policy flags even when --type is omitted")
	assert.Contains(t, err.Error(), "CLI destinations")
}

// TestUpsertAcceptsDeliveryPolicyOnStoredHTTPDestination is the other half.
func TestUpsertAcceptsDeliveryPolicyOnStoredHTTPDestination(t *testing.T) {
	client := destinationAPI(t, &hookdeck.Destination{ID: "des_2", Name: "web", Type: "HTTP"}, 0)
	flags := &destinationConfigFlags{RateLimit: 100, RateLimitPeriod: "minute"}

	config, err := buildDestinationConfigFromFlags("", "", "", flags)
	require.NoError(t, err)

	policyType, err := destinationTypeForPolicyCheck("", false, flags, func() (*hookdeck.Destination, error) {
		return fetchDestinationByName(context.Background(), client, "web")
	})
	require.NoError(t, err)
	assert.Equal(t, "HTTP", policyType)
	assert.NoError(t, rejectDeliveryPolicyInConfigForCLI(policyType, config, ""))
}

// TestFetchDestinationByName pins the (nil, nil) contract the callers rely on to
// tell "no such destination yet" apart from "the lookup failed".
func TestFetchDestinationByName(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		client := destinationAPI(t, &hookdeck.Destination{ID: "des_1", Name: "local", Type: "CLI"}, 0)
		got, err := fetchDestinationByName(context.Background(), client, "local")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "CLI", got.Type)
	})

	t.Run("not found", func(t *testing.T) {
		client := destinationAPI(t, nil, 0)
		got, err := fetchDestinationByName(context.Background(), client, "nope")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("lookup failure", func(t *testing.T) {
		client := destinationAPI(t, nil, http.StatusInternalServerError)
		_, err := fetchDestinationByName(context.Background(), client, "local")
		require.Error(t, err)
	})
}

// TestApplyStoredDestinationConfigFailsClosedOnLookupError pins the second half
// of the #393 fix. The lookup that carries delivery_policy.groups.overrides
// forward used to ignore its own errors: a transient failure left the bare
// groups object in the request, the PUT succeeded, and the stored overrides were
// gone with nothing reporting it.
func TestApplyStoredDestinationConfigFailsClosedOnLookupError(t *testing.T) {
	bareGroups := groupsConfig(map[string]interface{}{
		"key": "body.customer_id", "rate": 30, "rate_period": "second",
	})
	req := &hookdeck.DestinationCreateRequest{Name: "web", Config: bareGroups}

	err := applyStoredDestinationConfig("web", req, func() (*hookdeck.Destination, error) {
		return nil, errors.New("503 Service Unavailable")
	})

	require.Error(t, err, "a bare groups object must not be sent when the stored overrides could not be read")
	assert.Contains(t, err.Error(), "overrides")
	assert.Contains(t, err.Error(), "503 Service Unavailable")
}

// TestApplyStoredDestinationConfigLookupErrorPaths covers the surrounding cases,
// including the partial-update path that may legitimately continue on error.
func TestApplyStoredDestinationConfigLookupErrorPaths(t *testing.T) {
	t.Run("partial update still continues on a lookup failure", func(t *testing.T) {
		req := &hookdeck.DestinationCreateRequest{Name: "web"}
		err := applyStoredDestinationConfig("web", req, func() (*hookdeck.Destination, error) {
			return nil, errors.New("503 Service Unavailable")
		})
		require.NoError(t, err, "there is nothing to destroy here; the API rejecting a config-less PUT is visible")
		assert.Empty(t, req.Config)
	})

	t.Run("stored overrides are carried forward when the lookup succeeds", func(t *testing.T) {
		req := &hookdeck.DestinationCreateRequest{
			Name: "web",
			Config: groupsConfig(map[string]interface{}{
				"key": "body.customer_id", "rate": 30, "rate_period": "second",
			}),
		}
		stored := groupsConfig(map[string]interface{}{
			"key": "body.customer_id", "rate": 10, "rate_period": "second",
			"overrides": storedOverrides(),
		})
		err := applyStoredDestinationConfig("web", req, func() (*hookdeck.Destination, error) {
			return &hookdeck.Destination{ID: "des_2", Name: "web", Type: "HTTP", Config: stored}, nil
		})
		require.NoError(t, err)

		groups, ok := nestedMap(req.Config, "delivery_policy", "groups")
		require.True(t, ok)
		assert.Equal(t, storedOverrides(), groups["overrides"])
		assert.Equal(t, 30, groups["rate"], "the caller's new rate must still win")
	})

	t.Run("a create is not blocked by the absence of a stored destination", func(t *testing.T) {
		req := &hookdeck.DestinationCreateRequest{
			Name: "brand-new",
			Config: groupsConfig(map[string]interface{}{
				"key": "body.customer_id", "rate": 30, "rate_period": "second",
			}),
		}
		err := applyStoredDestinationConfig("brand-new", req, func() (*hookdeck.Destination, error) {
			return nil, nil
		})
		require.NoError(t, err)
	})

	t.Run("a config with its own overrides needs no lookup at all", func(t *testing.T) {
		req := &hookdeck.DestinationCreateRequest{
			Name: "web",
			Config: groupsConfig(map[string]interface{}{
				"key": "body.customer_id", "rate": 30, "rate_period": "second",
				"overrides": map[string]interface{}{"cust_2": map[string]interface{}{"rate": 1}},
			}),
		}
		err := applyStoredDestinationConfig("web", req, func() (*hookdeck.Destination, error) {
			t.Fatal("must not spend an API call when the caller supplied overrides")
			return nil, nil
		})
		require.NoError(t, err)
	})

	t.Run("partial update adopts the stored config and type", func(t *testing.T) {
		req := &hookdeck.DestinationCreateRequest{Name: "local"}
		err := applyStoredDestinationConfig("local", req, func() (*hookdeck.Destination, error) {
			return &hookdeck.Destination{
				ID: "des_1", Name: "local", Type: "CLI",
				Config: map[string]interface{}{"path": "/webhooks"},
			}, nil
		})
		require.NoError(t, err)
		assert.Equal(t, "CLI", req.Type)
		assert.Equal(t, "/webhooks", req.Config["path"])
	})
}

// TestPreserveStoredDeliveryGroupOverrides pins the update path's share of #393.
// `destination update` builds a full groups object from the flags — the CLI
// requires --delivery-group-key and --delivery-group-rate-period whenever
// --delivery-group-rate is given, so "just bump the rate" always sends one — and
// never looked the stored overrides up at all, so the PUT replaced groups
// wholesale and took them with it.
func TestPreserveStoredDeliveryGroupOverrides(t *testing.T) {
	bumpedRate := func() map[string]interface{} {
		return groupsConfig(map[string]interface{}{
			"key": "body.customer_id", "rate": 30, "rate_period": "second",
		})
	}
	stored := func() *hookdeck.Destination {
		return &hookdeck.Destination{
			ID: "des_1", Name: "web", Type: "HTTP",
			Config: groupsConfig(map[string]interface{}{
				"key": "body.customer_id", "rate": 10, "rate_period": "second",
				"overrides": storedOverrides(),
			}),
		}
	}

	t.Run("a rate bump carries the stored overrides forward", func(t *testing.T) {
		config := bumpedRate()
		err := preserveStoredDeliveryGroupOverrides("des_1", config, func() (*hookdeck.Destination, error) {
			return stored(), nil
		})
		require.NoError(t, err)

		groups, ok := nestedMap(config, "delivery_policy", "groups")
		require.True(t, ok)
		assert.Equal(t, storedOverrides(), groups["overrides"], "the stored overrides must survive the PUT")
		assert.Equal(t, 30, groups["rate"], "the caller's new rate must still win")
	})

	t.Run("a failed lookup refuses rather than wiping the overrides", func(t *testing.T) {
		config := bumpedRate()
		err := preserveStoredDeliveryGroupOverrides("des_1", config, func() (*hookdeck.Destination, error) {
			return nil, errors.New("503 Service Unavailable")
		})
		require.Error(t, err, "sending the bare groups object would destroy the overrides and the PUT would succeed")
		assert.Contains(t, err.Error(), "overrides")
		assert.Contains(t, err.Error(), "503 Service Unavailable")
	})

	t.Run("overrides supplied by the caller need no lookup", func(t *testing.T) {
		config := groupsConfig(map[string]interface{}{
			"key": "body.customer_id", "rate": 30, "rate_period": "second",
			"overrides": map[string]interface{}{"cust_2": map[string]interface{}{"rate": 1}},
		})
		err := preserveStoredDeliveryGroupOverrides("des_1", config, func() (*hookdeck.Destination, error) {
			t.Fatal("must not spend an API call when the caller supplied overrides")
			return nil, nil
		})
		require.NoError(t, err)
	})

	t.Run("a config that sets no groups needs no lookup", func(t *testing.T) {
		config := map[string]interface{}{"url": "https://api.example.com"}
		err := preserveStoredDeliveryGroupOverrides("des_1", config, func() (*hookdeck.Destination, error) {
			t.Fatal("must not spend an API call when no delivery group is being sent")
			return nil, nil
		})
		require.NoError(t, err)
	})

	t.Run("a stored destination with no overrides is left alone", func(t *testing.T) {
		config := bumpedRate()
		err := preserveStoredDeliveryGroupOverrides("des_1", config, func() (*hookdeck.Destination, error) {
			return &hookdeck.Destination{ID: "des_1", Name: "web", Type: "HTTP",
				Config: groupsConfig(map[string]interface{}{"key": "body.customer_id"})}, nil
		})
		require.NoError(t, err)

		groups, ok := nestedMap(config, "delivery_policy", "groups")
		require.True(t, ok)
		_, has := groups["overrides"]
		assert.False(t, has, "nothing stored means nothing to carry forward")
	})
}

// TestDestinationUpdateSharesOneLookup pins that the type resolution and the
// overrides preservation reuse a single GET. They are independent guards over
// the same record, and paying twice for it on every rate bump would be a
// regression in its own right.
func TestDestinationUpdateSharesOneLookup(t *testing.T) {
	calls := 0
	lookup := func() (*hookdeck.Destination, error) {
		calls++
		return &hookdeck.Destination{
			ID: "des_1", Name: "web", Type: "HTTP",
			Config: groupsConfig(map[string]interface{}{
				"key": "body.customer_id", "rate": 10, "rate_period": "second",
				"overrides": storedOverrides(),
			}),
		}, nil
	}
	// Memoised exactly as runDestinationUpdateCmd does it.
	var (
		cached  *hookdeck.Destination
		fetched bool
	)
	memoised := func() (*hookdeck.Destination, error) {
		if fetched {
			return cached, nil
		}
		found, err := lookup()
		if err != nil {
			return nil, err
		}
		cached, fetched = found, true
		return found, nil
	}

	flags := &destinationConfigFlags{
		DeliveryGroupKey:        "body.customer_id",
		DeliveryGroupRate:       30,
		DeliveryGroupRatePeriod: "second",
	}
	config, err := buildDestinationConfigFromFlags("", "", "", flags)
	require.NoError(t, err)

	policyType, err := destinationTypeForPolicyCheck("", false, flags, memoised)
	require.NoError(t, err)
	require.NoError(t, rejectDeliveryPolicyInConfigForCLI(policyType, config, ""))
	require.NoError(t, preserveStoredDeliveryGroupOverrides("des_1", config, memoised))

	assert.Equal(t, 1, calls, "both guards must share one GET")
	groups, ok := nestedMap(config, "delivery_policy", "groups")
	require.True(t, ok)
	assert.Equal(t, storedOverrides(), groups["overrides"])
}

// storedDestServer serves GET /destinations (list by name) and
// GET /destinations/{id}, so the builders below exercise the real lookup path.
// failAfter makes every request from the nth onwards fail, which is how a
// transient outage is reproduced.
func storedDestServer(t *testing.T, stored *hookdeck.Destination, failAfter int) (*hookdeck.Client, *int) {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if failAfter > 0 && calls >= failAfter {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"message":"503 Service Unavailable"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == hookdeck.APIPathPrefix+"/destinations" {
			models := []hookdeck.Destination{}
			if stored != nil {
				models = append(models, *stored)
			}
			_ = json.NewEncoder(w).Encode(hookdeck.DestinationListResponse{Models: models})
			return
		}
		_ = json.NewEncoder(w).Encode(stored)
	}))
	t.Cleanup(srv.Close)

	baseURL, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return &hookdeck.Client{BaseURL: baseURL, APIKey: "k"}, &calls
}

func storedHTTPDestWithOverrides() *hookdeck.Destination {
	return &hookdeck.Destination{
		ID: "des_1", Name: "web", Type: "HTTP",
		Config: map[string]interface{}{
			"url": "https://api.example.com",
			"delivery_policy": map[string]interface{}{
				"groups": map[string]interface{}{
					"key": "body.customer_id", "rate": 10, "rate_period": "second",
					"overrides": storedOverrides(),
				},
			},
		},
	}
}

// groupRateBumpFlags is the invocation that always lost the overrides: the CLI
// requires --delivery-group-key and --delivery-group-rate-period whenever
// --delivery-group-rate is given, so bumping the rate always sends a full
// groups object.
// storedOverridesOverTheWire is storedOverrides() as it comes back from JSON,
// where every number is a float64.
func storedOverridesOverTheWire() map[string]interface{} {
	return map[string]interface{}{
		"cust_1": map[string]interface{}{"rate": float64(5), "rate_period": "minute"},
	}
}

func groupRateBumpFlags() destinationConfigFlags {
	return destinationConfigFlags{
		DeliveryGroupKey:        "body.customer_id",
		DeliveryGroupRate:       30,
		DeliveryGroupRatePeriod: "second",
	}
}

// TestDestinationUpdateBuildsRequestPreservingOverrides pins the wiring, not the
// helper: `destination update` never looked the stored overrides up at all, so
// the PUT replaced delivery_policy.groups wholesale and took them with it.
func TestDestinationUpdateBuildsRequestPreservingOverrides(t *testing.T) {
	t.Run("a rate bump carries the stored overrides into the request", func(t *testing.T) {
		client, calls := storedDestServer(t, storedHTTPDestWithOverrides(), 0)
		dc := &destinationUpdateCmd{destinationConfigFlags: groupRateBumpFlags()}

		req, err := dc.buildUpdateRequest(context.Background(), client, "des_1")
		require.NoError(t, err)

		groups, ok := nestedMap(req.Config, "delivery_policy", "groups")
		require.True(t, ok)
		assert.Equal(t, storedOverridesOverTheWire(), groups["overrides"], "the PUT must not drop the stored overrides")
		assert.Equal(t, 30, groups["rate"])
		assert.Equal(t, 1, *calls, "the type guard and the overrides lookup must share one GET")
	})

	t.Run("a failed lookup refuses instead of sending a bare groups object", func(t *testing.T) {
		client, _ := storedDestServer(t, storedHTTPDestWithOverrides(), 1)
		// --type given, so the type guard needs no lookup and the overrides
		// lookup is the only one left to fail. Without --type the command also
		// refuses, but over the type guard's error rather than this one.
		dc := &destinationUpdateCmd{destType: "HTTP", destinationConfigFlags: groupRateBumpFlags()}

		_, err := dc.buildUpdateRequest(context.Background(), client, "des_1")
		require.Error(t, err, "continuing would wipe the overrides and the PUT would still succeed")
		assert.Contains(t, err.Error(), "overrides")
	})

	t.Run("delivery policy flags are refused on a stored CLI destination", func(t *testing.T) {
		client, _ := storedDestServer(t, &hookdeck.Destination{ID: "des_2", Name: "local", Type: "CLI"}, 0)
		dc := &destinationUpdateCmd{destinationConfigFlags: destinationConfigFlags{
			RateLimit: 100, RateLimitPeriod: "minute",
		}}

		// --type omitted, which is the ordinary form of the command.
		_, err := dc.buildUpdateRequest(context.Background(), client, "des_2")
		require.Error(t, err, "the API accepts the request and discards the policy, so the CLI has to refuse it")
		assert.Contains(t, err.Error(), "CLI destinations")
	})

	t.Run("an unrelated update spends no lookup at all", func(t *testing.T) {
		client, calls := storedDestServer(t, storedHTTPDestWithOverrides(), 0)
		dc := &destinationUpdateCmd{description: "just a note"}

		req, err := dc.buildUpdateRequest(context.Background(), client, "des_1")
		require.NoError(t, err)
		require.NotNil(t, req.Description)
		assert.Equal(t, 0, *calls)
	})
}

// TestDestinationUpsertBuildsRequestPreservingOverrides is the upsert half of the
// same wiring. The lookup used to ignore its own errors, so a transient failure
// left the bare groups object in the request and the overrides were destroyed
// with nothing reporting it (#393).
func TestDestinationUpsertBuildsRequestPreservingOverrides(t *testing.T) {
	t.Run("a rate bump carries the stored overrides into the request", func(t *testing.T) {
		client, _ := storedDestServer(t, storedHTTPDestWithOverrides(), 0)
		dc := &destinationUpsertCmd{name: "web", destinationConfigFlags: groupRateBumpFlags()}

		req, err := dc.buildUpsertRequest(context.Background(), client)
		require.NoError(t, err)

		groups, ok := nestedMap(req.Config, "delivery_policy", "groups")
		require.True(t, ok)
		assert.Equal(t, storedOverridesOverTheWire(), groups["overrides"])
	})

	t.Run("a failed lookup refuses instead of sending a bare groups object", func(t *testing.T) {
		// failAfter 2: the list call succeeds and the GET that follows fails.
		client, _ := storedDestServer(t, storedHTTPDestWithOverrides(), 2)
		dc := &destinationUpsertCmd{name: "web", destType: "HTTP", destinationConfigFlags: groupRateBumpFlags()}

		_, err := dc.buildUpsertRequest(context.Background(), client)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "overrides")
	})

	t.Run("delivery policy flags are refused on a stored CLI destination", func(t *testing.T) {
		client, _ := storedDestServer(t, &hookdeck.Destination{ID: "des_2", Name: "local", Type: "CLI"}, 0)
		dc := &destinationUpsertCmd{name: "local", destinationConfigFlags: destinationConfigFlags{
			RateLimit: 100, RateLimitPeriod: "minute",
		}}

		_, err := dc.buildUpsertRequest(context.Background(), client)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CLI destinations")
	})

	t.Run("a partial update still adopts the stored config", func(t *testing.T) {
		client, _ := storedDestServer(t, storedHTTPDestWithOverrides(), 0)
		dc := &destinationUpsertCmd{name: "web", description: "just a note"}

		req, err := dc.buildUpsertRequest(context.Background(), client)
		require.NoError(t, err)
		assert.Equal(t, "HTTP", req.Type)
		assert.Equal(t, "https://api.example.com", req.Config["url"])
	})
}
