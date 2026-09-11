package mcpcore

import (
	"context"
	"fmt"
	"sort"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Action is one action a tool supports.
//
// Write marks an action that a read-only server must not offer. That covers
// anything that changes data, and also the reads that hand back a credential:
// a tenant token and a portal URL are both reusable access to a tenant's data,
// so treating them as reads would let a read-only session mint them at will.
//
// Destructive drives the client-facing DestructiveHint annotation.
//
// Mutates says the action changes something, which is a separate question from
// whether it is gated. Almost always Write implies Mutates and there is no need
// to set it. It exists for the actions deliberately left available in read-only
// mode despite changing state — pausing a connection is the natural end of an
// investigation, so it is not gated, but a tool offering it is not a pure read
// and must not claim ReadOnlyHint. Keeping the two flags apart lets the gating
// decision and the annotation disagree on purpose rather than by accident.
//
// If Mutates looks redundant: collapsing it into Write is exactly the change
// that breaks "a tool offering pause is not annotated read-only" and
// TestWriteGuard_PauseIsNotGated, both in pkg/gateway/mcp/write_mode_test.go.
type Action struct {
	Name        string
	Desc        string
	Write       bool
	Destructive bool
	Mutates     bool
}

// Changes reports whether the action alters state, whether or not it is gated.
func (a Action) Changes() bool { return a.Write || a.Mutates }

// Enabled reports whether the action is available in this mode.
func (a Action) Enabled(writeEnabled bool) bool { return writeEnabled || !a.Write }

// ActionSet is a tool's action list.
type ActionSet []Action

// Available returns the actions offered in this mode.
func (as ActionSet) Available(writeEnabled bool) ActionSet {
	out := make(ActionSet, 0, len(as))
	for _, a := range as {
		if a.Enabled(writeEnabled) {
			out = append(out, a)
		}
	}
	return out
}

// Names returns the action names, for the schema enum.
func (as ActionSet) Names() []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = a.Name
	}
	return out
}

// Summary renders "list — …, get — …" for a tool description.
func (as ActionSet) Summary() string {
	parts := make([]string, 0, len(as))
	for _, a := range as {
		if a.Desc == "" {
			parts = append(parts, a.Name)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", a.Name, a.Desc))
	}
	return strings.Join(parts, ", ")
}

// Find returns the named action.
func (as ActionSet) Find(name string) (Action, bool) {
	for _, a := range as {
		if a.Name == name {
			return a, true
		}
	}
	return Action{}, false
}

// HasWrite reports whether any action in the set is a write.
func (as ActionSet) HasWrite() bool {
	for _, a := range as {
		if a.Write {
			return true
		}
	}
	return false
}

// HasChanging reports whether any action in the set alters state, including the
// ones left available in read-only mode. This drives ReadOnlyHint, which is a
// claim about what the tool does rather than about what this mode gates.
func (as ActionSet) HasChanging() bool {
	for _, a := range as {
		if a.Changes() {
			return true
		}
	}
	return false
}

// HasDestructive reports whether any action in the set is destructive.
func (as ActionSet) HasDestructive() bool {
	for _, a := range as {
		if a.Destructive {
			return true
		}
	}
	return false
}

// ToolSpec describes one product tool before write mode is applied.
type ToolSpec struct {
	Resource string    // e.g. "tenants" — the tool is named "<prefix>_<resource>"
	Summary  string    // what the tool is for, without listing actions
	Actions  ActionSet // every action, including the write-only ones
	Props    map[string]Prop
	Required []string
	Handler  func(*Server) mcpsdk.ToolHandler

	// Notes is hand-written guidance appended to the generated help topic:
	// worked examples, filter-syntax mappings, anything that cannot be derived
	// from the actions and props. Optional.
	Notes string
}

// VisibleProps returns the properties a tool advertises in this mode.
//
// Define and Help must agree: a help topic listing `config` for a tool whose
// schema does not have it gives an agent two different answers depending on
// where it looks, and the help topic is the more persuasive of the two. Both
// call this rather than reading Props directly.
func (spec ToolSpec) VisibleProps(writeEnabled bool) map[string]Prop {
	props := make(map[string]Prop, len(spec.Props))
	for name, prop := range spec.Props {
		if prop.Write && !writeEnabled {
			continue
		}
		props[name] = prop
	}
	return props
}

// Help renders a tool's help topic from its definition, so help cannot drift
// from the schema the agent is actually given. available is the action set for
// the current mode.
func (spec ToolSpec) Help(srv *Server, available ActionSet) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n%s\n\nActions:\n", srv.ToolName(spec.Resource), spec.Summary)

	width := 0
	for _, a := range available {
		if len(a.Name) > width {
			width = len(a.Name)
		}
	}
	for _, a := range available {
		fmt.Fprintf(&b, "  %-*s — %s\n", width, a.Name, a.Desc)
	}

	if hidden := spec.Actions.HasWrite() && !srv.WriteEnabled(); hidden {
		fmt.Fprintf(&b, "\nFurther actions exist but are unavailable in read-only mode. See %s for how to enable them.\n", srv.HelpToolName())
	}

	visible := spec.VisibleProps(srv.WriteEnabled())
	if len(visible) > 0 {
		b.WriteString("\nParameters:\n")
		names := make([]string, 0, len(visible))
		for name := range visible {
			names = append(names, name)
		}
		sort.Strings(names)

		width = 0
		for _, name := range names {
			if len(name) > width {
				width = len(name)
			}
		}
		for _, name := range names {
			prop := visible[name]
			required := ""
			for _, r := range spec.Required {
				if r == name {
					required = ", required"
					break
				}
			}
			fmt.Fprintf(&b, "  %-*s (%s%s) — %s\n", width, name, prop.Type, required, prop.Desc)
		}
	}

	if spec.Notes != "" {
		b.WriteString("\n")
		b.WriteString(spec.Notes)
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

// Define builds the tool definition for the current write mode. The bool is
// false when the tool should not be registered at all.
//
// The schema is the primary gate: in read-only mode the write actions are
// absent from the enum and from the description, so an agent is never told
// about an action it cannot use. Tools whose every action is a write are not
// registered at all rather than registered to always fail.
//
// THE INVARIANT: four things here depend on the mode and must agree — the
// action enum, the properties, the description, and the annotations. Nothing in
// the type system holds them together, and each has been wrong separately:
// descriptions named actions the enum had dropped, and schemas offered
// parameters belonging to actions that were not on offer. If you add another
// mode-dependent field, add it to TestReadOnlyModeHidesWriteOnlyPropsAndProse
// in pkg/gateway/mcp/server_test.go, which is what catches them drifting apart.
func (spec ToolSpec) Define(srv *Server) (ToolDef, bool) {
	available := spec.Actions.Available(srv.WriteEnabled())
	if len(available) == 0 {
		return ToolDef{}, false
	}

	// Properties are filtered by mode for the same reason actions are: a
	// read-only session offered `config` or `rules` has been shown an
	// affordance it cannot use, and nothing in the schema says which action
	// they belong to.
	props := spec.VisibleProps(srv.WriteEnabled())
	props["action"] = Prop{
		Type: "string",
		Desc: "Action: " + available.Summary(),
		Enum: available.Names(),
	}

	description := spec.Summary + " Actions: " + available.Summary() + "."
	if spec.Actions.HasWrite() && !srv.WriteEnabled() {
		// The help tool name is sourced from the server rather than hardcoded,
		// so each product points at its own help tool.
		description += fmt.Sprintf(
			" This server is running in read-only mode, so only the actions listed above are available; see %s for how to enable the rest.",
			srv.HelpToolName(),
		)
	}

	destructive := available.HasDestructive()
	return ToolDef{
		Tool: &mcpsdk.Tool{
			Name:        srv.ToolName(spec.Resource),
			Description: description,
			InputSchema: Schema(props, append([]string{"action"}, spec.Required...)...),
			Annotations: &mcpsdk.ToolAnnotations{
				// HasChanging, not HasWrite: an action can change state and
				// still be offered in read-only mode (connections pause). Using
				// HasWrite here would tell a client this tool is a pure read
				// while it can halt delivery. Pinned by "a tool offering pause
				// is not annotated read-only" in pkg/gateway/mcp/write_mode_test.go.
				ReadOnlyHint:    !available.HasChanging(),
				DestructiveHint: &destructive,
			},
		},
		Handler: rejectUnknownArgs(srv, spec, props, spec.Handler(srv)),
	}, true
}

// rejectUnknownArgs fails a call that passes an argument the tool does not have.
//
// additionalProperties: false says this in the schema, but nothing enforces it
// at this layer: an unknown key was accepted, ignored, and the call went out
// unfiltered. gateway_events with request_id set returned every event in the
// project — there is no such filter there — and the caller had no way to tell
// that from a genuine result.
//
// Two things deliberately take precedence over this message:
//
//   - An unauthenticated call is handed to the handler, so the caller is told to
//     sign in rather than being corrected on an argument they cannot use yet.
//
//   - An argument that exists on the tool but is hidden by read-only mode is let
//     through ONLY when the action being requested is itself hidden, so the
//     write guard answers it. "restart with --allow-write" is the useful reply
//     for {"action":"create","type":"HTTP"}; "unknown argument" would send the
//     caller looking for a typo that is not there.
//
//     The action test matters. Without it, a hidden argument on a VISIBLE action
//     — {"action":"list","type":"HTTP"} on gateway_sources — was exempted, then
//     ignored by the handler, and the caller got an unfiltered list that read as
//     a filtered one. That is the failure this guard exists to prevent, so the
//     exemption cannot be allowed to reintroduce it.
//
// What is left is a genuine mistake: a name this tool has never had.
//
// Both exemptions look like holes and are not. TestUnknownArgumentsAreRejected
// and TestHiddenWriteArgumentsGetTheWriteModeMessage, in
// pkg/gateway/mcp/write_actions_test.go, fail if either is removed.
// checkArgumentTypes reports arguments whose value this tool cannot use.
//
// The schema declares a type for every property and, as with
// additionalProperties, nothing enforces it at this layer. The input helpers
// discard what they cannot convert: a non-string element is dropped from an
// array, and a non-string scalar reads as absent. So {"topics":["orders",5]}
// subscribed a destination to one topic and reported success, and a name given
// as a number left the name unchanged with nothing said. Both are the failure
// this package keeps finding — a wrong answer that reads like a right one.
//
// It is deliberately narrower than full schema validation, because two
// conversions here are intentional and in use: a comma-separated string is
// accepted where an array is declared, and numbers and booleans are accepted as
// strings (NumberOrString, BoolOrString). Enforcing the declared type strictly
// would reject callers those helpers exist to support. What is rejected is only
// what nothing can consume: a non-string inside a string array, and an array or
// object where a single value belongs.
func checkArgumentTypes(visible map[string]Prop, in Input) []string {
	var problems []string

	for key, value := range in {
		prop, known := visible[key]
		if !known || value == nil {
			continue
		}

		// A JSON-filter property is satisfied by an object or by a string
		// holding JSON; anything else is dropped by JSONFilterParam.
		if prop.JSONValue {
			switch value.(type) {
			case map[string]interface{}, string:
			default:
				// Same wording as JSONFilterParam, which reports this when the
				// value reaches it by another route. One condition, one message.
				problems = append(problems, fmt.Sprintf("%s must be a JSON string or object", key))
			}
			continue
		}

		switch prop.Type {
		case "array":
			arr, isArray := value.([]interface{})
			if !isArray {
				// A bare string is the comma-separated form StringList accepts.
				if _, isString := value.(string); !isString {
					problems = append(problems, fmt.Sprintf("%s must be an array", key))
				}
				continue
			}
			if prop.Items == nil || prop.Items.Type != "string" {
				continue
			}
			for i, item := range arr {
				if _, isString := item.(string); !isString {
					problems = append(problems, fmt.Sprintf("%s[%d] must be a string", key, i))
				}
			}
		case "string", "integer", "number", "boolean":
			switch value.(type) {
			case []interface{}:
				problems = append(problems, fmt.Sprintf("%s takes a single value, not an array", key))
			case map[string]interface{}:
				problems = append(problems, fmt.Sprintf("%s takes a single value, not an object", key))
			}
		}
	}

	sort.Strings(problems)
	return problems
}

// actionScopeHint names the actions a write-only property belongs to.
func actionScopeHint(writeActions []string) string {
	if len(writeActions) == 0 {
		return "it belongs to this tool's write actions"
	}
	return "it belongs to " + strings.Join(writeActions, ", ")
}

func rejectUnknownArgs(srv *Server, spec ToolSpec, visible map[string]Prop, next mcpsdk.ToolHandler) mcpsdk.ToolHandler {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := srv.RequireAuth(); r != nil {
			return next(ctx, req)
		}

		in, err := ParseInput(req.Params.Arguments)
		if err != nil {
			// Malformed arguments are the handler's to report, with the context
			// of the action being attempted.
			return next(ctx, req)
		}

		// Only defer when the caller is reaching for an action this mode hides.
		// On a visible action a hidden argument is just as ignorable as an
		// invented one, and has to be rejected.
		requestedAction, haveAction := spec.Actions.Find(in.String("action"))
		requestedHidden := haveAction && !requestedAction.Enabled(srv.WriteEnabled())

		var unknown, wrongAction []string
		for key := range in {
			if prop, visibleNow := visible[key]; visibleNow {
				// A write-only property belongs to the write actions, and the
				// read actions do not read it. In read-only mode it is hidden
				// and so rejected, but once write mode made it visible it was
				// accepted on a read action and then ignored — gateway_sources
				// with {"action":"list","type":"STRIPE"} returned every source
				// as though it were filtered. That is the exact failure this
				// guard exists to prevent, so enabling writes must not
				// reintroduce it on the actions that never took the argument.
				if prop.Write && haveAction && !requestedAction.Write {
					// Reported separately: the tool does have this argument, so
					// calling it unknown while listing it among the accepted
					// ones contradicts itself and sends the caller hunting for
					// a typo that is not there.
					wrongAction = append(wrongAction, key)
				}
				continue
			}
			if _, existsAtAll := spec.Props[key]; existsAtAll && requestedHidden {
				continue // the write guard has the better message for this call
			}
			unknown = append(unknown, key)
		}
		if len(wrongAction) > 0 {
			sort.Strings(wrongAction)
			writeActions := make([]string, 0, len(spec.Actions))
			for _, a := range spec.Actions {
				if a.Write {
					writeActions = append(writeActions, a.Name)
				}
			}
			sort.Strings(writeActions)
			return ErrorResult(fmt.Sprintf(
				"%s cannot be used with action %q — %s. "+
					"The action ignores it, so the result would have looked filtered without being filtered.",
				strings.Join(wrongAction, ", "), requestedAction.Name,
				actionScopeHint(writeActions),
			)), nil
		}

		if len(unknown) == 0 {
			// Only once the names are known to be real is it worth talking about
			// their values; an unknown name has a better message of its own.
			if problems := checkArgumentTypes(visible, in); len(problems) > 0 {
				return ErrorResult(strings.Join(problems, "; ")), nil
			}
			return next(ctx, req)
		}

		sort.Strings(unknown)
		known := make([]string, 0, len(visible))
		for key := range visible {
			known = append(known, key)
		}
		sort.Strings(known)

		return ErrorResult(fmt.Sprintf(
			"unknown argument(s): %s. This tool accepts: %s. "+
				"An argument this tool does not have is ignored by the API, so the result would have looked "+
				"filtered without being filtered.",
			strings.Join(unknown, ", "), strings.Join(known, ", "),
		)), nil
	}
}

// Dispatch validates and gates an action before a handler runs it.
//
// The schema already hides write actions in read-only mode; this is the second
// line of defence, for a client that calls one anyway.
func Dispatch(srv *Server, actions ActionSet, name string, hint ...string) (string, *mcpsdk.CallToolResult) {
	a, ok := actions.Find(name)
	if !ok {
		available := actions.Available(srv.WriteEnabled())
		message := fmt.Sprintf(
			"unknown action %q; expected one of: %s",
			name, strings.Join(available.Names(), ", "),
		)
		// Where a tool has a sibling, the action a caller reached for is often
		// the sibling's. Naming it turns a dead end into a redirect: without
		// this an agent has to already know the other tool exists.
		if len(hint) > 0 && hint[0] != "" {
			message += ". " + hint[0]
		}
		return "", ErrorResult(message)
	}
	if a.Write {
		if r := RequireWrite(srv.WriteEnabled(), name); r != nil {
			return "", r
		}
	}
	return a.Name, nil
}

// DispatchWithDefault is Dispatch, with an empty action name resolving to
// fallback instead of erroring.
//
// Defaulting lives here, in the handler, rather than on ToolSpec. Define always
// puts "action" in the schema's required list, so a schema-validating client
// never omits it and a spec-level default would only ever apply to callers that
// ignore the schema. Keeping it at the call site makes the fallback visible next
// to the switch it feeds.
func DispatchWithDefault(srv *Server, actions ActionSet, name, fallback string, hint ...string) (string, *mcpsdk.CallToolResult) {
	if name == "" {
		name = fallback
	}
	return Dispatch(srv, actions, name, hint...)
}
