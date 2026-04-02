package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClearActiveProfileCredentials_MemoryOnly(t *testing.T) {
	c := &Config{}
	c.Profile.APIKey = "sk_test_123456789012"
	c.Profile.ProjectId = "proj_1"
	c.Profile.ProjectMode = "inbound"
	c.Profile.ProjectType = ProjectTypeGateway
	require.NoError(t, c.ClearActiveProfileCredentials())
	assert.Empty(t, c.Profile.APIKey)
	assert.Empty(t, c.Profile.ProjectId)
}
