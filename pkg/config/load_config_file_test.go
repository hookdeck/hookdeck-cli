package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadConfigFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `profile = "default"

[default]
api_key = "sk_test_123456789012"
project_id = "proj_a"
project_mode = "inbound"
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	c, err := LoadConfigFromFile(path)
	require.NoError(t, err)
	require.NotNil(t, c.viper)
	require.Equal(t, "default", c.Profile.Name)
	require.Equal(t, "sk_test_123456789012", c.Profile.APIKey)
	require.Equal(t, "proj_a", c.Profile.ProjectId)
	require.Equal(t, "inbound", c.Profile.ProjectMode)
	require.Equal(t, ProjectTypeGateway, c.Profile.ProjectType)
}
