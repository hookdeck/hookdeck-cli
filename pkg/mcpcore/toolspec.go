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

	// Tool puts this action on its own tool, named <resource>_<Tool>, instead of
	// the read or write tool its flags would otherwise imply.
	//
	// It exists for the ungated mutations. An action that changes state but is
	// deliberately not gated cannot sit on the read tool — that tool would lose
	// ReadOnlyHint and stop being blanket-allowable, which is the whole point of
	// the split — and it cannot sit on the write tool either, because that would
	// gate it. So it gets a third name with a third posture: available in both
	// modes, honestly annotated as changing state.
	//
	// connections pause/unpause and projects use are the cases. Leaving Tool
	// empty on a Mutates action silently puts it back on the read tool and
	// breaks that tool's annotation, so TestNoToolMixesReadAndWrite fails.
	Tool string
}

// Group names the tool this action belongs to: "read", "write", or whatever
// Tool says. It is the suffix in <prefix>_<resource>_<group>.
func (a Action) Group() string {
	if a.Tool != "" {
		return a.Tool
	}
	if a.Write {
		return GroupWrite
	}
	return GroupRead
}

// Changes reports whether the action alters state, whether or not it is gated.
func (a Action) Changes() bool { return a.Write || a.Mutates }

// Enabled reports whether the action is available in this mode.
func (a Action) Enabled(writeEnabled bool) bool { return writeEnabled || !a.Write }

// The two groups every resource has by default. A resource with no write
// actions still renders a _read tool, and a resource that is entirely write
// still renders only a _write tool: the suffix is a property of the actions,
// not of whether a counterpart happens to exist. That uniformity is what lets a
// grant be written as `*_read` on any client.
const (
	GroupRead  = "read"
	GroupWrite = "write"
)

// actionArgName is the argument every product tool dispatches on.
const actionArgName = "action"

// ActionSet is a tool's action list.
type ActionSet []Action

// Groups returns the distinct groups in the set, in the order they first
// appear. Declaration order is the advertised order, so a spec that lists
// list, get, pause, create renders read, pause, write — reads first, the
// gated tool last.
func (as ActionSet) Groups() []string {
	seen := map[string]bool{}
	out := make([]string, 0, 3)
	for _, a := range as {
		g := a.Group()
		if !seen[g] {
			seen[g] = true
			out = append(out, g)
		}
	}
	return out
}

// InGroup returns the actions belonging to one group.
func (as ActionSet) InGroup(group string) ActionSet {
	out := make(ActionSet, 0, len(as))
	for _, a := range as {
		if a.Group() == group {
			out = append(out, a)
		}
	}
	return out
}

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

	// Platform names this tool with the platform prefix (hookdeck_) rather than
	// the product one. Logging in, switching project and managing organizations
	// are Hookdeck operations, not Gateway or Outpost ones, so both servers
	// expose them under the same name — which is correct, and what a client
	// with both configured needs to see.
	Platform bool

	// Notes is hand-written guidance appended to the generated help topic:
	// worked examples, filter-syntax mappings, anything that cannot be derived
	// from the actions and props. Optional.
	Notes string
}

// ToolName is this resource's base tool name, before the group suffix.
func (spec ToolSpec) ToolName(srv *Server) string {
	if spec.Platform {
		return srv.platformToolName(spec.Resource)
	}
	return srv.ToolName(spec.Resource)
}

// GroupToolName is the full name of one group's tool.
func (spec ToolSpec) GroupToolName(srv *Server, group string) string {
	return spec.ToolName(srv) + "_" + group
}

// VisibleProps returns the properties a tool advertises in this mode.
//
// Define and Help must agree: a help topic listing `config` for a tool whose
// schema does not have it gives an agent two different answers depending on
// where it looks, and the help topic is the more persuasive of the two. Both
// call this rather than reading Props directly.
func (spec ToolSpec) VisibleProps(group string) map[string]Prop {
	// Which actions this group actually offers. A property scoped to actions
	// none of them include has nothing to act on here.
	inGroup := map[string]bool{}
	for _, a := range spec.Actions.InGroup(group) {
		inGroup[a.Name] = true
	}

	props := make(map[string]Prop, len(spec.Props))
	for name, prop := range spec.Props {
		if !prop.visibleIn(group) {
			continue
		}
		// Honour Actions as well as Only and Write. Without this a property
		// declared Actions: ["list"] was still advertised on the _write tool,
		// which has no list: gateway_destinations_write offered limit, next
		// and prev, and outpost_config_write offered a key described as "a
		// single configuration key to read (get)" on a tool with no get.
		// Dead parameters an agent can be led into passing.
		if len(prop.Actions) > 0 && !prop.usableBy(inGroup) {
			continue
		}
		props[name] = prop
	}
	return props
}

// visibleIn reports whether a property belongs on the tool for this group.
// usableBy reports whether any action this property is scoped to is offered by
// the group being built.
func (p Prop) usableBy(groupActions map[string]bool) bool {
	for _, a := range p.Actions {
		if groupActions[a] {
			return true
		}
	}
	return false
}

func (p Prop) visibleIn(group string) bool {
	if len(p.Only) > 0 {
		for _, g := range p.Only {
			if g == group {
				return true
			}
		}
		return false
	}
	if p.Write {
		return group == GroupWrite
	}
	return true
}

// appliesTo reports whether the property is meaningful for this action.
func (p Prop) appliesTo(action string) bool {
	if len(p.Actions) == 0 {
		return true
	}
	for _, a := range p.Actions {
		if a == action {
			return true
		}
	}
	return false
}

// actionsAccepting names the actions that do take a property, so a refusal can
// redirect rather than just say no.
func (spec ToolSpec) actionsAccepting(name string, group string) []string {
	prop, ok := spec.Props[name]
	if !ok {
		return nil
	}
	out := []string{}
	for _, a := range spec.Actions.InGroup(group) {
		if prop.appliesTo(a.Name) {
			out = append(out, a.Name)
		}
	}
	return out
}

// writeActionNames lists the gated actions, for the message that explains where
// a write-only property belongs.
func (spec ToolSpec) writeActionNames() []string {
	out := make([]string, 0, len(spec.Actions))
	for _, a := range spec.Actions {
		if a.Write {
			out = append(out, a.Name)
		}
	}
	sort.Strings(out)
	return out
}

// Help renders a tool's help topic from its definition, so help cannot drift
// from the schema the agent is actually given. available is the action set for
// the current mode.
func (spec ToolSpec) Help(srv *Server, group string, available ActionSet) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n%s\n\nActions:\n", spec.GroupToolName(srv, group), spec.Summary)

	width := 0
	for _, a := range available {
		if len(a.Name) > width {
			width = len(a.Name)
		}
	}
	for _, a := range available {
		fmt.Fprintf(&b, "  %-*s — %s\n", width, a.Name, a.Desc)
	}

	if group != GroupWrite && spec.Actions.HasWrite() {
		fmt.Fprintf(&b, "\nActions that create, change or delete live on %s, which this server registers only when started with --allow-write. See %s.\n",
			spec.GroupToolName(srv, GroupWrite), srv.HelpToolName())
	}

	visible := spec.VisibleProps(group)
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

// Define builds this resource's tools for the current write mode — one per
// action group, in declaration order.
//
// Before v3.0.0 this rendered a single tool whose action enum changed with the
// mode. That could not be permissioned: MCP clients grant per tool NAME, with
// no argument matching, so "let the agent read connections but ask before it
// changes one" was inexpressible — allowing gateway_connections allowed delete.
// Splitting by group makes the name the boundary, which is the only boundary
// the protocol offers.
//
// THE INVARIANT: four things here depend on the group and must agree — the
// action enum, the properties, the description, and the annotations. Nothing in
// the type system holds them together, and each has been wrong separately:
// descriptions named actions the enum had dropped, and schemas offered
// parameters belonging to actions that were not on offer. If you add another
// group-dependent field, add it to TestReadOnlyModeHidesWriteOnlyPropsAndProse
// in pkg/gateway/mcp/server_test.go, which is what catches them drifting apart.
//
// A tool is omitted entirely rather than registered to always fail: the write
// tool simply does not exist in read-only mode, and its absence from
// tools/list is the signal.
func (spec ToolSpec) Define(srv *Server) []ToolDef {
	var defs []ToolDef
	for _, group := range spec.Actions.Groups() {
		if def, ok := spec.defineGroup(srv, group); ok {
			defs = append(defs, def)
		}
	}
	return defs
}

// defineGroup renders one group's tool.
func (spec ToolSpec) defineGroup(srv *Server, group string) (ToolDef, bool) {
	actions := spec.Actions.InGroup(group)
	if len(actions) == 0 {
		return ToolDef{}, false
	}
	// The gated group exists only in write mode. Every other group is present
	// in both, byte for byte — that is what makes a grant on it durable.
	if group == GroupWrite && !srv.WriteEnabled() {
		return ToolDef{}, false
	}

	props := spec.VisibleProps(group)
	props[actionArgName] = Prop{
		Type: "string",
		Desc: "Action: " + actions.Summary(),
		Enum: actions.Names(),
	}

	name := spec.GroupToolName(srv, group)
	description := spec.Summary + " Actions: " + actions.Summary() + "."
	if group != GroupWrite && spec.Actions.HasWrite() {
		// Deliberately not conditioned on the mode. A read tool has to read
		// identically in both, or a permission granted against it no longer
		// describes the same thing after the server is restarted with
		// --allow-write — which is the property the whole split exists to
		// create. Pinned by TestReadToolsAreIdenticalInBothModes.
		//
		// The sentence is true either way: it says where those actions live and
		// what the server needs to register them, not whether they are
		// available right now.
		description += fmt.Sprintf(
			" Actions that create, change or delete live on %s, which this server registers only when started with --allow-write; see %s.",
			spec.GroupToolName(srv, GroupWrite), srv.HelpToolName(),
		)
	}

	// Both annotations are now honest without special-casing, because the enum
	// is homogeneous. HasChanging, not HasWrite: an ungated mutation still
	// changes state, so its tool must not claim to be a pure read — a client
	// that auto-approves ReadOnlyHint would otherwise halt production delivery
	// without asking anyone.
	destructive := actions.HasDestructive()
	return ToolDef{
		Tool: &mcpsdk.Tool{
			Name:        name,
			Description: description,
			InputSchema: Schema(props, append([]string{actionArgName}, spec.Required...)...),
			Annotations: &mcpsdk.ToolAnnotations{
				ReadOnlyHint:    !actions.HasChanging(),
				DestructiveHint: &destructive,
			},
		},
		Handler: rejectUnknownArgs(srv, spec, group, props, spec.Handler(srv)),
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
// what nothing can consume: a non-string inside a string array, an array or
// object where a single value belongs, and a value outside a declared enum.
//
// The enum check is the same failure as the rest. An enum in the schema is a
// claim about what the tool accepts, and nothing was checking it, so
// outpost_tenants_write minted a portal URL for theme "purple" while
// `hookdeck outpost tenant portal <t> --theme purple` — the same operation on
// the other surface — refused it. A declared enum is now enforced on both.
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
				continue
			case map[string]interface{}:
				problems = append(problems, fmt.Sprintf("%s takes a single value, not an object", key))
				continue
			}
			if problem := checkEnum(key, prop, value); problem != "" {
				problems = append(problems, problem)
			}
		}
	}

	sort.Strings(problems)
	return problems
}

// checkEnum reports a value outside the property's declared enum.
//
// "action" is left alone: Dispatch already rejects an unknown action, with the
// better message — it names the actions available in the current mode and can
// point at the sibling tool that carries the one the caller reached for.
func checkEnum(key string, prop Prop, value interface{}) string {
	if key == actionArgName || len(prop.Enum) == 0 {
		return ""
	}
	// Only a string can be compared against the enum; a number or boolean where
	// one belongs is the input helpers' business, not this check's.
	given, isString := value.(string)
	if !isString || given == "" {
		return ""
	}
	for _, allowed := range prop.Enum {
		if given == allowed {
			return ""
		}
	}
	return fmt.Sprintf("%s must be one of: %s", key, strings.Join(prop.Enum, ", "))
}

// actionScopeHint names the actions a write-only property belongs to.
func actionScopeHint(writeActions []string) string {
	if len(writeActions) == 0 {
		return "it belongs to this tool's write actions"
	}
	return "it belongs to " + strings.Join(writeActions, ", ")
}

func rejectUnknownArgs(srv *Server, spec ToolSpec, group string, visible map[string]Prop, next mcpsdk.ToolHandler) mcpsdk.ToolHandler {
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

		// Only defer when the caller is reaching for an action this tool does
		// not carry. On an action this tool does have, a hidden argument is
		// just as ignorable as an invented one, and has to be rejected.
		requestedAction, haveAction := spec.Actions.Find(in.String("action"))
		requestedHidden := haveAction && requestedAction.Group() != group

		// An action that belongs to a sibling tool must never run here. The
		// schema enum already omits it, but a client that does not validate
		// against the schema would otherwise have the read tool perform a
		// write once --allow-write was on — the split would be advisory.
		//
		// In read-only mode the write guard has the better message, so this
		// defers to it: "restart with --allow-write" is what the caller needs,
		// not the name of a tool that is not registered.
		if requestedHidden {
			sibling := spec.GroupToolName(srv, requestedAction.Group())
			if !(requestedAction.Write && !srv.WriteEnabled()) {
				return ErrorResult(fmt.Sprintf(
					"action %q is not available on this tool; it belongs to %s. "+
						"This tool offers: %s.",
					requestedAction.Name, sibling,
					strings.Join(spec.Actions.InGroup(group).Names(), ", "),
				)), nil
			}
		}

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
				// A property this tool has, on an action that does not read it.
				// The API ignores it, so the result comes back unfiltered while
				// reading as filtered — the failure this guard exists for.
				// Only for an action this tool actually carries. When the action
				// belongs to a sibling tool, the mode guard above has the
				// better message — "restart with --allow-write" beats telling
				// the caller an argument is on the wrong action of a tool that
				// was never going to run it.
				if haveAction && !requestedHidden && !prop.appliesTo(requestedAction.Name) {
					wrongAction = append(wrongAction, key)
					continue
				}
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
			writeActions := spec.writeActionNames()
			hint := actionScopeHint(writeActions)
			if accepted := spec.actionsAccepting(wrongAction[0], group); len(accepted) > 0 {
				hint = "it belongs to " + strings.Join(accepted, ", ")
			}
			return ErrorResult(fmt.Sprintf(
				"%s cannot be used with action %q — %s. "+
					"The action ignores it, so the result would have looked filtered without being filtered.",
				strings.Join(wrongAction, ", "), requestedAction.Name, hint,
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
