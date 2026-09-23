// Package toolprose checks the prose an MCP tool shows an agent against the
// schema and actions that tool actually has.
//
// It exists because the two disagree silently. Nothing fails to compile when a
// property's description offers a shape the validator refuses, or names an
// action that belongs to the sibling tool — and the description is the more
// persuasive of the two, so an agent follows it into an error the tool could
// have avoided telling it about.
//
// Both servers define their tools from the same mcpcore.ToolSpec, so both
// guards live here and both servers' tests call them. It is a non-test package
// for the same reason internal/speccheck is: it is shared by the tests of two
// packages that cannot import each other's test code.
package toolprose

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

// arrayWord matches a description offering array input.
var arrayWord = regexp.MustCompile(`(?i)\barrays?\b`)

// ArrayPromises reports properties whose description offers an array while the
// schema declares a single value.
//
// mcpcore.checkArgumentTypes refuses an array for a scalar property, so such a
// description documents a call that cannot succeed. Thirteen Outpost properties
// said "Accepts an array of strings or a comma-separated string" over
// "type": "string", and the array half was answered with
// "<name> takes a single value, not an array".
func ArrayPromises(props map[string]mcpcore.Prop) []string {
	var problems []string
	for name, prop := range props {
		if prop.Type == "array" || prop.JSONValue {
			continue
		}
		if arrayWord.MatchString(prop.Desc) {
			problems = append(problems, fmt.Sprintf(
				"%s is declared %q but its description offers an array: %q",
				name, prop.Type, prop.Desc))
		}
	}
	sort.Strings(problems)
	return problems
}

// UnknownActionMentions reports action names that text names but the tool does
// not offer.
//
// vocabulary is the action names the whole resource has, across every group;
// offered is the subset this tool carries. Restricting the search to the
// resource's own vocabulary is what keeps it quiet: "create" in a tenants
// description is prose, because tenants has no create action, while "create" in
// a destinations description is a promise about an action that may live on the
// other tool.
//
// Only the forms this codebase uses to name an action are searched, because a
// bare word match would flag every "Max results (list)" style phrase that is
// already correct and every ordinary use of "get", "set" or "run".
func UnknownActionMentions(text string, offered, vocabulary []string) []string {
	known := map[string]bool{}
	for _, a := range offered {
		known[a] = true
	}
	vocab := map[string]bool{}
	for _, a := range vocabulary {
		vocab[a] = true
	}

	seen := map[string]bool{}
	var unknown []string
	for _, mention := range actionMentions(text, vocab) {
		if known[mention] || seen[mention] {
			continue
		}
		seen[mention] = true
		unknown = append(unknown, mention)
	}
	sort.Strings(unknown)
	return unknown
}

var (
	// parenthesised captures "(list)", "(create/upsert/update)", "(get, cancel)",
	// "(required for create)" — the trailing action scope this codebase writes.
	parenthesised = regexp.MustCompile(`\(([^()]*)\)`)

	// prefixed captures "Required for get/update/delete", "optional on run",
	// "On list, filters by…", "Filters on list".
	prefixed = regexp.MustCompile(`(?i)\b(?:required|optional|available|accepted|supported|filters?|used)?\s*\b(?:for|on)\s+([a-z_]+(?:\s*(?:/|,\s*|\s+and\s+|\s+or\s+)[a-z_]+)*)`)

	// named captures "retry action only".
	named = regexp.MustCompile(`(?i)\b([a-z_]+)\s+action\b`)

	// separators split an action list into its names.
	separators = regexp.MustCompile(`\s*(?:/|,|;|\s+and\s+|\s+or\s+)\s*`)

	// scopePrefix strips the lead-in from a parenthesised scope so
	// "(required for create)" yields "create".
	scopePrefix = regexp.MustCompile(`(?i)^(?:required|optional|available|accepted|supported|only)?\s*(?:for|on)?\s+`)
)

// actionMentions extracts the action names a description refers to.
func actionMentions(text string, vocab map[string]bool) []string {
	var out []string

	collect := func(list string) {
		for _, token := range separators.Split(list, -1) {
			token = strings.TrimSpace(scopePrefix.ReplaceAllString(strings.TrimSpace(token), ""))
			token = strings.Trim(token, ".;:")
			if vocab[token] {
				out = append(out, token)
			}
		}
	}

	for _, m := range parenthesised.FindAllStringSubmatch(text, -1) {
		collect(m[1])
	}
	for _, m := range prefixed.FindAllStringSubmatch(text, -1) {
		collect(m[1])
	}
	for _, m := range named.FindAllStringSubmatch(text, -1) {
		collect(m[1])
	}
	return out
}
