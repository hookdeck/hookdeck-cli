package mcpcore

import (
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
type Action struct {
	Name        string
	Desc        string
	Write       bool
	Destructive bool
}

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

	// DefaultAction, when set, is the action used if the caller omits one. It
	// exists for tools that shipped before "action" was mandatory in practice
	// and whose callers still send bare list requests.
	DefaultAction string

	// Notes is hand-written guidance appended to the generated help topic:
	// worked examples, filter-syntax mappings, anything that cannot be derived
	// from the actions and props. Optional.
	Notes string
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

	if len(spec.Props) > 0 {
		b.WriteString("\nParameters:\n")
		names := make([]string, 0, len(spec.Props))
		for name := range spec.Props {
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
			prop := spec.Props[name]
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
func (spec ToolSpec) Define(srv *Server) (ToolDef, bool) {
	available := spec.Actions.Available(srv.WriteEnabled())
	if len(available) == 0 {
		return ToolDef{}, false
	}

	props := make(map[string]Prop, len(spec.Props)+1)
	for k, v := range spec.Props {
		props[k] = v
	}
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
				ReadOnlyHint:    !available.HasWrite(),
				DestructiveHint: &destructive,
			},
		},
		Handler: spec.Handler(srv),
	}, true
}

// Dispatch validates and gates an action before a handler runs it.
//
// The schema already hides write actions in read-only mode; this is the second
// line of defence, for a client that calls one anyway.
func Dispatch(srv *Server, actions ActionSet, name string) (string, *mcpsdk.CallToolResult) {
	a, ok := actions.Find(name)
	if !ok {
		available := actions.Available(srv.WriteEnabled())
		return "", ErrorResult(fmt.Sprintf(
			"unknown action %q; expected one of: %s",
			name, strings.Join(available.Names(), ", "),
		))
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
func DispatchWithDefault(srv *Server, actions ActionSet, name, fallback string) (string, *mcpsdk.CallToolResult) {
	if name == "" {
		name = fallback
	}
	return Dispatch(srv, actions, name)
}
