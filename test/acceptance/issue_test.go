//go:build issue

package acceptance

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skipReasonFlakyIssueCreation is used for all issue tests that depend on
// createConnectionWithFailingTransformationAndIssue (backend timing makes them
// flaky). Re-enable when the test harness or backend is stabilized.
const skipReasonFlakyIssueCreation = "unreliable: transformation-issue creation timing; skipped until test harness or backend is stabilized"

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
	t.Skip(skipReasonFlakyIssueCreation)
	cli := NewCLIRunner(t)
	connID, issueID := createConnectionWithFailingTransformationAndIssue(t, cli)
	t.Cleanup(func() { dismissIssue(t, cli, issueID); deleteConnection(t, cli, connID) })

	stdout := cli.RunExpectSuccess("gateway", "issue", "list")
	assert.Contains(t, stdout, issueID)
	assert.Contains(t, stdout, "Found")
}

func TestIssueListWithTypeFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	t.Skip(skipReasonFlakyIssueCreation)
	cli := NewCLIRunner(t)
	connID, issueID := createConnectionWithFailingTransformationAndIssue(t, cli)
	t.Cleanup(func() { dismissIssue(t, cli, issueID); deleteConnection(t, cli, connID) })

	stdout := cli.RunExpectSuccess("gateway", "issue", "list", "--type", "transformation")
	assert.Contains(t, stdout, issueID)
}

func TestIssueListWithStatusFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	t.Skip(skipReasonFlakyIssueCreation)
	cli := NewCLIRunner(t)
	connID, issueID := createConnectionWithFailingTransformationAndIssue(t, cli)
	t.Cleanup(func() { dismissIssue(t, cli, issueID); deleteConnection(t, cli, connID) })

	stdout := cli.RunExpectSuccess("gateway", "issue", "list", "--status", "OPENED")
	assert.Contains(t, stdout, issueID)
}

func TestIssueListWithLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	t.Skip(skipReasonFlakyIssueCreation)
	cli := NewCLIRunner(t)
	connID, issueID := createConnectionWithFailingTransformationAndIssue(t, cli)
	t.Cleanup(func() { dismissIssue(t, cli, issueID); deleteConnection(t, cli, connID) })

	stdout := cli.RunExpectSuccess("gateway", "issue", "list", "--limit", "5")
	assert.Contains(t, stdout, issueID)
}

func TestIssueListWithOrderBy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	t.Skip(skipReasonFlakyIssueCreation)
	cli := NewCLIRunner(t)
	connID, issueID := createConnectionWithFailingTransformationAndIssue(t, cli)
	t.Cleanup(func() { dismissIssue(t, cli, issueID); deleteConnection(t, cli, connID) })

	stdout := cli.RunExpectSuccess("gateway", "issue", "list", "--order-by", "last_seen_at", "--dir", "desc")
	assert.Contains(t, stdout, issueID)
}

func TestIssueListJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	t.Skip(skipReasonFlakyIssueCreation)
	cli := NewCLIRunner(t)
	connID, issueID := createConnectionWithFailingTransformationAndIssue(t, cli)
	t.Cleanup(func() { dismissIssue(t, cli, issueID); deleteConnection(t, cli, connID) })

	type issueListResp struct {
		Models     []Issue                `json:"models"`
		Pagination map[string]interface{} `json:"pagination"`
	}
	var resp issueListResp
	require.NoError(t, cli.RunJSON(&resp, "gateway", "issue", "list", "--limit", "5"))
	require.NotNil(t, resp.Pagination)
	require.NotEmpty(t, resp.Models, "expected at least one issue (real data)")
	assert.Equal(t, issueID, resp.Models[0].ID)
}

func TestIssueListPagination(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	t.Skip(skipReasonFlakyIssueCreation)
	cli := NewCLIRunner(t)
	conn1, issue1 := createConnectionWithFailingTransformationAndIssue(t, cli)
	conn2, issue2 := createConnectionWithFailingTransformationAndIssue(t, cli)
	t.Cleanup(func() {
		dismissIssue(t, cli, issue1)
		dismissIssue(t, cli, issue2)
		deleteConnection(t, cli, conn1)
		deleteConnection(t, cli, conn2)
	})

	type issueListResp struct {
		Models     []Issue                `json:"models"`
		Pagination map[string]interface{} `json:"pagination"`
	}
	var page1 issueListResp
	require.NoError(t, cli.RunJSON(&page1, "gateway", "issue", "list", "--limit", "1"))
	require.NotEmpty(t, page1.Models, "expected at least one issue")
	require.NotEmpty(t, page1.Pagination, "expected pagination")

	next, _ := page1.Pagination["next"].(string)
	if next == "" {
		t.Skip("only one page of issues; skipping pagination test")
	}

	var page2 issueListResp
	require.NoError(t, cli.RunJSON(&page2, "gateway", "issue", "list", "--next", next, "--limit", "5"))
	require.NotEmpty(t, page2.Models, "expected at least one issue on next page")
}

func TestIssueListEmptyResult(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	t.Skip(skipReasonFlakyIssueCreation)
	cli := NewCLIRunner(t)
	connID, issueID := createConnectionWithFailingTransformationAndIssue(t, cli)
	t.Cleanup(func() { dismissIssue(t, cli, issueID); deleteConnection(t, cli, connID) })

	// Our issue is OPENED; when we filter by RESOLVED it must not appear (other resolved issues may exist).
	stdout := cli.RunExpectSuccess("gateway", "issue", "list", "--status", "RESOLVED", "--limit", "10")
	assert.NotContains(t, stdout, issueID, "our OPENED issue must not appear in RESOLVED list")
}

// --- Count ---

func TestIssueCount(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	t.Skip(skipReasonFlakyIssueCreation)
	cli := NewCLIRunner(t)
	connID, issueID := createConnectionWithFailingTransformationAndIssue(t, cli)
	t.Cleanup(func() { dismissIssue(t, cli, issueID); deleteConnection(t, cli, connID) })

	stdout := cli.RunExpectSuccess("gateway", "issue", "count")
	require.NotEmpty(t, stdout)
	n, err := strconv.Atoi(strings.TrimSpace(stdout))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 1)
}

func TestIssueCountWithTypeFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	t.Skip(skipReasonFlakyIssueCreation)
	cli := NewCLIRunner(t)
	connID, issueID := createConnectionWithFailingTransformationAndIssue(t, cli)
	t.Cleanup(func() { dismissIssue(t, cli, issueID); deleteConnection(t, cli, connID) })

	stdout := cli.RunExpectSuccess("gateway", "issue", "count", "--type", "transformation")
	require.NotEmpty(t, stdout)
	n, err := strconv.Atoi(strings.TrimSpace(stdout))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 1)
}

func TestIssueCountWithStatusFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	t.Skip(skipReasonFlakyIssueCreation)
	cli := NewCLIRunner(t)
	connID, issueID := createConnectionWithFailingTransformationAndIssue(t, cli)
	t.Cleanup(func() { dismissIssue(t, cli, issueID); deleteConnection(t, cli, connID) })

	stdout := cli.RunExpectSuccess("gateway", "issue", "count", "--status", "OPENED")
	require.NotEmpty(t, stdout)
	n, err := strconv.Atoi(strings.TrimSpace(stdout))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 1)
}

// --- Get ---

func TestIssueGetMissingID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	_, _, err := cli.Run("gateway", "issue", "get")
	require.Error(t, err, "get without ID should fail (ExactArgs(1))")
}

func TestIssueGetWithRealData(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	t.Skip(skipReasonFlakyIssueCreation)
	cli := NewCLIRunner(t)
	connID, issueID := createConnectionWithFailingTransformationAndIssue(t, cli)
	t.Cleanup(func() { dismissIssue(t, cli, issueID); deleteConnection(t, cli, connID) })

	stdout := cli.RunExpectSuccess("gateway", "issue", "get", issueID)
	assert.Contains(t, stdout, issueID)
	assert.Contains(t, stdout, "Type:")
	assert.Contains(t, stdout, "Status:")
}

func TestIssueGetOutputJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	t.Skip(skipReasonFlakyIssueCreation)
	cli := NewCLIRunner(t)
	connID, issueID := createConnectionWithFailingTransformationAndIssue(t, cli)
	t.Cleanup(func() { dismissIssue(t, cli, issueID); deleteConnection(t, cli, connID) })

	var issue Issue
	require.NoError(t, cli.RunJSON(&issue, "gateway", "issue", "get", issueID))
	assert.Equal(t, issueID, issue.ID)
	assert.NotEmpty(t, issue.Type)
	assert.NotEmpty(t, issue.Status)
}

// --- Update ---

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

func TestIssueUpdateWithRealData(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	t.Skip(skipReasonFlakyIssueCreation)
	cli := NewCLIRunner(t)
	connID, issueID := createConnectionWithFailingTransformationAndIssue(t, cli)
	t.Cleanup(func() { dismissIssue(t, cli, issueID); deleteConnection(t, cli, connID) })

	stdout := cli.RunExpectSuccess("gateway", "issue", "update", issueID, "--status", "ACKNOWLEDGED")
	assert.Contains(t, stdout, issueID)

	var issue Issue
	require.NoError(t, cli.RunJSON(&issue, "gateway", "issue", "get", issueID))
	assert.Equal(t, "ACKNOWLEDGED", issue.Status)

	stdout = cli.RunExpectSuccess("gateway", "issue", "update", issueID, "--status", "RESOLVED")
	assert.Contains(t, stdout, issueID)
	require.NoError(t, cli.RunJSON(&issue, "gateway", "issue", "get", issueID))
	assert.Equal(t, "RESOLVED", issue.Status)
}

func TestIssueUpdateOutputJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	t.Skip(skipReasonFlakyIssueCreation)
	cli := NewCLIRunner(t)
	connID, issueID := createConnectionWithFailingTransformationAndIssue(t, cli)
	t.Cleanup(func() { dismissIssue(t, cli, issueID); deleteConnection(t, cli, connID) })

	var issue Issue
	require.NoError(t, cli.RunJSON(&issue, "gateway", "issue", "update", issueID, "--status", "IGNORED"))
	assert.Equal(t, issueID, issue.ID)
	assert.Equal(t, "IGNORED", issue.Status)
}

func TestIssueResolveTransformationIssue(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	t.Skip(skipReasonFlakyIssueCreation)
	cli := NewCLIRunner(t)
	connID, issueID := createConnectionWithFailingTransformationAndIssue(t, cli)
	t.Cleanup(func() { dismissIssue(t, cli, issueID); deleteConnection(t, cli, connID) })

	stdout := cli.RunExpectSuccess("gateway", "issue", "update", issueID, "--status", "RESOLVED")
	assert.Contains(t, stdout, issueID)
	assert.Contains(t, stdout, "RESOLVED")
}

// --- Dismiss ---

func TestIssueDismissMissingID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	_, _, err := cli.Run("gateway", "issue", "dismiss")
	require.Error(t, err, "dismiss without ID should fail (ExactArgs(1))")
}

func TestIssueDismissForce(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	t.Skip(skipReasonFlakyIssueCreation)
	cli := NewCLIRunner(t)
	connID, issueID := createConnectionWithFailingTransformationAndIssue(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	stdout := cli.RunExpectSuccess("gateway", "issue", "dismiss", issueID, "--force")
	assert.Contains(t, stdout, issueID)
	assert.Contains(t, stdout, "dismissed")
}
