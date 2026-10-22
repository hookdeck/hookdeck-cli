package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// transformationLookup is the subset of the API client used to resolve
// --rule-transform-name.
type transformationLookup interface {
	GetTransformation(ctx context.Context, id string) (*hookdeck.Transformation, error)
	ListTransformations(ctx context.Context, params map[string]string) (*hookdeck.TransformationListResponse, error)
}

// findTransformationID returns the ID of the transformation whose ID or exact
// name is nameOrID, or "" when there is none.
func findTransformationID(ctx context.Context, client transformationLookup, nameOrID string) (string, error) {
	if strings.HasPrefix(nameOrID, "trs_") {
		trn, err := client.GetTransformation(ctx, nameOrID)
		if err == nil {
			return trn.ID, nil
		}
		if !hookdeck.IsNotFoundError(err) {
			return "", fmt.Errorf("failed to look up transformation '%s': %w", nameOrID, err)
		}
	}

	result, err := client.ListTransformations(ctx, map[string]string{"name": nameOrID})
	if err != nil {
		return "", fmt.Errorf("failed to look up transformation '%s': %w", nameOrID, err)
	}
	for _, trn := range result.Models {
		if trn.Name == nameOrID {
			return trn.ID, nil
		}
	}
	return "", nil
}

// resolveRuleTransformation makes --rule-transform-name accept a transformation
// name or ID, as its help text says.
//
// The API matches a transform rule's transformation by name and creates one when
// the name is new, even without code. Sent as a name, an ID or a mistyped name
// therefore created an empty transformation that failed every event. So the value
// is resolved first:
//   - found: the rule references it by transformation_id; --rule-transform-code and
//     --rule-transform-env, if given, update that transformation
//   - not found, with --rule-transform-code: a new transformation is created with
//     that name, as before (not allowed for an ID)
//   - not found, without --rule-transform-code: an error
//
// Rules from --rules or --rules-file are sent as given.
func resolveRuleTransformation(ctx context.Context, client transformationLookup, f *connectionRuleFlags, rules []hookdeck.Rule) error {
	if f.Rules != "" || f.RulesFile != "" {
		return nil
	}
	if f.RuleTransformName == "" {
		if f.RuleTransformCode != "" || f.RuleTransformEnv != "" {
			return fmt.Errorf("--rule-transform-name is required when using transform rule flags")
		}
		return nil
	}

	idx := -1
	for i, r := range rules {
		if r["type"] == "transform" {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil
	}

	nameOrID := f.RuleTransformName
	id, err := findTransformationID(ctx, client, nameOrID)
	if err != nil {
		return err
	}

	if id == "" {
		if strings.HasPrefix(nameOrID, "trs_") {
			return fmt.Errorf("transformation '%s' not found. Check the ID with 'hookdeck gateway transformation list'", nameOrID)
		}
		if f.RuleTransformCode == "" {
			return fmt.Errorf("transformation '%s' not found. Check the name with 'hookdeck gateway transformation list', or pass --rule-transform-code to create a new transformation with this name", nameOrID)
		}
		// New transformation: keep name and code so the API creates it
		return nil
	}

	rule := hookdeck.Rule{"type": "transform", "transformation_id": id}
	if existing, ok := rules[idx]["transformation"].(map[string]interface{}); ok {
		update := make(map[string]interface{})
		for _, key := range []string{"code", "env"} {
			if v, ok := existing[key]; ok {
				update[key] = v
			}
		}
		if len(update) > 0 {
			rule["transformation"] = update
		}
	}
	rules[idx] = rule
	return nil
}
