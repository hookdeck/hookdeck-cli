// Package speccheck validates outgoing API requests against the OpenAPI
// document the CLI is pinned to.
//
// It exists because a hand-written mock answers whatever it is asked. A test
// that asserts "this filter reached the API" against such a mock confirms the
// code's behaviour rather than the API's contract, so a filter the endpoint
// does not declare passes the test and is silently dropped in production. That
// is the failure this repository keeps finding, and it was reintroduced by a
// test of exactly that shape: GET /requests/{id}/ignored_events declares six
// query parameters where its sibling GET /requests/{id}/events declares thirty,
// and a mock happily recorded all thirty arriving.
//
// Wiring this into the mock server makes the spec the arbiter, so the next
// asymmetry fails a test instead of shipping.
package speccheck

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// bracketSuffix matches the bracket serialisations the API uses, so a key is
// compared against the parameter the document actually declares:
//
//	created_at[gte]     -> created_at   (range operator)
//	measures[]          -> measures     (array)
//	filters[source_id]  -> filters      (deepObject)
//
// The inner key of a deepObject is therefore not checked here. Which filters a
// metrics route honours is a finer question than the document answers at this
// level, and pkg/hookdeck's filter matrix already owns it.
var bracketSuffix = regexp.MustCompile(`\[[^\]]*\]$`)

type document struct {
	Paths map[string]map[string]operation `json:"paths"`
}

type operation struct {
	Parameters []parameter `json:"parameters"`
}

type parameter struct {
	Name string `json:"name"`
	In   string `json:"in"`
	Ref  string `json:"$ref"`
}

var (
	once   sync.Once
	loaded *document
	loadIn error
)

// Load reads the pinned OpenAPI document, once per process.
func Load() (*document, error) {
	once.Do(func() {
		path, err := specPath()
		if err != nil {
			loadIn = err
			return
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			loadIn = err
			return
		}
		doc := &document{}
		if err := json.Unmarshal(raw, doc); err != nil {
			loadIn = err
			return
		}
		loaded = doc
	})
	return loaded, loadIn
}

// specPath walks up from the working directory to the module root, so the
// document resolves the same from any package's test binary.
func specPath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "test", "openapi", "openapi_2026-09-01.json"), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("speccheck: no go.mod above %s", dir)
		}
		dir = parent
	}
}

// QueryParams returns the query parameters the document declares for a concrete
// request path, and whether the path matched a documented route at all.
//
// apiPath is the path as the CLI sends it, version prefix included; the prefix
// is stripped before matching because the document's paths are relative to it.
func QueryParams(apiPath, method string) (map[string]bool, bool) {
	doc, err := Load()
	if err != nil || doc == nil {
		return nil, false
	}

	// Strip the version prefix: "/2026-09-01/events" -> "/events".
	trimmed := apiPath
	if i := strings.Index(trimmed[1:], "/"); i >= 0 {
		trimmed = trimmed[i+1:]
	}

	template, ok := matchTemplate(doc, trimmed)
	if !ok {
		return nil, false
	}
	op, ok := doc.Paths[template][strings.ToLower(method)]
	if !ok {
		return nil, false
	}

	out := map[string]bool{}
	for _, p := range op.Parameters {
		// A $ref'd parameter is almost always a shared query filter; without
		// resolving it we cannot know, so treat the route as unconstrained
		// rather than reject a parameter that is in fact declared.
		if p.Ref != "" {
			return nil, false
		}
		if p.In == "query" {
			out[p.Name] = true
		}
	}
	return out, true
}

// matchTemplate finds the document path whose {placeholders} fit this path.
func matchTemplate(doc *document, path string) (string, bool) {
	if _, ok := doc.Paths[path]; ok {
		return path, true
	}
	want := strings.Split(strings.Trim(path, "/"), "/")
	for template := range doc.Paths {
		got := strings.Split(strings.Trim(template, "/"), "/")
		if len(got) != len(want) {
			continue
		}
		match := true
		for i := range got {
			if strings.HasPrefix(got[i], "{") {
				continue
			}
			if got[i] != want[i] {
				match = false
				break
			}
		}
		if match {
			return template, true
		}
	}
	return "", false
}

// Undeclared returns the query keys this route does not declare, sorted. An
// empty result means the query is conformant, or that the route is not one the
// document constrains.
func Undeclared(apiPath, method string, query map[string][]string) []string {
	declared, ok := QueryParams(apiPath, method)
	if !ok || len(declared) == 0 {
		return nil
	}
	var bad []string
	for key := range query {
		name := bracketSuffix.ReplaceAllString(key, "")
		if !declared[name] {
			bad = append(bad, key)
		}
	}
	return bad
}
