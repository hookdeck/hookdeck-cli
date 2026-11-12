package cmd

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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

// pagedTransformations serves ListTransformations one page at a time, the way
// the API pages a name filter that matches more than one page of near matches.
type pagedTransformations struct {
	pages  [][]hookdeck.Transformation
	params []map[string]string
}

func (p *pagedTransformations) GetTransformation(_ context.Context, id string) (*hookdeck.Transformation, error) {
	return nil, &hookdeck.APIError{StatusCode: 404, Message: "not found"}
}

func (p *pagedTransformations) ListTransformations(_ context.Context, params map[string]string) (*hookdeck.TransformationListResponse, error) {
	copied := map[string]string{}
	for k, v := range params {
		copied[k] = v
	}
	p.params = append(p.params, copied)

	page := 0
	if next := params["next"]; next != "" {
		fmt.Sscanf(next, "page-%d", &page)
	}
	resp := &hookdeck.TransformationListResponse{}
	if page < len(p.pages) {
		resp.Models = p.pages[page]
	}
	if page+1 < len(p.pages) {
		next := fmt.Sprintf("page-%d", page+1)
		resp.Pagination.Next = &next
	}
	return resp, nil
}

func TestFindTransformationIDFollowsPagination(t *testing.T) {
	p := &pagedTransformations{pages: [][]hookdeck.Transformation{
		{{ID: "trs_near1", Name: "orders-transform-v1"}, {ID: "trs_near2", Name: "orders-transform-v2"}},
		{{ID: "trs_near3", Name: "orders-transform-v3"}},
		{{ID: "trs_exact", Name: "orders-transform"}},
	}}

	id, err := findTransformationID(context.Background(), p, "orders-transform")
	require.NoError(t, err)
	assert.Equal(t, "trs_exact", id, "the exact match on the third page should be found")

	require.Len(t, p.params, 3, "one request per page")
	for _, params := range p.params {
		assert.Equal(t, "orders-transform", params["name"])
		assert.Equal(t, strconv.Itoa(transformationLookupPageLimit), params["limit"], "each page asks for the largest page size")
	}
	assert.Equal(t, "", p.params[0]["next"])
	assert.Equal(t, "page-1", p.params[1]["next"])
	assert.Equal(t, "page-2", p.params[2]["next"])
}

func TestFindTransformationIDStopsWhenNoNextPage(t *testing.T) {
	p := &pagedTransformations{pages: [][]hookdeck.Transformation{
		{{ID: "trs_near1", Name: "orders-transform-v1"}},
	}}

	id, err := findTransformationID(context.Background(), p, "orders-transform")
	require.NoError(t, err)
	assert.Equal(t, "", id, "no exact match means not found")
	assert.Len(t, p.params, 1, "no further request without pagination.next")
}

func TestFindTransformationIDBoundsPagination(t *testing.T) {
	pages := make([][]hookdeck.Transformation, transformationLookupMaxPages+1)
	for i := range pages {
		pages[i] = []hookdeck.Transformation{{ID: fmt.Sprintf("trs_near%d", i), Name: fmt.Sprintf("orders-transform-%d", i)}}
	}
	p := &pagedTransformations{pages: pages}

	_, err := findTransformationID(context.Background(), p, "orders-transform")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pass its ID instead")
	assert.Len(t, p.params, transformationLookupMaxPages, "stops at the page bound")
}
