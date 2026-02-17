package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

// newTransformationExecutionsCmd returns the "executions" parent command with list and get subcommands.
func newTransformationExecutionsCmd() *cobra.Command {
	exec := &cobra.Command{
		Use:   "executions",
		Short: "List or get transformation executions",
		Long:  `List executions for a transformation, or get a single execution by ID.`,
	}
	exec.AddCommand(newTransformationExecutionsListCmd().cmd)
	exec.AddCommand(newTransformationExecutionsGetCmd().cmd)
	return exec
}

type transformationExecutionsListCmd struct {
	cmd        *cobra.Command
	trnID      string
	logLevel   string
	webhookID  string
	issueID    string
	createdAt  string
	orderBy    string
	dir        string
	limit      int
	next       string
	prev       string
	output     string
}

func newTransformationExecutionsListCmd() *transformationExecutionsListCmd {
	tc := &transformationExecutionsListCmd{}

	tc.cmd = &cobra.Command{
		Use:   "list <transformation-id-or-name>",
		Args:  validators.ExactArgs(1),
		Short: "List transformation executions",
		Long:  `List executions for a transformation.`,
		RunE:  tc.run,
	}

	tc.cmd.Flags().StringVar(&tc.logLevel, "log-level", "", "Filter by log level (debug, info, warn, error, fatal)")
	tc.cmd.Flags().StringVar(&tc.webhookID, "webhook-id", "", "Filter by connection (webhook) ID")
	tc.cmd.Flags().StringVar(&tc.issueID, "issue-id", "", "Filter by issue ID")
	tc.cmd.Flags().StringVar(&tc.createdAt, "created-at", "", "Filter by created_at (ISO date or operator)")
	tc.cmd.Flags().StringVar(&tc.orderBy, "order-by", "", "Sort key (created_at)")
	tc.cmd.Flags().StringVar(&tc.dir, "dir", "", "Sort direction (asc, desc)")
	tc.cmd.Flags().IntVar(&tc.limit, "limit", 100, "Limit number of results")
	tc.cmd.Flags().StringVar(&tc.next, "next", "", "Pagination cursor for next page")
	tc.cmd.Flags().StringVar(&tc.prev, "prev", "", "Pagination cursor for previous page")
	tc.cmd.Flags().StringVar(&tc.output, "output", "", "Output format (json)")

	return tc
}

func (tc *transformationExecutionsListCmd) run(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	tc.trnID = args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	trnID, err := resolveTransformationID(ctx, client, tc.trnID)
	if err != nil {
		return err
	}

	params := make(map[string]string)
	if tc.logLevel != "" {
		params["log_level"] = tc.logLevel
	}
	if tc.webhookID != "" {
		params["webhook_id"] = tc.webhookID
	}
	if tc.issueID != "" {
		params["issue_id"] = tc.issueID
	}
	if tc.createdAt != "" {
		params["created_at"] = tc.createdAt
	}
	if tc.orderBy != "" {
		params["order_by"] = tc.orderBy
	}
	if tc.dir != "" {
		params["dir"] = tc.dir
	}
	params["limit"] = strconv.Itoa(tc.limit)
	if tc.next != "" {
		params["next"] = tc.next
	}
	if tc.prev != "" {
		params["prev"] = tc.prev
	}

	resp, err := client.ListTransformationExecutions(ctx, trnID, params)
	if err != nil {
		return fmt.Errorf("failed to list executions: %w", err)
	}

	if tc.output == "json" {
		if len(resp.Models) == 0 {
			fmt.Println("[]")
			return nil
		}
		jsonBytes, err := json.MarshalIndent(resp.Models, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal executions to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	if len(resp.Models) == 0 {
		fmt.Println("No executions found.")
		return nil
	}

	color := ansi.Color(os.Stdout)
	for _, e := range resp.Models {
		fmt.Printf("%s %s\n", color.Green(e.ID), e.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	return nil
}

type transformationExecutionsGetCmd struct {
	cmd    *cobra.Command
	output string
}

func newTransformationExecutionsGetCmd() *transformationExecutionsGetCmd {
	tc := &transformationExecutionsGetCmd{}

	tc.cmd = &cobra.Command{
		Use:   "get <transformation-id-or-name> <execution-id>",
		Args:  validators.ExactArgs(2),
		Short: "Get a transformation execution",
		Long:  `Get a single execution by transformation ID and execution ID.`,
		RunE:  tc.run,
	}

	tc.cmd.Flags().StringVar(&tc.output, "output", "", "Output format (json)")

	return tc
}

func (tc *transformationExecutionsGetCmd) run(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}

	trnIDOrName := args[0]
	executionID := args[1]
	client := Config.GetAPIClient()
	ctx := context.Background()

	trnID, err := resolveTransformationID(ctx, client, trnIDOrName)
	if err != nil {
		return err
	}

	exec, err := client.GetTransformationExecution(ctx, trnID, executionID)
	if err != nil {
		return fmt.Errorf("failed to get execution: %w", err)
	}

	if tc.output == "json" {
		jsonBytes, err := json.MarshalIndent(exec, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal execution to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	color := ansi.Color(os.Stdout)
	fmt.Printf("\n%s\n", color.Green(exec.ID))
	fmt.Printf("  Created: %s\n", exec.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()
	return nil
}
