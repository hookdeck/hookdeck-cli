package acceptance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransformationList(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "transformation", "list")
	assert.NotEmpty(t, stdout)
}

func TestTransformationListWithName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	trnID := createTestTransformation(t, cli)
	t.Cleanup(func() { deleteTransformation(t, cli, trnID) })

	var trn Transformation
	require.NoError(t, cli.RunJSON(&trn, "gateway", "transformation", "get", trnID))

	stdout := cli.RunExpectSuccess("gateway", "transformation", "list", "--name", trn.Name)
	assert.Contains(t, stdout, trn.ID)
	assert.Contains(t, stdout, trn.Name)
}

func TestTransformationListWithOrderByDir(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "transformation", "list", "--order-by", "created_at", "--dir", "desc", "--limit", "5")
	assert.NotEmpty(t, stdout)
}

func TestTransformationCreateAndDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	trnID := createTestTransformation(t, cli)
	t.Cleanup(func() { deleteTransformation(t, cli, trnID) })

	stdout := cli.RunExpectSuccess("gateway", "transformation", "get", trnID)
	assert.Contains(t, stdout, trnID)
}

func TestTransformationGetByName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()
	name := "test-trn-get-" + timestamp
	code := `addHandler("transform", (request, context) => { return request; });`

	var trn Transformation
	err := cli.RunJSON(&trn, "gateway", "transformation", "create", "--name", name, "--code", code)
	require.NoError(t, err)
	t.Cleanup(func() { deleteTransformation(t, cli, trn.ID) })

	stdout := cli.RunExpectSuccess("gateway", "transformation", "get", name)
	assert.Contains(t, stdout, trn.ID)
	assert.Contains(t, stdout, name)
}

func TestTransformationCreateWithEnv(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()
	name := "test-trn-env-" + timestamp
	code := `addHandler("transform", (request, context) => { return request; });`

	var trn Transformation
	err := cli.RunJSON(&trn, "gateway", "transformation", "create", "--name", name, "--code", code, "--env", "FOO=bar,BAZ=qux")
	require.NoError(t, err)
	t.Cleanup(func() { deleteTransformation(t, cli, trn.ID) })

	stdout := cli.RunExpectSuccess("gateway", "transformation", "get", trn.ID)
	assert.Contains(t, stdout, trn.ID)
}

func TestTransformationCreateWithCodeFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	dir := t.TempDir()
	codePath := filepath.Join(dir, "code.js")
	require.NoError(t, os.WriteFile(codePath, []byte(`addHandler("transform", (request, context) => { return request; });`), 0644))

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()
	name := "test-trn-codefile-" + timestamp

	var trn Transformation
	err := cli.RunJSON(&trn, "gateway", "transformation", "create", "--name", name, "--code-file", codePath)
	require.NoError(t, err)
	t.Cleanup(func() { deleteTransformation(t, cli, trn.ID) })

	assert.Equal(t, name, trn.Name)
	stdout := cli.RunExpectSuccess("gateway", "transformation", "get", trn.ID)
	assert.Contains(t, stdout, trn.ID)
}

func TestTransformationUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	trnID := createTestTransformation(t, cli)
	t.Cleanup(func() { deleteTransformation(t, cli, trnID) })

	newName := "test-trn-updated-" + generateTimestamp()
	cli.RunExpectSuccess("gateway", "transformation", "update", trnID, "--name", newName)

	stdout := cli.RunExpectSuccess("gateway", "transformation", "get", trnID)
	assert.Contains(t, stdout, newName)
}

func TestTransformationUpdateWithCode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	trnID := createTestTransformation(t, cli)
	t.Cleanup(func() { deleteTransformation(t, cli, trnID) })

	newCode := `addHandler("transform", (request, context) => { request.headers["x-patched"] = "true"; return request; });`
	cli.RunExpectSuccess("gateway", "transformation", "update", trnID, "--code", newCode)

	stdout := cli.RunExpectSuccess("gateway", "transformation", "get", trnID)
	assert.Contains(t, stdout, "patched")
}

func TestTransformationUpdateWithEnv(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	trnID := createTestTransformation(t, cli)
	t.Cleanup(func() { deleteTransformation(t, cli, trnID) })

	cli.RunExpectSuccess("gateway", "transformation", "update", trnID, "--env", "K=vvv")
	stdout := cli.RunExpectSuccess("gateway", "transformation", "get", trnID)
	assert.Contains(t, stdout, trnID)
}

func TestTransformationDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	trnID := createTestTransformation(t, cli)

	cli.RunExpectSuccess("gateway", "transformation", "delete", trnID, "--force")

	_, _, err := cli.Run("gateway", "transformation", "get", trnID)
	require.Error(t, err)
}

func TestTransformationUpsertCreate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	name := "test-trn-upsert-create-" + generateTimestamp()
	code := `addHandler("transform", (request, context) => { return request; });`

	var trn Transformation
	err := cli.RunJSON(&trn, "gateway", "transformation", "upsert", name, "--code", code)
	require.NoError(t, err)
	require.NotEmpty(t, trn.ID)
	assert.Equal(t, name, trn.Name)
	t.Cleanup(func() { deleteTransformation(t, cli, trn.ID) })
}

func TestTransformationUpsertUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	name := "test-trn-upsert-upd-" + generateTimestamp()
	code := `addHandler("transform", (request, context) => { return request; });`

	var trn Transformation
	err := cli.RunJSON(&trn, "gateway", "transformation", "upsert", name, "--code", code)
	require.NoError(t, err)
	t.Cleanup(func() { deleteTransformation(t, cli, trn.ID) })

	newCode := `addHandler("transform", (request, context) => { request.headers["x-updated"] = "true"; return request; });`
	err = cli.RunJSON(&trn, "gateway", "transformation", "upsert", name, "--code", newCode)
	require.NoError(t, err)

	stdout := cli.RunExpectSuccess("gateway", "transformation", "get", trn.ID)
	assert.Contains(t, stdout, "updated")
}

func TestTransformationUpsertDryRun(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	name := "test-trn-dryrun-" + generateTimestamp()
	code := `addHandler("transform", (request, context) => { return request; });`

	stdout := cli.RunExpectSuccess("gateway", "transformation", "upsert", name, "--code", code, "--dry-run")
	assert.Contains(t, stdout, "Dry Run")
	assert.Contains(t, stdout, "CREATE")

	// No resource should exist
	_, _, err := cli.Run("gateway", "transformation", "get", name)
	require.Error(t, err)
}

func TestTransformationCount(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "transformation", "count")
	assert.NotEmpty(t, stdout)
}

func TestTransformationCountWithName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	trnID := createTestTransformation(t, cli)
	t.Cleanup(func() { deleteTransformation(t, cli, trnID) })

	var trn Transformation
	require.NoError(t, cli.RunJSON(&trn, "gateway", "transformation", "get", trnID))

	stdout := cli.RunExpectSuccess("gateway", "transformation", "count", "--name", trn.Name)
	assert.NotEmpty(t, stdout)
}

func TestTransformationCountOutputJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "transformation", "count", "--output", "json")
	assert.True(t, len(stdout) > 0 && (stdout[0] == '{' || (stdout[0] >= '0' && stdout[0] <= '9')))
}

func TestTransformationRun(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	code := `addHandler("transform", (request, context) => { return request; });`
	request := `{"headers":{}}`

	stdout, stderr, err := cli.Run("gateway", "transformation", "run", "--code", code, "--request", request)
	require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)
	assert.Contains(t, stdout, "Transformation run completed")
}

func TestTransformationRunModifiesRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	code := `addHandler("transform", (request, context) => { request.headers["x-transformed"] = "true"; return request; });`
	request := `{"headers":{},"body":{"foo":"bar"}}`

	stdout, stderr, err := cli.Run("gateway", "transformation", "run", "--code", code, "--request", request)
	require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)
	assert.Contains(t, stdout, "Transformation run completed")
	assert.Contains(t, stdout, "x-transformed", "transformation output should include the modified header")
}

func TestTransformationRunWithTransformationID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	trnID := createTestTransformation(t, cli)
	t.Cleanup(func() { deleteTransformation(t, cli, trnID) })

	request := `{"headers":{}}`
	stdout, stderr, err := cli.Run("gateway", "transformation", "run", "--id", trnID, "--request", request)
	require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)
	assert.Contains(t, stdout, "Transformation run completed")
}

func TestTransformationRunWithEnv(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	code := `addHandler("transform", (request, context) => { return request; });`
	request := `{"headers":{}}`

	stdout, stderr, err := cli.Run("gateway", "transformation", "run", "--code", code, "--request", request, "--env", "X=y")
	require.NoError(t, err, "stdout: %s, stderr: %s", stdout, stderr)
	assert.Contains(t, stdout, "Transformation run completed")
}

func TestTransformationExecutionsList(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	trnID := createTestTransformation(t, cli)
	t.Cleanup(func() { deleteTransformation(t, cli, trnID) })

	stdout := cli.RunExpectSuccess("gateway", "transformation", "executions", "list", trnID)
	assert.NotEmpty(t, stdout)
}

func TestTransformationExecutionsListWithLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	trnID := createTestTransformation(t, cli)
	t.Cleanup(func() { deleteTransformation(t, cli, trnID) })

	stdout := cli.RunExpectSuccess("gateway", "transformation", "executions", "list", trnID, "--limit", "2", "--order-by", "created_at", "--dir", "desc")
	assert.NotEmpty(t, stdout)
}

func TestTransformationExecutionsGetNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	trnID := createTestTransformation(t, cli)
	t.Cleanup(func() { deleteTransformation(t, cli, trnID) })

	_, _, err := cli.Run("gateway", "transformation", "executions", "get", trnID, "exec_nonexistent")
	require.Error(t, err)
}

func TestTransformationListOutputJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	type TransformationListResponse struct {
		Models     []interface{}          `json:"models"`
		Pagination map[string]interface{} `json:"pagination"`
	}
	var resp TransformationListResponse
	require.NoError(t, cli.RunJSON(&resp, "gateway", "transformation", "list"))
	assert.NotNil(t, resp.Models)
	assert.NotNil(t, resp.Pagination)
}
