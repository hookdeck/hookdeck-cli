// Command credential-matrix probes every platform API route with each class of
// Hookdeck credential and prints what each one is actually allowed to do.
//
// It exists because the OpenAPI document cannot answer the question. Auth is
// declared once, globally, as bearerAuth/basicAuth with no per-operation
// override anywhere in 95 paths — and those schemes describe how a key is sent,
// not which class of key it is. The class is what decides 200 from 401, and
// only seven operations mention it at all, in prose. GET /organizations/current
// says nothing, and it is one of the routes that rejects a CLI session.
//
// So the runtime is the only authority, and this asks it.
//
// Every by-id route is probed against a record that really exists, created for
// the run and deleted at the end. An earlier version used ids that could not
// exist and read the resulting 404 as "auth passed, handler found nothing".
// That inference is false: these routes resolve the id BEFORE authorizing, so a
// fake id returns 404 to a credential that a real id would have rejected with
// 401. It made a project API key look able to PUT and DELETE any project in the
// organization, when it is in fact refused on its own. Fake ids are gone; there
// is no read of a response that can substitute for using a real record.
//
// Consequently this tool creates and destroys real records and must only ever
// be pointed at a throwaway organization.
//
// Budget: a run spends roughly eleven "user management actions" — a project and
// an API key per column, plus a shared foreign project. Free-tier organizations
// allow thirty per five hours, so three runs in a sitting is the ceiling and the
// fourth fails partway through setup with a 429.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hookdeck/hookdeck-cli/pkg/useragent"
)

const defaultBaseURL = "https://api.hookdeck.com/2026-09-01"

// probeDomain is a custom domain the API accepts. The hostname is validated
// against the public suffix list, so example.com and .test are both refused —
// with a 429 CUSTOM_DOMAIN_INVALID, which is a validation error wearing a rate
// limit's status code.
//
// Hostnames are unique across the whole service, not per project, so a fixed
// one fails with 409 the moment a second project wants one — including on a
// re-run after a crash left the first behind. Each call returns a fresh label.
func probeDomain() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return fmt.Sprintf("cm-%s.hookdeck-probe.dev", hex.EncodeToString(b[:]))
}

// fixturePrefix labels every record this tool creates, so cleanup can find
// them again without having tracked them.
const fixturePrefix = "cred-matrix"

// credential is one class of key to test.
type credential struct {
	name     string
	envVar   string
	value    string
	note     string
	identity string // what /cli-auth/validate says this key is

	// Real records this credential is probed against. scratch is created for
	// it alone, so a DELETE in one column cannot disturb another. home is the
	// project the credential itself belongs to: the project key's scope, the
	// CLI session's pin, and for an organization key simply its scratch.
	scratch string
	home    string
	apiKey  string
	domain  string
}

type outcome struct {
	status  int
	message string
	raw     []byte
	err     error
}

// probe is one request to make.
type probe struct {
	label  string
	method string
	path   string
	body   string
}

func main() {
	baseURL := flag.String("base-url", defaultBaseURL, "API base URL")
	confirm := flag.Bool("throwaway-org", false,
		"required: confirms the organization behind these credentials is disposable. "+
			"The run creates and deletes projects, API keys and custom domains.")
	flag.Parse()

	loadDotEnv()
	creds := loadCredentials()
	if len(creds) == 0 {
		fmt.Fprintln(os.Stderr, "No credentials found. Set at least one of:")
		for _, c := range credentialSpecs() {
			fmt.Fprintf(os.Stderr, "  %-28s %s\n", c.envVar, c.note)
		}
		os.Exit(1)
	}
	if !*confirm {
		fmt.Fprintln(os.Stderr, "Refusing to run without --throwaway-org.")
		fmt.Fprintln(os.Stderr, "This tool creates and deletes real projects, API keys and custom")
		fmt.Fprintln(os.Stderr, "domains. Point it at a disposable organization and pass the flag.")
		os.Exit(1)
	}

	org := findOrg(creds)
	if org == nil {
		fmt.Fprintln(os.Stderr, "HOOKDECK_ORG_API_KEY is required: the fixtures every other")
		fmt.Fprintln(os.Stderr, "column is probed against are created with it.")
		os.Exit(1)
	}

	fmt.Printf("Base URL: %s\n\n", *baseURL)

	creds, foreign, cleanup, err := setUp(*baseURL, creds, *org)
	// os.Exit does not run deferred functions, so the failure path has to tear
	// the fixtures down itself. Getting this wrong leaks real projects and API
	// keys into the organization, which is exactly what happened the first
	// time setup failed.
	defer cleanup()
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup failed: %v\n", err)
		cleanup()
		os.Exit(1)
	}

	fmt.Printf("\nCredentials under test: %d\n\n", len(creds))
	for i := range creds {
		// Never print a key. A fingerprint is enough to tell two apart.
		fmt.Printf("  %-22s %s (len %d, %s…)\n",
			creds[i].name, creds[i].envVar, len(creds[i].value), safePrefix(creds[i].value))
		fmt.Printf("  %-22s %s\n\n", "", creds[i].identity)
	}

	// Each column is run to completion before the next, because a column's
	// probes mutate and finally delete its own scratch project.
	results := make(map[string]map[string]string)
	var labels []string
	for _, c := range creds {
		for _, p := range probesFor(c, foreign) {
			if _, seen := results[p.label]; !seen {
				results[p.label] = map[string]string{}
				labels = append(labels, p.label)
			}
			results[p.label][c.name] = classify(call(*baseURL, c, p))
			// The API rate-limits; probing is not urgent.
			time.Sleep(250 * time.Millisecond)
		}
	}
	printMarkdown(creds, results, labels)
}

// setUp creates one scratch project per credential, an API key and a custom
// domain inside it, and mints the project-key column if none was supplied.
//
// The returned cleanup always runs, including on a failed setup, so a partial
// run does not strand records in the organization.
func setUp(baseURL string, creds []credential, org credential) ([]credential, string, func(), error) {
	var undo []func()
	var cleaned bool
	cleanup := func() {
		if cleaned {
			return
		}
		cleaned = true
		fmt.Println("\nCleaning up:")
		for i := len(undo) - 1; i >= 0; i-- {
			undo[i]()
		}
		sweep(baseURL, org)
	}

	// A project the organization already had, so the organization key is
	// probed against something it did not just create.
	existing, err := firstProjectID(baseURL, org)
	if err != nil {
		return creds, "", cleanup, err
	}

	mintProject := func(name string) (string, error) {
		out := call(baseURL, org, probe{
			method: "POST", path: "/projects", body: fmt.Sprintf(`{"name":%q}`, name)})
		if out.err != nil {
			return "", out.err
		}
		if out.status < 200 || out.status >= 300 {
			return "", fmt.Errorf("creating project %s returned %d: %s", name, out.status, out.message)
		}
		var created struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(out.raw, &created); err != nil || created.ID == "" {
			return "", fmt.Errorf("creating project %s returned no id", name)
		}
		undo = append(undo, func() {
			del := call(baseURL, org, probe{method: "DELETE", path: "/projects/" + created.ID})
			// A column's own DELETE probe may already have removed it.
			if del.status == 404 {
				fmt.Printf("  project %s already deleted by its own probe\n", created.ID)
				return
			}
			report("project "+created.ID, del)
		})
		return created.ID, nil
	}

	mintKey := func(projectID, label string) (id, value string, err error) {
		out := call(baseURL, org, probe{
			method: "POST", path: "/organizations/current/api-keys",
			body: fmt.Sprintf(`{"label":%q,"type":"project","team_id":%q}`, label, projectID)})
		if out.err != nil {
			return "", "", out.err
		}
		if out.status < 200 || out.status >= 300 {
			return "", "", fmt.Errorf("creating api key returned %d: %s", out.status, out.message)
		}
		var created struct {
			ID  string `json:"id"`
			Key string `json:"key"`
		}
		if err := json.Unmarshal(out.raw, &created); err != nil || created.Key == "" {
			return "", "", fmt.Errorf("creating api key returned no key")
		}
		// A leaked credential is a worse outcome than a missing column, so the
		// delete is reported loudly rather than swallowed.
		undo = append(undo, func() {
			del := call(baseURL, org, probe{
				method: "DELETE", path: "/organizations/current/api-keys/" + created.ID})
			if del.status == 404 {
				fmt.Printf("  api key %s already deleted by its own probe\n", created.ID)
				return
			}
			if del.err != nil || del.status < 200 || del.status >= 300 {
				fmt.Fprintf(os.Stderr,
					"\n!! FAILED to delete temporary key %s (status %d, %v).\n"+
						"   Delete it by hand: hookdeck org api-key delete %s --api-key $HOOKDECK_ORG_API_KEY --force\n",
					created.ID, del.status, del.err, created.ID)
				return
			}
			fmt.Printf("  api key %s deleted\n", created.ID)
		})
		return created.ID, created.Key, nil
	}

	// A project no column owns, so `{other}` is a real record the credential
	// has no claim on rather than one of its own fixtures.
	foreign, err := mintProject(fixturePrefix + " foreign project")
	if err != nil {
		return creds, "", cleanup, err
	}

	// Mint the project-key column when it was not supplied. The organization
	// key can only create project keys, so this is the one class that does not
	// have to be pasted in by hand — which also keeps the column reproducible
	// rather than dependent on whichever key someone had lying around.
	if findByEnv(creds, "HOOKDECK_PROJECT_API_KEY") == nil {
		p, err := mintProject(fixturePrefix + " scratch (project key)")
		if err != nil {
			return creds, "", cleanup, err
		}
		id, value, err := mintKey(p, fixturePrefix+" project column")
		if err != nil {
			return creds, "", cleanup, err
		}
		fmt.Printf("Minted project API key %s scoped to %s for the project-key column.\n", id, p)
		creds = append(creds, credential{
			name:   "Project API key",
			envVar: "(minted for this run)",
			value:  value,
			// Its scope IS this project, so "home" and "scratch" coincide and
			// the write probes land on the project it owns — which is the
			// question worth asking of a project-scoped key.
			scratch: p,
			home:    p,
		})
	}

	// A project-bound CLI key is a fourth class, not a variant of the third:
	// `hookdeck ci` exchanges a project API key for one via POST /cli-auth/ci,
	// and the result authenticates as a CLI session (user_id null) rather than
	// as the API key it came from. The two reach different routes, so probing
	// only the API key would leave the class CI actually runs on untested.
	if findByEnv(creds, "HOOKDECK_PROJECT_CLI_KEY") == nil {
		p, err := mintProject(fixturePrefix + " scratch (project CLI key)")
		if err != nil {
			return creds, "", cleanup, err
		}
		id, value, err := mintKey(p, fixturePrefix+" project CLI exchange")
		if err != nil {
			return creds, "", cleanup, err
		}
		exchanged, err := exchangeForCIKey(baseURL, value)
		if err != nil {
			return creds, "", cleanup, err
		}
		fmt.Printf("Exchanged project API key %s for a project-bound CLI key on %s.\n", id, p)
		creds = append(creds, credential{
			name:    "Project CLI key",
			envVar:  "(minted for this run)",
			value:   exchanged,
			scratch: p,
			home:    p,
		})
	}

	for i := range creds {
		c := &creds[i]
		c.identity = describe(baseURL, *c)

		if c.scratch == "" {
			p, err := mintProject(fixturePrefix + " scratch (" + c.name + ")")
			if err != nil {
				return creds, "", cleanup, err
			}
			c.scratch = p
		}
		if c.home == "" {
			// A CLI session names the project it is pinned to; an organization
			// key belongs to no single project, so it is probed against one the
			// organization already had.
			if pinned := pinnedProject(baseURL, *c); pinned != "" {
				c.home = pinned
			} else {
				c.home = existing
			}
		}

		id, _, err := mintKey(c.scratch, fixturePrefix+" target ("+c.name+")")
		if err != nil {
			return creds, "", cleanup, err
		}
		c.apiKey = id

		c.domain, err = mintDomain(baseURL, org, c.scratch, &undo)
		if err != nil {
			return creds, "", cleanup, err
		}
	}
	return creds, foreign, cleanup, nil
}

// exchangeForCIKey runs the `hookdeck ci` exchange: a project API key in,
// a project-bound CLI session key out. The returned key is revoked when the
// project it belongs to is deleted during cleanup.
func exchangeForCIKey(baseURL, projectAPIKey string) (string, error) {
	out := call(baseURL, credential{value: projectAPIKey},
		probe{method: "POST", path: "/cli-auth/ci"})
	if out.err != nil {
		return "", out.err
	}
	if out.status < 200 || out.status >= 300 {
		return "", fmt.Errorf("POST /cli-auth/ci returned %d: %s", out.status, out.message)
	}
	var v struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(out.raw, &v); err != nil || v.Key == "" {
		return "", fmt.Errorf("POST /cli-auth/ci returned no key")
	}
	return v.Key, nil
}

func mintDomain(baseURL string, org credential, projectID string, undo *[]func()) (string, error) {
	out := call(baseURL, org, probe{
		method: "POST", path: "/projects/" + projectID + "/custom_domains",
		body: fmt.Sprintf(`{"hostname":%q}`, probeDomain())})
	if out.err != nil {
		return "", out.err
	}
	if out.status < 200 || out.status >= 300 {
		return "", fmt.Errorf("creating custom domain returned %d: %s", out.status, out.message)
	}
	// The create echoes the hostname only; the id comes from the listing.
	list := call(baseURL, org, probe{method: "GET", path: "/projects/" + projectID + "/custom_domains"})
	var domains []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(list.raw, &domains); err != nil || len(domains) == 0 {
		return "", fmt.Errorf("custom domain created but not listed")
	}
	id := domains[0].ID
	*undo = append(*undo, func() {
		del := call(baseURL, org, probe{
			method: "DELETE", path: "/projects/" + projectID + "/custom_domains/" + id})
		// Deleting the project removes its domains, so a 404 here is expected
		// whenever the project went first.
		if del.status == 404 {
			return
		}
		report("custom domain "+id, del)
	})
	return id, nil
}

// sweep deletes everything carrying fixturePrefix, and is the backstop the
// undo stack cannot be.
//
// Probes create records of their own — POST /organizations/current/api-keys
// leaves a key behind on success — and those have no undo registered because
// nothing knows in advance which columns will succeed. A run that dies partway
// leaves fixtures the same way. Listing by label catches both, and catches the
// residue of previous runs too, so the organization converges on clean rather
// than accumulating.
func sweep(baseURL string, org credential) {
	keys := call(baseURL, org, probe{method: "GET", path: "/organizations/current/api-keys"})
	var ks []struct {
		ID    string `json:"id"`
		Label string `json:"label"`
	}
	if err := json.Unmarshal(keys.raw, &ks); err != nil {
		fmt.Fprintf(os.Stderr, "  !! could not list api keys to sweep (%d): %v\n", keys.status, err)
	} else {
		for _, k := range ks {
			if !strings.HasPrefix(k.Label, fixturePrefix) {
				continue
			}
			del := call(baseURL, org, probe{
				method: "DELETE", path: "/organizations/current/api-keys/" + k.ID})
			report("swept api key "+k.ID, del)
		}
	}

	projects := call(baseURL, org, probe{method: "GET", path: "/projects"})
	var ps []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(projects.raw, &ps); err != nil {
		fmt.Fprintf(os.Stderr, "  !! could not list projects to sweep (%d): %v\n", projects.status, err)
	} else {
		for _, pr := range ps {
			if !strings.HasPrefix(pr.Name, fixturePrefix) {
				continue
			}
			del := call(baseURL, org, probe{method: "DELETE", path: "/projects/" + pr.ID})
			report("swept project "+pr.ID, del)
		}
	}
}

func report(what string, o outcome) {
	if o.err != nil || o.status < 200 || o.status >= 300 {
		fmt.Fprintf(os.Stderr, "  !! failed to delete %s (status %d, %v)\n", what, o.status, o.err)
		return
	}
	fmt.Printf("  %s deleted\n", what)
}

// probesFor builds the request list for one credential against its own real
// records. The project delete comes last because it destroys the fixtures the
// earlier probes need.
func probesFor(c credential, foreign string) []probe {
	// A no-op body: PUT /organizations/current replaces the record, so it is
	// sent the name the organization already has.
	orgBody := fmt.Sprintf(`{"name":%q}`, currentOrgName)

	return []probe{
		{label: "GET /cli-auth/validate", method: "GET", path: "/cli-auth/validate"},
		{label: "POST /cli-auth/ci", method: "POST", path: "/cli-auth/ci"},

		{label: "GET /projects", method: "GET", path: "/projects"},
		{label: "GET /projects/{own}", method: "GET", path: "/projects/" + c.home},
		{label: "GET /projects/{other}", method: "GET", path: "/projects/" + foreign},
		{label: "PUT /projects/{own}", method: "PUT", path: "/projects/" + c.scratch,
			body: `{"name":"cred-matrix renamed"}`},

		{label: "GET /projects/{own}/custom_domains", method: "GET",
			path: "/projects/" + c.scratch + "/custom_domains"},
		{label: "DELETE /projects/{own}/custom_domains/{id}", method: "DELETE",
			path: "/projects/" + c.scratch + "/custom_domains/" + c.domain},
		{label: "POST /projects/{own}/custom_domains", method: "POST",
			path: "/projects/" + c.scratch + "/custom_domains",
			body: fmt.Sprintf(`{"hostname":%q}`, probeDomain())},

		{label: "GET /organizations/current", method: "GET", path: "/organizations/current"},
		{label: "PUT /organizations/current", method: "PUT", path: "/organizations/current", body: orgBody},

		{label: "GET /organizations/current/api-keys", method: "GET",
			path: "/organizations/current/api-keys"},
		{label: "POST /organizations/current/api-keys", method: "POST",
			path: "/organizations/current/api-keys",
			body: fmt.Sprintf(`{"label":"%s create probe","type":"project","team_id":%q}`,
				fixturePrefix, c.scratch)},
		// PUT accepts scopes and grants only — not label — and rejects an
		// empty object outright, so a no-op body has to name a field. roll
		// requires delay_sec. Sending the wrong shape returns 422, which reads
		// as an authorization answer in the table when it is really our bug.
		{label: "PUT /organizations/current/api-keys/{id}", method: "PUT",
			path: "/organizations/current/api-keys/" + c.apiKey,
			body: `{"scopes":[]}`},
		{label: "POST /organizations/current/api-keys/{id}/roll", method: "POST",
			path: "/organizations/current/api-keys/" + c.apiKey + "/roll",
			body: `{"delay_sec":0}`},
		{label: "DELETE /organizations/current/api-keys/{id}", method: "DELETE",
			path: "/organizations/current/api-keys/" + c.apiKey},

		{label: "DELETE /projects/{own}", method: "DELETE", path: "/projects/" + c.scratch},
	}
}

// currentOrgName is read once during setup so PUT /organizations/current can be
// a no-op rather than renaming the organization under test.
var currentOrgName = "throwaway"

// describe asks the API what a credential is, so the run reports the classes it
// actually tested rather than the labels someone typed.
//
// user_name is the field that separates them. /cli-auth/validate does NOT
// return user_id or user_email — classifying on those silently labels every
// credential project-bound, which is a fabricated finding, not a null result.
// team_id says which project the session is currently pinned to; it is set for
// user-bound sessions too, so it does not distinguish the classes on its own.
func describe(baseURL string, c credential) string {
	out := call(baseURL, c, probe{method: "GET", path: "/cli-auth/validate"})
	if out.err != nil {
		return "identity unknown: " + out.err.Error()
	}
	if out.status == 401 {
		return "-> /cli-auth/validate rejects it (401): not a CLI-session credential"
	}
	if out.status < 200 || out.status >= 300 {
		return fmt.Sprintf("-> /cli-auth/validate returned %d: %s", out.status, out.message)
	}

	var v struct {
		UserName         string `json:"user_name"`
		ClientID         string `json:"client_id"`
		OrganizationName string `json:"organization_name"`
		ProjectID        string `json:"team_id"`
		ProjectName      string `json:"team_name_no_org"`
	}
	if err := json.Unmarshal(out.raw, &v); err != nil {
		return "-> /cli-auth/validate returned an unreadable body"
	}

	switch {
	case v.UserName != "" && v.ProjectID == "":
		return fmt.Sprintf("-> USER-bound (user %s), no project pinned", v.UserName)
	case v.UserName != "":
		return fmt.Sprintf("-> USER-bound (user %s), currently pinned to project %s in %s",
			v.UserName, v.ProjectName, v.OrganizationName)
	case v.ProjectID != "":
		return fmt.Sprintf("-> PROJECT-bound (no user_name), project %s in %s",
			v.ProjectName, v.OrganizationName)
	default:
		return "-> accepted by /cli-auth/validate but neither user nor project identified"
	}
}

// pinnedProject returns the project a CLI session is attached to, or "".
func pinnedProject(baseURL string, c credential) string {
	out := call(baseURL, c, probe{method: "GET", path: "/cli-auth/validate"})
	if out.status < 200 || out.status >= 300 {
		return ""
	}
	var v struct {
		ProjectID string `json:"team_id"`
	}
	if err := json.Unmarshal(out.raw, &v); err != nil {
		return ""
	}
	return v.ProjectID
}

// firstProjectID returns a project the organization already had, and records
// the organization's own name for the no-op PUT.
func firstProjectID(baseURL string, org credential) (string, error) {
	if out := call(baseURL, org, probe{method: "GET", path: "/organizations/current"}); out.status == 200 {
		var o struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(out.raw, &o) == nil && o.Name != "" {
			currentOrgName = o.Name
		}
	}

	out := call(baseURL, org, probe{method: "GET", path: "/projects"})
	if out.err != nil {
		return "", out.err
	}
	if out.status < 200 || out.status >= 300 {
		return "", fmt.Errorf("listing projects returned %d: %s", out.status, out.message)
	}
	var projects []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out.raw, &projects); err != nil {
		return "", err
	}
	if len(projects) == 0 {
		return "", fmt.Errorf("the organization has no projects")
	}
	return projects[0].ID, nil
}

// dotEnvPaths are searched in order; the first that exists is loaded.
//
// The tool's own directory comes first: these credentials belong to whatever
// throwaway organization this is pointed at, and keeping them beside the tool
// stops them being confused with the acceptance suite's keys, which belong to a
// different account and are used for something else.
//
// Both spellings are listed because `go run ./tools/credential-matrix` runs
// from the repository root while running the built binary from the tool's own
// directory does not. Values already in the environment win, so an export still
// overrides the file.
var dotEnvPaths = []string{
	"tools/credential-matrix/.env",
	".env",
}

func loadDotEnv() {
	for _, path := range dotEnvPaths {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(strings.TrimPrefix(key, "export "))
			value = strings.Trim(strings.TrimSpace(value), `"'`)
			if _, already := os.LookupEnv(key); !already {
				_ = os.Setenv(key, value)
			}
		}
		fmt.Printf("Loaded credentials from %s\n", path)
		return
	}
}

func credentialSpecs() []credential {
	return []credential{
		{name: "Org API key", envVar: "HOOKDECK_ORG_API_KEY",
			note: "REQUIRED — organization API key with projects.*, organizations.* and api-keys.* scopes"},
		{name: "Project API key", envVar: "HOOKDECK_PROJECT_API_KEY",
			note: "OPTIONAL — one is minted with the organization key, and deleted afterwards"},
		{name: "Project CLI key", envVar: "HOOKDECK_PROJECT_CLI_KEY",
			note: "OPTIONAL — one is minted via POST /cli-auth/ci, the `hookdeck ci` exchange"},
		{name: "CLI session key", envVar: "HOOKDECK_CLI_SESSION_KEY",
			note: "the api_key stored in config.toml after `hookdeck login` in a browser"},
	}
}

func loadCredentials() []credential {
	var out []credential
	for _, c := range credentialSpecs() {
		if v := strings.TrimSpace(os.Getenv(c.envVar)); v != "" {
			c.value = v
			out = append(out, c)
		}
	}
	return out
}

func findOrg(creds []credential) *credential { return findByEnv(creds, "HOOKDECK_ORG_API_KEY") }

func findByEnv(creds []credential, env string) *credential {
	for i := range creds {
		if creds[i].envVar == env {
			return &creds[i]
		}
	}
	return nil
}

func safePrefix(key string) string {
	if i := strings.LastIndex(key, "_"); i > 0 && i < 12 {
		return key[:i+1]
	}
	if len(key) > 4 {
		return key[:4]
	}
	return "?"
}

func call(baseURL string, c credential, p probe) outcome {
	var body io.Reader
	if p.body != "" {
		body = strings.NewReader(p.body)
	}
	req, err := http.NewRequest(p.method, baseURL+p.path, body)
	if err != nil {
		return outcome{err: err}
	}
	// Basic auth with the key as the username is how the CLI authenticates.
	req.SetBasicAuth(c.value, "")
	req.Header.Set("Content-Type", "application/json")

	// The User-Agent is load-bearing, not cosmetic. A CLI session key is
	// rejected with 401 when the request does not identify itself as the CLI —
	// the same key and URL returns 200 with this header and 401 with
	// Go-http-client/2.0. Without it every CLI-key column reads as "rejected",
	// which is a false negative that looks exactly like a real finding.
	//
	// Taken from the package the CLI itself uses, so the two cannot drift.
	req.Header.Set("User-Agent", useragent.GetEncodedUserAgent())
	req.Header.Set("X-Hookdeck-Client-User-Agent", useragent.GetEncodedHookdeckUserAgent())

	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return outcome{err: err}
	}
	defer resp.Body.Close()

	// The cap has to clear a full listing, not just an error body. At 4096 the
	// api-keys listing came back mid-object, every Unmarshal of it failed, and
	// the cleanup sweep that depends on it silently found nothing to delete.
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return outcome{status: resp.StatusCode, message: apiMessage(raw), raw: raw}
}

func apiMessage(raw []byte) string {
	// data.required names the exact scope a 403 is missing. Without it every
	// scope failure renders as the same sentence and an unscoped key is
	// indistinguishable from a route that refuses the credential class.
	var payload struct {
		Message string `json:"message"`
		Data    struct {
			Required string `json:"required"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err == nil && payload.Message != "" {
		if payload.Data.Required != "" {
			return fmt.Sprintf("%s (requires %s)", payload.Message, payload.Data.Required)
		}
		return payload.Message
	}
	text := strings.TrimSpace(string(raw))
	if len(text) > 90 {
		text = text[:90] + "…"
	}
	return text
}

// classify reduces a response to what the question is actually about: did this
// credential get past authentication?
//
// 404 is deliberately NOT reported as success. Every probe here names a record
// that exists, so a 404 means the record is invisible to this credential —
// scoped out rather than found. That is a different answer from 401 and worth
// seeing as itself.
func classify(o outcome) string {
	switch {
	case o.err != nil:
		return "error: " + o.err.Error()
	case o.status == 401:
		return "**401 rejected**"
	case o.status == 403:
		return "403 — " + o.message
	case o.status == 404:
		return "404 not visible"
	case o.status >= 200 && o.status < 300:
		return "**OK**"
	default:
		return fmt.Sprintf("%d — %s", o.status, o.message)
	}
}

func printMarkdown(creds []credential, results map[string]map[string]string, labels []string) {
	head := "| Endpoint |"
	sep := "|---|"
	for _, c := range creds {
		head += " " + c.name + " |"
		sep += "---|"
	}
	fmt.Println()
	fmt.Println(head)
	fmt.Println(sep)
	for _, l := range labels {
		row := "| `" + l + "` |"
		for _, c := range creds {
			cell := results[l][c.name]
			if cell == "" {
				cell = "—"
			}
			row += " " + cell + " |"
		}
		fmt.Println(row)
	}

	fmt.Println()
	fmt.Println("Every by-id probe names a record that exists, created for this run.")
	fmt.Println("`{own}` is the credential's own project — a project key's scope, a CLI")
	fmt.Println("session's pin. `{other}` is a project belonging to the same organization")
	fmt.Println("that the credential has no claim on.")
	fmt.Println()
	fmt.Println("`404 not visible` means the record exists but this credential cannot see it,")
	fmt.Println("which is scoping, not absence. It is NOT evidence that authentication passed.")
}
