package acceptance

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDestinationList(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "destination", "list")
	assert.NotEmpty(t, stdout)
}

func TestDestinationCreateAndDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	destID := createTestDestination(t, cli)
	t.Cleanup(func() { deleteDestination(t, cli, destID) })

	stdout := cli.RunExpectSuccess("gateway", "destination", "get", destID)
	assert.Contains(t, stdout, destID)
	assert.Contains(t, stdout, "HTTP")
}

func TestDestinationGetByName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()
	name := "test-dst-get-" + timestamp

	var dst Destination
	err := cli.RunJSON(&dst, "gateway", "destination", "create", "--name", name, "--type", "HTTP", "--url", "https://example.com/webhooks")
	require.NoError(t, err)
	t.Cleanup(func() { deleteDestination(t, cli, dst.ID) })

	stdout := cli.RunExpectSuccess("gateway", "destination", "get", name)
	assert.Contains(t, stdout, dst.ID)
	assert.Contains(t, stdout, name)
}

func TestDestinationCreateWithDescription(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()
	name := "test-dst-desc-" + timestamp
	desc := "Test destination description"

	var dst Destination
	err := cli.RunJSON(&dst, "gateway", "destination", "create", "--name", name, "--type", "HTTP", "--url", "https://example.com/webhooks", "--description", desc)
	require.NoError(t, err)
	t.Cleanup(func() { deleteDestination(t, cli, dst.ID) })

	stdout := cli.RunExpectSuccess("gateway", "destination", "get", dst.ID)
	assert.Contains(t, stdout, desc)
}

func TestDestinationUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	destID := createTestDestination(t, cli)
	t.Cleanup(func() { deleteDestination(t, cli, destID) })

	newName := "test-dst-updated-" + generateTimestamp()
	cli.RunExpectSuccess("gateway", "destination", "update", destID, "--name", newName)

	stdout := cli.RunExpectSuccess("gateway", "destination", "get", destID)
	assert.Contains(t, stdout, newName)
}

func TestDestinationUpsertCreate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	name := "test-dst-upsert-create-" + generateTimestamp()

	var dst Destination
	err := cli.RunJSON(&dst, "gateway", "destination", "upsert", name, "--type", "HTTP", "--url", "https://example.com/upsert")
	require.NoError(t, err)
	require.NotEmpty(t, dst.ID)
	assert.Equal(t, name, dst.Name)
	t.Cleanup(func() { deleteDestination(t, cli, dst.ID) })
}

func TestDestinationUpsertUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	name := "test-dst-upsert-upd-" + generateTimestamp()

	var dst Destination
	err := cli.RunJSON(&dst, "gateway", "destination", "upsert", name, "--type", "HTTP", "--url", "https://example.com/webhooks")
	require.NoError(t, err)
	t.Cleanup(func() { deleteDestination(t, cli, dst.ID) })

	newDesc := "Updated via upsert"
	err = cli.RunJSON(&dst, "gateway", "destination", "upsert", name, "--description", newDesc)
	require.NoError(t, err)

	stdout := cli.RunExpectSuccess("gateway", "destination", "get", dst.ID)
	assert.Contains(t, stdout, newDesc)
}

func TestDestinationEnableDisable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	destID := createTestDestination(t, cli)
	t.Cleanup(func() { deleteDestination(t, cli, destID) })

	cli.RunExpectSuccess("gateway", "destination", "disable", destID)
	stdout := cli.RunExpectSuccess("gateway", "destination", "get", destID)
	assert.Contains(t, stdout, "disabled")

	cli.RunExpectSuccess("gateway", "destination", "enable", destID)
	stdout = cli.RunExpectSuccess("gateway", "destination", "get", destID)
	assert.Contains(t, stdout, "active")
}

func TestDestinationCount(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "destination", "count")
	stdout = strings.TrimSpace(stdout)
	assert.NotEmpty(t, stdout)
	assert.Regexp(t, `^\d+$`, stdout)
}

func TestDestinationListFilterByName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	destID := createTestDestination(t, cli)
	t.Cleanup(func() { deleteDestination(t, cli, destID) })

	var dst Destination
	err := cli.RunJSON(&dst, "gateway", "destination", "get", destID)
	require.NoError(t, err)

	stdout := cli.RunExpectSuccess("gateway", "destination", "list", "--name", dst.Name)
	assert.Contains(t, stdout, dst.ID)
	assert.Contains(t, stdout, dst.Name)
}

func TestDestinationListFilterByType(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "destination", "list", "--type", "HTTP", "--limit", "5")
	assert.NotContains(t, stdout, "failed")
}

func TestDestinationDeleteForce(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	destID := createTestDestination(t, cli)

	cli.RunExpectSuccess("gateway", "destination", "delete", destID, "--force")

	_, _, err := cli.Run("gateway", "destination", "get", destID)
	require.Error(t, err)
}

func TestDestinationUpsertDryRun(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	name := "test-dst-dryrun-" + generateTimestamp()
	stdout := cli.RunExpectSuccess("gateway", "destination", "upsert", name, "--type", "HTTP", "--url", "https://example.com", "--dry-run")
	assert.Contains(t, stdout, "Dry Run")
	assert.Contains(t, stdout, "CREATE")
}

func TestDestinationGetOutputJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	destID := createTestDestination(t, cli)
	t.Cleanup(func() { deleteDestination(t, cli, destID) })

	var dst Destination
	err := cli.RunJSON(&dst, "gateway", "destination", "get", destID, "--output", "json")
	require.NoError(t, err)
	assert.Equal(t, destID, dst.ID)
	assert.Equal(t, "HTTP", dst.Type)
}

func TestDestinationCreateWithBearerToken(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()
	name := "test-dst-bearer-" + timestamp

	var dst Destination
	err := cli.RunJSON(&dst, "gateway", "destination", "create",
		"--name", name,
		"--type", "HTTP",
		"--url", "https://api.example.com/webhooks",
		"--auth-method", "bearer",
		"--bearer-token", "test-token-123",
	)
	require.NoError(t, err)
	require.NotEmpty(t, dst.ID)
	t.Cleanup(func() { deleteDestination(t, cli, dst.ID) })

	stdout := cli.RunExpectSuccess("gateway", "destination", "get", dst.ID)
	assert.Contains(t, stdout, name)
	assert.Contains(t, stdout, "HTTP")
}

// TestDestinationCreateWithAuthThenGetWithIncludeAuth creates a destination with auth (bearer token),
// then gets it with --include-auth. Verifies that config.auth is returned and the token
// set at creation is present in the get output (auth round-trip).
func TestDestinationCreateWithAuthThenGetWithIncludeAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()
	name := "test-dst-auth-include-" + timestamp
	bearerToken := "test-bearer-roundtrip-secret"

	var dst Destination
	err := cli.RunJSON(&dst, "gateway", "destination", "create",
		"--name", name,
		"--type", "HTTP",
		"--url", "https://api.example.com/webhooks",
		"--auth-method", "bearer",
		"--bearer-token", bearerToken,
	)
	require.NoError(t, err)
	require.NotEmpty(t, dst.ID)
	t.Cleanup(func() { deleteDestination(t, cli, dst.ID) })

	// Get with --include-auth: auth content must be included (include=config.auth).
	stdout, _, err := cli.Run("gateway", "destination", "get", dst.ID, "--output", "json", "--include-auth")
	require.NoError(t, err)

	if !strings.Contains(stdout, bearerToken) {
		t.Logf("Full API response body: %s", stdout)
	}
	require.Contains(t, stdout, bearerToken,
		"get with --include-auth must return auth content; bearer token set at creation should be present in output")

	// When include-auth is used, config must include auth_type (e.g. BEARER_TOKEN for bearer auth)
	var getResp map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(stdout), &getResp), "parse get response as JSON")
	config, ok := getResp["config"].(map[string]interface{})
	require.True(t, ok, "get with --include-auth must return config")
	authType, hasType := config["auth_type"].(string)
	require.True(t, hasType && authType != "", "get with --include-auth must return config.auth_type")
	assert.Equal(t, "BEARER_TOKEN", authType, "destination with bearer auth should have config.auth_type BEARER_TOKEN")
	t.Logf("Destination config.auth_type: %s", authType)
}

// TestDestinationCreateWithBasicAuthThenGetWithIncludeAuth creates a destination with basic auth,
// then gets it with --include-auth. Verifies config.auth_type is BASIC_AUTH.
func TestDestinationCreateWithBasicAuthThenGetWithIncludeAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()
	name := "test-dst-basic-include-" + timestamp
	username := "basic_user"
	password := "basic_pass_secret"

	var dst Destination
	err := cli.RunJSON(&dst, "gateway", "destination", "create",
		"--name", name,
		"--type", "HTTP",
		"--url", "https://api.example.com/webhooks",
		"--auth-method", "basic",
		"--basic-auth-user", username,
		"--basic-auth-pass", password,
	)
	require.NoError(t, err)
	require.NotEmpty(t, dst.ID)
	t.Cleanup(func() { deleteDestination(t, cli, dst.ID) })

	stdout, _, err := cli.Run("gateway", "destination", "get", dst.ID, "--output", "json", "--include-auth")
	require.NoError(t, err)

	require.Contains(t, stdout, username, "get with --include-auth must return auth content (username)")

	var getResp map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(stdout), &getResp), "parse get response as JSON")
	config, ok := getResp["config"].(map[string]interface{})
	require.True(t, ok, "get with --include-auth must return config")
	authType, hasType := config["auth_type"].(string)
	require.True(t, hasType && authType != "", "get with --include-auth must return config.auth_type")
	assert.Equal(t, "BASIC_AUTH", authType, "destination with basic auth should have config.auth_type BASIC_AUTH")
	t.Logf("Destination config.auth_type: %s", authType)
}

// TestDestinationCreateWithAPIKeyThenGetWithIncludeAuth creates a destination with API key auth,
// then gets it with --include-auth. Verifies config.auth_type is API_KEY.
func TestDestinationCreateWithAPIKeyThenGetWithIncludeAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()
	name := "test-dst-apikey-include-" + timestamp
	apiKey := "test_dst_apikey_secret"

	var dst Destination
	err := cli.RunJSON(&dst, "gateway", "destination", "create",
		"--name", name,
		"--type", "HTTP",
		"--url", "https://api.example.com/webhooks",
		"--auth-method", "api_key",
		"--api-key", apiKey,
		"--api-key-header", "X-API-Key",
	)
	require.NoError(t, err)
	require.NotEmpty(t, dst.ID)
	t.Cleanup(func() { deleteDestination(t, cli, dst.ID) })

	stdout, _, err := cli.Run("gateway", "destination", "get", dst.ID, "--output", "json", "--include-auth")
	require.NoError(t, err)

	require.Contains(t, stdout, apiKey, "get with --include-auth must return auth content (api_key)")

	var getResp map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(stdout), &getResp), "parse get response as JSON")
	config, ok := getResp["config"].(map[string]interface{})
	require.True(t, ok, "get with --include-auth must return config")
	authType, hasType := config["auth_type"].(string)
	require.True(t, hasType && authType != "", "get with --include-auth must return config.auth_type")
	assert.Equal(t, "API_KEY", authType, "destination with API key auth should have config.auth_type API_KEY")
	t.Logf("Destination config.auth_type: %s", authType)
}

func TestDestinationCreateCLI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()
	name := "test-dst-cli-" + timestamp

	var dst Destination
	err := cli.RunJSON(&dst, "gateway", "destination", "create", "--name", name, "--type", "CLI", "--cli-path", "/webhooks")
	require.NoError(t, err)
	require.NotEmpty(t, dst.ID)
	t.Cleanup(func() { deleteDestination(t, cli, dst.ID) })

	stdout := cli.RunExpectSuccess("gateway", "destination", "get", dst.ID)
	assert.Contains(t, stdout, name)
	assert.Contains(t, stdout, "CLI")
}

func TestGatewayDestinationsAliasWorks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "destinations", "list")
	assert.NotContains(t, stdout, "unknown command")
}
