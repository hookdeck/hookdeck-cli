package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/cmd/sources"
)

// TestMain keeps unit tests off the network: source auth_type inference and source auth
// validation read the OpenAPI spec, so serve the fixture cut from the real spec instead of
// downloading it.
func TestMain(m *testing.M) {
	loadSourceAuthMethods = fixtureSourceAuthMethods
	loadSourceTypes = fixtureSourceTypes
	os.Exit(m.Run())
}

func readSourceSpecFixture() ([]byte, error) {
	return os.ReadFile(filepath.Join("sources", "testdata", "openapi_source_auth.json"))
}

func fixtureSourceAuthMethods() (map[string][]sources.AuthMethod, error) {
	data, err := readSourceSpecFixture()
	if err != nil {
		return nil, err
	}
	return sources.ParseSourceAuthMethods(data)
}

func fixtureSourceTypes() (map[string]sources.SourceType, error) {
	data, err := readSourceSpecFixture()
	if err != nil {
		return nil, err
	}
	return sources.ParseSourceTypes(data)
}
