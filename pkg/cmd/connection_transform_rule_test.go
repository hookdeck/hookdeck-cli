package cmd

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// fakeTransformations serves GetTransformation by ID and ListTransformations by
// name the way the API does: list may return near matches, so callers must
// compare names exactly.
type fakeTransformations struct {
	byID    map[string]hookdeck.Transformation
	listErr error
	calls   int
}

func (f *fakeTransformations) GetTransformation(_ context.Context, id string) (*hookdeck.Transformation, error) {
	f.calls++
	if trn, ok := f.byID[id]; ok {
		return &trn, nil
	}
	return nil, &hookdeck.APIError{StatusCode: 404, Message: "not found"}
}

func (f *fakeTransformations) ListTransformations(_ context.Context, params map[string]string) (*hookdeck.TransformationListResponse, error) {
	f.calls++
	if f.listErr != nil {
		return nil, f.listErr
	}
	resp := &hookdeck.TransformationListResponse{}
	for _, trn := range f.byID {
		if len(params["name"]) > 0 && len(trn.Name) >= len(params["name"]) && trn.Name[:len(params["name"])] == params["name"] {
			resp.Models = append(resp.Models, trn)
		}
	}
	return resp, nil
}

func newFakeTransformations() *fakeTransformations {
	return &fakeTransformations{byID: map[string]hookdeck.Transformation{
		"trs_abc123": {ID: "trs_abc123", Name: "orders-transform"},
	}}
}

func buildTransformRules(t *testing.T, f *connectionRuleFlags) []hookdeck.Rule {
	t.Helper()
	rules, err := buildConnectionRules(f)
	require.NoError(t, err)
	return rules
}

func TestResolveRuleTransformationAttachesExisting(t *testing.T) {
	for _, nameOrID := range []string{"trs_abc123", "orders-transform"} {
		t.Run(nameOrID, func(t *testing.T) {
			f := &connectionRuleFlags{RuleTransformName: nameOrID, RuleFilterBody: `{"type":"order"}`}
			rules := buildTransformRules(t, f)

			require.NoError(t, resolveRuleTransformation(context.Background(), newFakeTransformations(), f, rules))

			assert.Equal(t, hookdeck.Rule{"type": "transform", "transformation_id": "trs_abc123"}, rules[0],
				"an existing transformation is referenced by ID, never sent as a name")
			assert.Equal(t, "filter", rules[1]["type"], "other rules are untouched")
		})
	}
}

func TestResolveRuleTransformationUpdatesExistingWithCodeAndEnv(t *testing.T) {
	f := &connectionRuleFlags{
		RuleTransformName: "trs_abc123",
		RuleTransformCode: `addHandler("transform", (request) => request);`,
		RuleTransformEnv:  `{"KEY":"value"}`,
	}
	rules := buildTransformRules(t, f)

	require.NoError(t, resolveRuleTransformation(context.Background(), newFakeTransformations(), f, rules))

	assert.Equal(t, "trs_abc123", rules[0]["transformation_id"])
	update, ok := rules[0]["transformation"].(map[string]interface{})
	require.True(t, ok, "code and env are sent as an update to the existing transformation")
	assert.Equal(t, f.RuleTransformCode, update["code"])
	assert.Equal(t, map[string]interface{}{"KEY": "value"}, update["env"])
	assert.NotContains(t, update, "name", "the ID must not be sent as the transformation's new name")
}

func TestResolveRuleTransformationNotFound(t *testing.T) {
	t.Run("name without code is an error", func(t *testing.T) {
		// "orders" is a prefix of an existing name, so only an exact match may count
		f := &connectionRuleFlags{RuleTransformName: "orders"}
		err := resolveRuleTransformation(context.Background(), newFakeTransformations(), f, buildTransformRules(t, f))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "transformation 'orders' not found")
		assert.Contains(t, err.Error(), "--rule-transform-code")
	})

	t.Run("ID is an error, even with code", func(t *testing.T) {
		f := &connectionRuleFlags{RuleTransformName: "trs_missing", RuleTransformCode: "x"}
		err := resolveRuleTransformation(context.Background(), newFakeTransformations(), f, buildTransformRules(t, f))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "transformation 'trs_missing' not found")
	})

	t.Run("name with code creates a new transformation", func(t *testing.T) {
		f := &connectionRuleFlags{RuleTransformName: "new-transform", RuleTransformCode: "x"}
		rules := buildTransformRules(t, f)
		require.NoError(t, resolveRuleTransformation(context.Background(), newFakeTransformations(), f, rules))
		assert.Equal(t, map[string]interface{}{"name": "new-transform", "code": "x"}, rules[0]["transformation"])
		assert.NotContains(t, rules[0], "transformation_id")
	})
}

func TestResolveRuleTransformationRequiresName(t *testing.T) {
	f := &connectionRuleFlags{RuleTransformCode: "x"}
	err := resolveRuleTransformation(context.Background(), newFakeTransformations(), f, buildTransformRules(t, f))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--rule-transform-name is required")
}

func TestResolveRuleTransformationLeavesRulesJSONAlone(t *testing.T) {
	f := &connectionRuleFlags{
		Rules:             `[{"type":"transform","transformation":{"name":"trs_abc123"}}]`,
		RuleTransformName: "ignored",
	}
	rules := buildTransformRules(t, f)
	fake := newFakeTransformations()

	require.NoError(t, resolveRuleTransformation(context.Background(), fake, f, rules))
	assert.Equal(t, map[string]interface{}{"name": "trs_abc123"}, rules[0]["transformation"])
	assert.Zero(t, fake.calls, "--rules is sent as given, without lookups")
}

func TestResolveRuleTransformationReturnsLookupErrors(t *testing.T) {
	fake := newFakeTransformations()
	fake.listErr = errors.New("boom")
	f := &connectionRuleFlags{RuleTransformName: "orders-transform"}

	err := resolveRuleTransformation(context.Background(), fake, f, buildTransformRules(t, f))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}
