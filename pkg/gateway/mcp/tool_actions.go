package mcp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// Per-action argument matrices for the multi-action tools.
//
// The CLI keeps one flag set per subcommand, so `gateway request list` simply
// has no --delivery-group. The MCP layer flattens those subcommands into one
// tool with one flat schema, and that precision is lost: a filter meant for a
// sibling action is accepted, never forwarded, and the caller reads an
// unfiltered result as a filtered one. Every other filter narrows to zero rows
// on a bogus value, so nothing in the response reveals the drop.
//
// These maps re-impose the per-subcommand precision, naming the arguments each
// action actually forwards to the API. Membership is decided by what the route
// declares in https://api.hookdeck.com/2026-09-01/openapi, not by what the
// handler happens to send today: an argument the route honours belongs here and
// must be forwarded, and only one it does not is refused. The refusal wording
// matches the metrics tool's, which has guarded the same hazard for a while.
var (
	// requestsActionArgs mirrors the flags of `hookdeck gateway request <sub>`.
	//
	// GET /requests/{id}/events declares the same filter set as GET /events, so
	// events carries nearly everything list does plus the delivery filters.
	// delivery_group stays events-only: the /requests collection has no such
	// query parameter, so on list the API answers with unfiltered rows.
	// rejection_cause, verified and ingested_* are the reverse - they describe
	// the edge decision, which the events sub-resource knows nothing about.
	requestsActionArgs = map[string][]string{
		"list": {
			"id", "source_id", "status", "rejection_cause", "verified",
			"created_after", "created_before", "ingested_after", "ingested_before",
			"body", "headers", "parsed_query", "path",
			"order_by", "dir", "limit", "next", "prev",
		},
		"get":      {"id"},
		"raw_body": {"id"},
		"events": {
			"id", "connection_id", "source_id", "destination_id", "delivery_group",
			"status", "attempts", "issue_id", "error_code", "response_status", "cli_id",
			"created_after", "created_before", "successful_after", "successful_before",
			"last_attempt_after", "last_attempt_before",
			"body", "headers", "parsed_query", "path",
			"order_by", "dir", "limit", "next", "prev",
		},
		"ignored_events": {"id", "limit", "next", "prev"},
	}

	// eventsActionArgs mirrors the flags of `hookdeck gateway event <sub>`.
	// get and raw_body address one event by id, so every list filter passed
	// alongside them was dropped in silence.
	eventsActionArgs = map[string][]string{
		"list": {
			"id", "connection_id", "source_id", "destination_id", "delivery_group",
			"status", "attempts", "issue_id", "error_code", "response_status", "cli_id",
			"created_after", "created_before", "successful_after", "successful_before",
			"last_attempt_after", "last_attempt_before",
			"body", "headers", "parsed_query", "path",
			"order_by", "dir", "limit", "next", "prev",
		},
		"get":      {"id"},
		"raw_body": {"id"},
	}
)

// argIsSet reports whether the caller actually supplied a value. The handlers
// forward string and numeric arguments only when non-zero, so a key present
// with an empty value is not a dropped filter and must not be refused.
func argIsSet(v interface{}) bool {
	switch val := v.(type) {
	case nil:
		return false
	case string:
		return val != ""
	case float64:
		return val != 0
	case []interface{}:
		return len(val) > 0
	case map[string]interface{}:
		return len(val) > 0
	default:
		return true
	}
}

// rejectArgsUnsupportedByAction reports the first argument the tool declares but
// this action does not honour.
//
// Only arguments the tool's own schema advertises are policed: an unrecognised
// key is not a filter the caller expected to take effect, and MCP clients add
// their own. declared is the tool's schema property map.
func rejectArgsUnsupportedByAction(in input, tool, action string, byAction map[string][]string, declared map[string]prop) error {
	supported, ok := byAction[action]
	if !ok {
		// An unknown action is the handler's own error to report.
		return nil
	}
	allowed := make(map[string]bool, len(supported))
	for _, name := range supported {
		allowed[name] = true
	}

	names := make([]string, 0, len(in))
	for name := range in {
		names = append(names, name)
	}
	// Deterministic so the same call always names the same argument first.
	sort.Strings(names)

	for _, name := range names {
		if name == "action" || allowed[name] || !argIsSet(in[name]) {
			continue
		}
		if _, isDeclared := declared[name]; !isDeclared {
			continue
		}
		return fmt.Errorf("%s is not supported by the %s action of %s; the API would ignore it and return unfiltered results%s",
			name, action, tool, otherActionsFor(name, byAction, action))
	}
	return nil
}

// otherActionsFor names the actions that do honour the argument, so the caller
// can move the call rather than guess.
func otherActionsFor(name string, byAction map[string][]string, except string) string {
	var actions []string
	for action, supported := range byAction {
		if action == except {
			continue
		}
		for _, s := range supported {
			if s == name {
				actions = append(actions, action)
				break
			}
		}
	}
	if len(actions) == 0 {
		return ""
	}
	sort.Strings(actions)
	return ". It applies to: " + strings.Join(actions, ", ")
}

// requestsStatusVocabulary is the status enum of the collection each
// hookdeck_requests action queries.
//
// The tool has one flat `status` property and the two actions mean different
// things by it, so the value has to be checked against the action's own
// vocabulary. The API does reject an out-of-enum status - "SUCCESSFUL" on list
// comes back as 422 "status must be one of [accepted, rejected]" - but that
// message names only the route it was sent to, so a caller who reached for the
// wrong action is told the value is wrong rather than that the sibling action
// takes it. Checking here also lets either case through, which the API does
// not: it 422s "successful" against the upper-case enum.
var requestsStatusVocabulary = map[string][]string{
	"list":   hookdeck.RequestLogStatusValueList,
	"events": hookdeck.EventStatusValueList,
}

// canonicalRequestsStatus returns the status to send for this action, in the
// API's own spelling, or an error naming the vocabulary the action does take.
func canonicalRequestsStatus(action, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	vocabulary, ok := requestsStatusVocabulary[action]
	if !ok {
		return value, nil
	}
	if canonical, ok := hookdeck.CanonicalStatusValue(vocabulary, value); ok {
		return canonical, nil
	}
	return "", fmt.Errorf("status %q is not supported by the %s action of hookdeck_requests; it filters by %s%s",
		value, action, hookdeck.ValueList(vocabulary), otherStatusVocabularyFor(value, action))
}

// otherStatusVocabularyFor points at the action the value does belong to, so a
// caller who reached for the wrong one is told where it works.
func otherStatusVocabularyFor(value, except string) string {
	for _, action := range []string{"list", "events"} {
		if action == except {
			continue
		}
		if _, ok := hookdeck.CanonicalStatusValue(requestsStatusVocabulary[action], value); ok {
			return fmt.Sprintf(". It belongs to the %s action, which filters by %s",
				action, hookdeck.ValueList(requestsStatusVocabulary[action]))
		}
	}
	return ""
}
