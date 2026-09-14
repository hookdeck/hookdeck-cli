package mcp

import (
	"fmt"
	"sort"
	"strings"
)

// Per-action argument matrices for the multi-action tools.
//
// The CLI keeps one flag set per subcommand, so `gateway request list` simply
// has no --delivery-group and `gateway request events` has no --source-id. The
// MCP layer flattens those subcommands into one tool with one flat schema, and
// that precision is lost: a filter meant for a sibling action is accepted,
// never forwarded, and the caller reads an unfiltered result as a filtered one.
// Every other filter narrows to zero rows on a bogus value, so nothing in the
// response reveals the drop.
//
// These maps re-impose the per-subcommand precision, naming the arguments each
// action actually forwards to the API. The refusal wording matches the metrics
// tool's, which has guarded the same hazard for a while.
var (
	// requestsActionArgs mirrors the flags of `hookdeck gateway request <sub>`.
	// delivery_group is on events only - the /requests collection has no such
	// query parameter, so on list the API answers with unfiltered rows.
	requestsActionArgs = map[string][]string{
		"list": {
			"id", "source_id", "status", "rejection_cause", "verified",
			"created_after", "created_before", "ingested_after", "ingested_before",
			"body", "headers", "parsed_query", "path",
			"order_by", "dir", "limit", "next", "prev",
		},
		"get":            {"id"},
		"raw_body":       {"id"},
		"events":         {"id", "delivery_group", "limit", "next", "prev"},
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
