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
// Safety: by default nothing is created or destroyed. By-id routes are probed
// with ids that cannot exist, so a 404 proves the credential got past auth to a
// handler that then found nothing. Creates are skipped unless --destructive is
// passed, because "invalid input will 422 before anything is made" is a guess:
// probing with an over-long label once turned out to be under the real limit,
// and only counting rows afterwards proved nothing had been created.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.hookdeck.com/2026-09-01"

// credential is one class of key to test.
type credential struct {
	name   string
	envVar string
	value  string
	note   string
}

// probe is one request to make.
type probe struct {
	label       string
	method      string
	path        string
	body        string
	destructive bool // only run with --destructive
}

func main() {
	baseURL := flag.String("base-url", defaultBaseURL, "API base URL")
	destructive := flag.Bool("destructive", false,
		"also run probes that create or delete real records. Only point this at a throwaway organization.")
	markdown := flag.Bool("markdown", true, "print a Markdown table")
	noMint := flag.Bool("no-mint", false,
		"do not create a temporary project API key, even when an organization key is available")
	flag.Parse()

	loadDotEnv()

	creds := loadCredentials()

	// An organization key can mint a project key — "Organization API keys can
	// only create project keys", per the API's own description — so there is no
	// need to be handed one. Minting it here also makes the project-key column
	// reproducible instead of depending on whichever key someone happened to
	// paste.
	if !*noMint {
		if minted, cleanup, err := mintProjectKey(*baseURL, creds); err != nil {
			fmt.Fprintf(os.Stderr, "could not mint a project key: %v\n", err)
			fmt.Fprintln(os.Stderr, "continuing without that column")
		} else if minted != nil {
			creds = append(creds, *minted)
			defer cleanup()
		}
	}

	if len(creds) == 0 {
		fmt.Fprintln(os.Stderr, "No credentials found. Set at least one of:")
		for _, c := range credentialSpecs() {
			fmt.Fprintf(os.Stderr, "  %-28s %s\n", c.envVar, c.note)
		}
		os.Exit(1)
	}

	fmt.Printf("Base URL: %s\n", *baseURL)
	fmt.Printf("Credentials under test: %d\n", len(creds))
	for _, c := range creds {
		// Never print a key. A fingerprint is enough to tell two apart.
		fmt.Printf("  %-22s %s (len %d, %s…)\n", c.name, c.envVar, len(c.value), safePrefix(c.value))
	}
	if *destructive {
		fmt.Println("\n!! --destructive: creates and deletes real records. Throwaway org only.")
	} else {
		fmt.Println("\nRead-only probes. Pass --destructive to include create/delete.")
	}
	fmt.Println()

	results := make(map[string]map[string]string)
	for _, p := range probes() {
		if p.destructive && !*destructive {
			continue
		}
		results[p.label] = map[string]string{}
		for _, c := range creds {
			results[p.label][c.name] = classify(call(*baseURL, c, p))
			// The API rate-limits; probing is not urgent.
			time.Sleep(250 * time.Millisecond)
		}
	}

	if *markdown {
		printMarkdown(creds, results, *destructive)
	}
}

// mintProjectKey creates a short-lived project API key with the organization
// key, so the project-key column does not have to be supplied by hand.
//
// It returns a cleanup that deletes the key again. The caller must run it: a
// leaked credential is a worse outcome than a missing column, so a failed
// cleanup is reported loudly rather than swallowed.
func mintProjectKey(baseURL string, creds []credential) (*credential, func(), error) {
	var org *credential
	for i := range creds {
		if creds[i].envVar == "HOOKDECK_ORG_API_KEY" {
			org = &creds[i]
		}
		if creds[i].envVar == "HOOKDECK_PROJECT_API_KEY" {
			return nil, nil, nil // one was supplied; nothing to do
		}
	}
	if org == nil {
		return nil, nil, nil
	}

	projectID, err := firstProjectID(baseURL, *org)
	if err != nil {
		return nil, nil, err
	}

	label := fmt.Sprintf("credential-matrix probe %s", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	body := fmt.Sprintf(`{"label":%q,"type":"project","team_id":%q}`, label, projectID)
	out := call(baseURL, *org, probe{method: "POST", path: "/organizations/current/api-keys", body: body})
	if out.err != nil {
		return nil, nil, out.err
	}
	if out.status < 200 || out.status >= 300 {
		return nil, nil, fmt.Errorf("create returned %d: %s", out.status, out.message)
	}

	var created struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	if err := json.Unmarshal(out.raw, &created); err != nil || created.Key == "" {
		return nil, nil, fmt.Errorf("create succeeded but no key came back")
	}

	fmt.Printf("Minted a temporary project API key (%s) on project %s; it is deleted at the end.\n",
		created.ID, projectID)

	cleanup := func() {
		del := call(baseURL, *org, probe{
			method: "DELETE", path: "/organizations/current/api-keys/" + created.ID})
		if del.err != nil || del.status < 200 || del.status >= 300 {
			fmt.Fprintf(os.Stderr,
				"\n!! FAILED to delete the temporary key %s (status %d, %v).\n"+
					"   Delete it by hand: hookdeck org api-key delete %s --api-key $HOOKDECK_ORG_API_KEY --force\n",
				created.ID, del.status, del.err, created.ID)
			return
		}
		fmt.Printf("\nDeleted the temporary project API key %s.\n", created.ID)
	}

	return &credential{
		name:   "Project API key",
		envVar: "(minted for this run)",
		value:  created.Key,
		note:   "created with the organization key, deleted afterwards",
	}, cleanup, nil
}

// firstProjectID returns any project in the organization, to scope the key to.
func firstProjectID(baseURL string, org credential) (string, error) {
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
		return "", fmt.Errorf("the organization has no projects to scope a key to")
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
			note: "organization API key (dashboard → organization settings → API keys)"},
		{name: "Project API key", envVar: "HOOKDECK_PROJECT_API_KEY",
			note: "OPTIONAL — one is minted with the organization key, and deleted afterwards"},
		{name: "CLI session key", envVar: "HOOKDECK_CLI_SESSION_KEY",
			note: "the api_key stored in config.toml after `hookdeck login` — NOT the CLI key you pasted"},
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

func safePrefix(key string) string {
	if i := strings.LastIndex(key, "_"); i > 0 && i < 12 {
		return key[:i+1]
	}
	if len(key) > 4 {
		return key[:4]
	}
	return "?"
}

// Ids that cannot exist, so a 404 means the credential reached the handler.
const (
	fakeProject = "tm_credentialmatrixprobe"
	fakeKey     = "apk_credentialmatrixprobe"
	fakeDomain  = "dom_credentialmatrixprobe"
)

func probes() []probe {
	return []probe{
		{label: "GET /organizations/current", method: "GET", path: "/organizations/current"},
		{label: "PUT /organizations/current", method: "PUT", path: "/organizations/current", body: `{}`},
		{label: "GET /organizations/current/api-keys", method: "GET", path: "/organizations/current/api-keys"},
		{label: "POST /organizations/current/api-keys (org key)", method: "POST",
			path: "/organizations/current/api-keys",
			body: `{"label":"credential-matrix probe","type":"organization"}`, destructive: true},
		{label: "POST /organizations/current/api-keys (project key)", method: "POST",
			path: "/organizations/current/api-keys",
			body: `{"label":"credential-matrix probe","type":"project","team_id":"` + fakeProject + `"}`},
		{label: "PUT /organizations/current/api-keys/{id}", method: "PUT",
			path: "/organizations/current/api-keys/" + fakeKey, body: `{"scopes":["*"]}`},
		{label: "POST /organizations/current/api-keys/{id}/roll", method: "POST",
			path: "/organizations/current/api-keys/" + fakeKey + "/roll", body: `{"delay_sec":60}`},
		{label: "DELETE /organizations/current/api-keys/{id}", method: "DELETE",
			path: "/organizations/current/api-keys/" + fakeKey},

		{label: "GET /projects", method: "GET", path: "/projects"},
		{label: "POST /projects", method: "POST", path: "/projects",
			body: `{"name":"credential-matrix probe","type":"event_gateway"}`, destructive: true},
		{label: "GET /projects/{id}", method: "GET", path: "/projects/" + fakeProject},
		{label: "PUT /projects/{id}", method: "PUT", path: "/projects/" + fakeProject, body: `{"name":"probe"}`},
		{label: "DELETE /projects/{id}", method: "DELETE", path: "/projects/" + fakeProject},
		{label: "GET /projects/{id}/custom_domains", method: "GET",
			path: "/projects/" + fakeProject + "/custom_domains"},
		{label: "POST /projects/{id}/custom_domains", method: "POST",
			path: "/projects/" + fakeProject + "/custom_domains", body: `{"hostname":"probe.example.test"}`},
		{label: "DELETE /projects/{id}/custom_domains/{domain_id}", method: "DELETE",
			path: "/projects/" + fakeProject + "/custom_domains/" + fakeDomain},

		// The route the Slack thread is about: exchanging a key for a CLI session.
		{label: "POST /cli-auth/ci", method: "POST", path: "/cli-auth/ci", body: `{}`},
		{label: "GET /cli-auth/validate", method: "GET", path: "/cli-auth/validate"},
	}
}

type outcome struct {
	status  int
	message string
	raw     []byte
	err     error
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

	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return outcome{err: err}
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return outcome{status: resp.StatusCode, message: apiMessage(raw), raw: raw}
}

// apiMessage pulls the readable part out of an error body.
func apiMessage(raw []byte) string {
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &payload); err == nil && payload.Message != "" {
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
func classify(o outcome) string {
	switch {
	case o.err != nil:
		return "error: " + o.err.Error()
	case o.status == 401:
		return "**401 rejected**"
	case o.status == 403:
		return "403 forbidden — " + o.message
	case o.status == 404:
		return "auth OK (404)"
	case o.status == 422:
		return "auth OK (422)"
	case o.status >= 200 && o.status < 300:
		return "**OK**"
	default:
		return fmt.Sprintf("%d — %s", o.status, o.message)
	}
}

func printMarkdown(creds []credential, results map[string]map[string]string, destructive bool) {
	labels := make([]string, 0, len(results))
	for l := range results {
		labels = append(labels, l)
	}
	sort.Strings(labels)

	head := "| Endpoint |"
	sep := "|---|"
	for _, c := range creds {
		head += " " + c.name + " |"
		sep += "---|"
	}
	fmt.Println(head)
	fmt.Println(sep)
	for _, l := range labels {
		row := "| `" + l + "` |"
		for _, c := range creds {
			row += " " + results[l][c.name] + " |"
		}
		fmt.Println(row)
	}

	fmt.Println()
	fmt.Println("`auth OK (404)` / `auth OK (422)`: the credential passed authentication and reached")
	fmt.Println("a handler that then rejected the fake id or invalid body — the request was allowed,")
	fmt.Println("nothing was created or changed.")
	if !destructive {
		fmt.Println()
		fmt.Println("Create probes were skipped. Re-run with --destructive against a throwaway")
		fmt.Println("organization to include them.")
	}
}
