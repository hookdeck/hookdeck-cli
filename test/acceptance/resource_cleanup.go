package acceptance

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/slug"
)

// Acceptance tests create Hookdeck resources through the CLI and are supposed to
// delete them again. Most of them create three resources in one command —
// `gateway connection create --source-name … --destination-name …` creates a
// source and a destination alongside the connection — and then register cleanup
// for the connection alone. That is how the test projects accumulated tens of
// thousands of orphaned sources and destinations (#362).
//
// Patching every call site would fix the tests that exist today and none of the
// ones written tomorrow, so the bookkeeping lives in CLIRunner instead: every
// command that reports creating a gateway resource has that resource's id
// recorded, every command that deletes one has it struck off, and whatever is
// still on the list when the test finishes is deleted. Tests that already clean
// up after themselves cost nothing — their resources are struck off before the
// sweep runs, so the sweep issues no API calls for them.

// trackedKind is the CLI noun used to delete a tracked resource.
type trackedKind string

const (
	kindConnection     trackedKind = "connection"
	kindSource         trackedKind = "source"
	kindDestination    trackedKind = "destination"
	kindTransformation trackedKind = "transformation"
)

// sweepOrder is the order the sweep deletes in. Connections first: a source or
// destination that still has a connection attached may not be deletable.
var sweepOrder = []trackedKind{kindConnection, kindSource, kindDestination, kindTransformation}

// trackedResource is one resource an acceptance test created through the CLI.
type trackedResource struct {
	kind trackedKind
	id   string
}

// resourceTracker records what a runner created and what has since been deleted.
// A CLIRunner is shared by every command in one test, so access is mutex-guarded
// for the tests that run commands from more than one goroutine.
type resourceTracker struct {
	created []trackedResource
	seen    map[string]bool
	deleted map[string]bool
}

// gatewayResourceIDPattern matches the id formats the gateway API issues. Used
// both to spot ids in human-readable output and to pick the ids out of a delete
// command's arguments.
var gatewayResourceIDPattern = regexp.MustCompile(`\b(web|conn|src|des|tsf|trs)_[A-Za-z0-9]+\b`)

// kindForIDPrefix maps an id to the noun that deletes it. Ids whose prefix is
// not recognised are ignored rather than guessed at.
func kindForIDPrefix(id string) (trackedKind, bool) {
	switch {
	case strings.HasPrefix(id, "web_"), strings.HasPrefix(id, "conn_"):
		return kindConnection, true
	case strings.HasPrefix(id, "src_"):
		return kindSource, true
	case strings.HasPrefix(id, "des_"):
		return kindDestination, true
	case strings.HasPrefix(id, "tsf_"), strings.HasPrefix(id, "trs_"):
		return kindTransformation, true
	}
	return "", false
}

// gatewayCommandForTracking reports the resource noun and verb of a gateway
// command, plus the operands that follow the verb.
//
// It matches on position rather than by stripping flags, because a flag value
// can be any word: `--name source` must not read as the source command. The
// noun therefore has to be the first argument or directly follow `gateway`.
// Outpost commands never match, which matters — outpost ids are not gateway ids
// and `gateway destination delete` would be the wrong call for them.
func gatewayCommandForTracking(args []string) (noun trackedKind, verb string, operands []string, ok bool) {
	for i, arg := range args {
		var kind trackedKind
		switch arg {
		case "connection":
			kind = kindConnection
		case "source":
			kind = kindSource
		case "destination":
			kind = kindDestination
		case "transformation":
			kind = kindTransformation
		default:
			continue
		}
		if i > 0 && args[i-1] != "gateway" {
			continue
		}
		if i+1 >= len(args) {
			continue
		}
		return kind, args[i+1], args[i+2:], true
	}
	return "", "", nil, false
}

// createdResourcesFromOutput returns the resources a successful create or upsert
// reported. A connection create returns the source and destination it created
// inline; those are exactly the resources that used to be left behind.
func createdResourcesFromOutput(noun trackedKind, stdout string) []trackedResource {
	var payload struct {
		ID     string `json:"id"`
		Source struct {
			ID string `json:"id"`
		} `json:"source"`
		Destination struct {
			ID string `json:"id"`
		} `json:"destination"`
	}
	// The CLI sometimes prints a warning line before the JSON body (a rate-limited
	// spec download, say), so parsing starts at the first brace rather than at the
	// first byte. A test whose own parse fails on that warning still fails, but it
	// no longer leaks the resources it managed to create first.
	body := strings.TrimSpace(stdout)
	if brace := strings.Index(body, "{"); brace > 0 {
		body = body[brace:]
	}
	if err := json.Unmarshal([]byte(body), &payload); err == nil && payload.ID != "" {
		resources := []trackedResource{{kind: noun, id: payload.ID}}
		if payload.Source.ID != "" {
			resources = append(resources, trackedResource{kind: kindSource, id: payload.Source.ID})
		}
		if payload.Destination.ID != "" {
			resources = append(resources, trackedResource{kind: kindDestination, id: payload.Destination.ID})
		}
		return resources
	}

	// Without --output json the ids are printed in parentheses, as in
	// "Source:      my-source (src_…)". Anything else in the output that looks
	// like an id (a hint quoting an id that does not exist, say) is not in
	// parentheses and so is not picked up.
	var resources []trackedResource
	for _, match := range regexp.MustCompile(`\((web|conn|src|des|tsf|trs)_[A-Za-z0-9]+\)`).FindAllString(stdout, -1) {
		id := strings.Trim(match, "()")
		if kind, ok := kindForIDPrefix(id); ok {
			resources = append(resources, trackedResource{kind: kind, id: id})
		}
	}
	return resources
}

// recordCommandResources updates the tracker from one CLI command. Called for
// every command a runner executes; commands that neither create nor delete a
// gateway resource are ignored.
func (r *CLIRunner) recordCommandResources(args []string, stdout string, runErr error) {
	if r.tracker == nil || runErr != nil {
		return
	}
	noun, verb, operands, ok := gatewayCommandForTracking(args)
	if !ok {
		return
	}

	r.trackerMu.Lock()
	defer r.trackerMu.Unlock()

	switch verb {
	case "create", "upsert":
		for _, res := range createdResourcesFromOutput(noun, stdout) {
			if r.tracker.seen[res.id] {
				continue
			}
			r.tracker.seen[res.id] = true
			r.tracker.created = append(r.tracker.created, res)
		}
	case "delete":
		// Deletes take one or more ids; a delete by name leaves the id on the
		// list and the sweep finds it already gone, which costs one call and no
		// correctness.
		for _, operand := range operands {
			if strings.HasPrefix(operand, "-") {
				continue
			}
			if gatewayResourceIDPattern.MatchString(operand) {
				r.tracker.deleted[operand] = true
			}
		}
	}
}

// pendingResources returns what this runner created and has not deleted, in the
// order the sweep should delete it.
func (r *CLIRunner) pendingResources() []trackedResource {
	r.trackerMu.Lock()
	defer r.trackerMu.Unlock()

	var pending []trackedResource
	for _, kind := range sweepOrder {
		for _, res := range r.tracker.created {
			if res.kind == kind && !r.tracker.deleted[res.id] {
				pending = append(pending, res)
			}
		}
	}
	return pending
}

// sweepCreatedResources deletes everything the test created and did not delete.
// Registered by the runner constructors, so it runs after the cleanups the test
// registered itself and only has the leftovers to deal with.
func (r *CLIRunner) sweepCreatedResources() {
	if r.tracker == nil {
		return
	}
	pending := r.pendingResources()
	if len(pending) == 0 {
		return
	}

	var deleted, alreadyGone, failed int
	for _, res := range pending {
		stdout, stderr, err := r.Run("gateway", string(res.kind), "delete", res.id, "--force")
		switch {
		case err == nil:
			deleted++
		case resourceAlreadyGone(stdout + stderr):
			alreadyGone++
		default:
			failed++
			r.t.Logf("acceptance cleanup: could not delete %s %s: %v\nstdout: %s\nstderr: %s",
				res.kind, res.id, err, stdout, stderr)
		}
	}
	r.t.Logf("acceptance cleanup: deleted %d resource(s) the test left behind (%d already gone, %d failed)",
		deleted, alreadyGone, failed)
}

// resourceAlreadyGone reports whether a delete failed because the resource was
// not there — the expected outcome when the test deleted it by name, or when
// deleting a source took its connections with it.
func resourceAlreadyGone(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "not found") ||
		strings.Contains(lower, "404") ||
		strings.Contains(lower, "does not exist")
}

// listenSourceNames returns the source names a `hookdeck listen` argv asks for.
// The signature is `listen <port> [source] [path]`, where source may be a
// comma-separated list. A wildcard listens to every existing source and creates
// nothing, so it is not returned.
func listenSourceNames(args []string) []string {
	listenAt := -1
	for i, arg := range args {
		if arg == "listen" {
			listenAt = i
			break
		}
	}
	if listenAt < 0 {
		return nil
	}

	var operands []string
	for _, arg := range args[listenAt+1:] {
		if strings.HasPrefix(arg, "-") {
			break
		}
		operands = append(operands, arg)
	}
	if len(operands) < 2 {
		return nil
	}

	var names []string
	for _, name := range strings.Split(operands[1], ",") {
		name = strings.TrimSpace(name)
		if name == "" || name == "*" {
			continue
		}
		names = append(names, name)
	}
	return names
}

// registerListenCleanup arranges for the resources `hookdeck listen` creates on
// the fly to be deleted when the test finishes. Register it before anything that
// stops the listen process, so it runs after that process is gone.
func registerListenCleanup(t *testing.T, cli *CLIRunner, args []string) {
	t.Helper()
	for _, name := range listenSourceNames(args) {
		sourceName := name
		t.Cleanup(func() { cleanupListenResources(t, cli, sourceName) })
	}
}

// cleanupListenResources deletes what `hookdeck listen` creates when the source
// it is pointed at does not exist yet: the source itself, and the cli-<source>
// connection and destination the tunnel adds when that source has no CLI
// destination. The listen process creates these itself, so no id ever reaches
// the runner's bookkeeping — the test knows only the name it passed.
func cleanupListenResources(t *testing.T, cli *CLIRunner, sourceName string) {
	t.Helper()

	// listen slugifies the name before creating the source, so a name that is
	// not already a slug is stored under a different one.
	name := slug.Make(sourceName)
	destinationName := "cli-" + name

	var sources struct {
		Models []Source `json:"models"`
	}
	if err := cli.RunJSON(&sources, "gateway", "source", "list", "--name", name); err != nil {
		t.Logf("acceptance cleanup: could not list sources named %s: %v", name, err)
	}
	for _, source := range sources.Models {
		if source.Name != name {
			continue
		}
		var connections ConnectionListResponse
		if err := cli.RunJSON(&connections, "gateway", "connection", "list", "--source-id", source.ID); err != nil {
			t.Logf("acceptance cleanup: could not list connections for source %s: %v", source.ID, err)
		}
		for _, connection := range connections.Models {
			deleteConnection(t, cli, connection.ID)
			if connection.Destination.ID != "" {
				deleteDestination(t, cli, connection.Destination.ID)
			}
		}
		deleteSource(t, cli, source.ID)
	}

	// listen can create the source and stop before creating the connection, so
	// the destination is looked up by name rather than only through connections.
	var destinations struct {
		Models []Destination `json:"models"`
	}
	if err := cli.RunJSON(&destinations, "gateway", "destination", "list", "--name", destinationName); err != nil {
		t.Logf("acceptance cleanup: could not list destinations named %s: %v", destinationName, err)
		return
	}
	for _, destination := range destinations.Models {
		if destination.Name == destinationName {
			deleteDestination(t, cli, destination.ID)
		}
	}
}
