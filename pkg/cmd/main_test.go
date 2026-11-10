package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/cmd/sources"
)

// TestMain keeps unit tests off the network: source auth_type inference reads the OpenAPI
// spec, so serve the fixture cut from the real spec instead of downloading it.
func TestMain(m *testing.M) {
	loadSourceAuthMethods = fixtureSourceAuthMethods
	os.Exit(m.Run())
}

func fixtureSourceAuthMethods() (map[string][]sources.AuthMethod, error) {
	data, err := os.ReadFile(filepath.Join("sources", "testdata", "openapi_source_auth.json"))
	if err != nil {
		return nil, err
	}
	return sources.ParseSourceAuthMethods(data)
}
