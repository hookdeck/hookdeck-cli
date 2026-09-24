package mcpcore

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Models routinely send a bare or comma-separated string where a tool declares
// an array. mcpcore.StringList accepts both; reading a caller-supplied list any
// other way drops the string without an error. That is how gateway_metrics_read
// came to tell callers "measures is required" for a measures they had passed
// (#440) -- and how a merge later put the bug back unnoticed.
//
// stringSlice is unexported, so tool code cannot call the array-only reader.
// This covers the remaining way round it: asserting .([]interface{}) directly
// on a tool argument. Each existing use is listed with the reason it is right;
// a new one fails here until someone decides it belongs on that list.
var arrayAssertionAllowed = map[string]string{
	// rules is an array of rule OBJECTS, not strings, so StringList does not
	// apply -- and ruleList errors on a non-array rather than dropping it.
	"gateway/mcp/tool_connections.go": "rules: array of objects, and a non-array is an error",
}

var arrayAssertion = regexp.MustCompile(`\.\(\s*\[\](interface\{\}|any)\s*\)`)

func TestToolArgumentsAreNotReadAsArraysDirectly(t *testing.T) {
	for _, dir := range []string{"../gateway/mcp", "../outpost/mcp"} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		if len(files) == 0 {
			t.Fatalf("no Go files found in %s; the guard would pass vacuously", dir)
		}
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if !arrayAssertion.Match(src) {
				continue
			}
			key := strings.TrimPrefix(filepath.ToSlash(f), "../")
			if _, ok := arrayAssertionAllowed[key]; ok {
				continue
			}
			t.Errorf("%s asserts .([]interface{}) on tool input. Read a list argument with "+
				"mcpcore.StringList, which also accepts the string forms models send; "+
				"if this really is not a list of strings, add the file to arrayAssertionAllowed with the reason.", key)
		}
	}
}

// An allow-list entry for a file that no longer has the assertion is stale, and
// a stale entry would silently cover the next use added to that file.
func TestArrayAssertionAllowListIsCurrent(t *testing.T) {
	for key := range arrayAssertionAllowed {
		src, err := os.ReadFile(filepath.Join("..", key))
		if err != nil {
			t.Errorf("allow-listed file %s: %v", key, err)
			continue
		}
		if !arrayAssertion.Match(src) {
			t.Errorf("%s is allow-listed but no longer asserts .([]interface{}); remove the entry", key)
		}
	}
}
