package acceptance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Help ---

func TestIssueHelp(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "issue", "--help")
	assert.Contains(t, stdout, "list")
	assert.Contains(t, stdout, "get")
	assert.Contains(t, stdout, "update")
	assert.Contains(t, stdout, "dismiss")
	assert.Contains(t, stdout, "count")
}

// TestIssueHelpAliases verifies that "issues" (plural) is accepted as an alias.
func TestIssueHelpAliases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "issues", "--help")
	assert.Contains(t, stdout, "list")
}

// --- List ---

func TestIssueList(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	// List should succeed even with zero issues.
	stdout := cli.RunExpectSuccess("gateway", "issue", "list")
	assert.NotEmpty(t, stdout)
}

func TestIssueListWithTypeFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "issue", "list", "--type", "delivery")
	assert.NotEmpty(t, stdout)
}

func TestIssueListWithStatusFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "issue", "list", "--status", "OPENED")
	assert.NotEmpty(t, stdout)
}

func TestIssueListWithLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "issue", "list", "--limit", "5")
	assert.NotEmpty(t, stdout)
}

func TestIssueListWithOrderBy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "issue", "list", "--order-by", "last_seen_at", "--dir", "desc")
	assert.NotEmpty(t, stdout)
}

// --- List JSON ---

func TestIssueListJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)

	type IssueListResponse struct {
		Models     []map[string]interface{} `json:"models"`
		Pagination map[string]interface{}   `json:"pagination"`
	}
	var resp IssueListResponse
	require.NoError(t, cli.RunJSON(&resp, "gateway", "issue", "list", "--limit", "5"))
	assert.NotNil(t, resp.Pagination)
	// Models may be empty if no issues exist; just verify structure is valid.
}

// --- Count ---

func TestIssueCount(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "issue", "count")
	assert.NotEmpty(t, stdout) // Prints a number (possibly "0")
}

func TestIssueCountWithTypeFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "issue", "count", "--type", "delivery")
	assert.NotEmpty(t, stdout)
}

func TestIssueCountWithStatusFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "issue", "count", "--status", "OPENED")
	assert.NotEmpty(t, stdout)
}

// --- Get (validation) ---

func TestIssueGetMissingID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	_, _, err := cli.Run("gateway", "issue", "get")
	require.Error(t, err, "get without ID should fail (ExactArgs(1))")
}

// --- Update (validation) ---

func TestIssueUpdateMissingID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	_, _, err := cli.Run("gateway", "issue", "update", "--status", "ACKNOWLEDGED")
	require.Error(t, err, "update without ID should fail (ExactArgs(1))")
}

func TestIssueUpdateMissingStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	_, _, err := cli.Run("gateway", "issue", "update", "iss_placeholder")
	require.Error(t, err, "update without --status should fail (required flag)")
}

func TestIssueUpdateInvalidStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	_, _, err := cli.Run("gateway", "issue", "update", "iss_placeholder", "--status", "INVALID")
	require.Error(t, err, "update with invalid status should fail")
}

// --- Dismiss (validation) ---

func TestIssueDismissMissingID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	_, _, err := cli.Run("gateway", "issue", "dismiss")
	require.Error(t, err, "dismiss without ID should fail (ExactArgs(1))")
}

// --- Get/Update/Dismiss with a real issue (if any exist) ---

// TestIssueGetUpdateWorkflow lists issues, and if any exist, tests get and update on a real one.
func TestIssueGetUpdateWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)

	type Issue struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Type   string `json:"type"`
	}
	type IssueListResponse struct {
		Models     []Issue                `json:"models"`
		Pagination map[string]interface{} `json:"pagination"`
	}
	var resp IssueListResponse
	require.NoError(t, cli.RunJSON(&resp, "gateway", "issue", "list", "--limit", "1"))

	if len(resp.Models) == 0 {
		t.Skip("No issues exist in the project; skipping get/update workflow test")
	}

	issueID := resp.Models[0].ID

	// Get
	stdout := cli.RunExpectSuccess("gateway", "issue", "get", issueID)
	assert.Contains(t, stdout, issueID)

	// Get JSON
	var issue Issue
	require.NoError(t, cli.RunJSON(&issue, "gateway", "issue", "get", issueID))
	assert.Equal(t, issueID, issue.ID)
	assert.NotEmpty(t, issue.Type)
	assert.NotEmpty(t, issue.Status)

	// Update to ACKNOWLEDGED (safe, non-destructive)
	stdout = cli.RunExpectSuccess("gateway", "issue", "update", issueID, "--status", "ACKNOWLEDGED")
	assert.Contains(t, stdout, issueID)

	// Verify status changed
	var updated Issue
	require.NoError(t, cli.RunJSON(&updated, "gateway", "issue", "get", issueID))
	assert.Equal(t, "ACKNOWLEDGED", updated.Status)
}
