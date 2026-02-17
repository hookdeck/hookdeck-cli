package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type transformationRunCmd struct {
	cmd               *cobra.Command
	code              string
	codeFile          string
	transformationID  string
	request           string
	requestFile       string
	connectionID      string
	env               string
	output            string
}

func newTransformationRunCmd() *transformationRunCmd {
	tc := &transformationRunCmd{}

	tc.cmd = &cobra.Command{
		Use:   "run",
		Args:  validators.NoArgs,
		Short: "Run transformation code (test)",
		Long: `Test run transformation code against a sample request.

Provide either inline --code/--code-file or --id to use an existing transformation.
The --request or --request-file must be JSON with at least "headers" (can be {}). Optional: body, path, query.

Examples:
  hookdeck gateway transformation run --id trs_abc123 --request '{"headers":{}}'
  hookdeck gateway transformation run --code "module.exports = async (r) => r;" --request-file ./sample.json
  hookdeck gateway transformation run --id trs_abc123 --request '{"headers":{},"body":{"foo":"bar"}}' --connection-id web_xxx`,
		PreRunE: tc.validateFlags,
		RunE:    tc.runTransformationRunCmd,
	}

	tc.cmd.Flags().StringVar(&tc.code, "code", "", "JavaScript code string to run")
	tc.cmd.Flags().StringVar(&tc.codeFile, "code-file", "", "Path to JavaScript file")
	tc.cmd.Flags().StringVar(&tc.transformationID, "id", "", "Use existing transformation by ID")
	tc.cmd.Flags().StringVar(&tc.request, "request", "", "Request JSON (must include headers, e.g. {\"headers\":{}})")
	tc.cmd.Flags().StringVar(&tc.requestFile, "request-file", "", "Path to request JSON file")
	tc.cmd.Flags().StringVar(&tc.connectionID, "connection-id", "", "Connection ID for execution context")
	tc.cmd.Flags().StringVar(&tc.env, "env", "", "Environment variables as KEY=value,KEY2=value2")
	tc.cmd.Flags().StringVar(&tc.output, "output", "", "Output format (json)")

	return tc
}

func (tc *transformationRunCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	if tc.code == "" && tc.codeFile == "" && tc.transformationID == "" {
		return fmt.Errorf("either --code, --code-file, or --id is required")
	}
	if tc.code != "" && tc.codeFile != "" {
		return fmt.Errorf("cannot use both --code and --code-file")
	}
	if tc.request != "" && tc.requestFile != "" {
		return fmt.Errorf("cannot use both --request and --request-file")
	}
	if tc.request == "" && tc.requestFile == "" {
		return fmt.Errorf("--request or --request-file is required (use {\"headers\":{}} for minimal request)")
	}
	return nil
}

func (tc *transformationRunCmd) runTransformationRunCmd(cmd *cobra.Command, args []string) error {
	client := Config.GetAPIClient()
	ctx := context.Background()

	code := tc.code
	if tc.codeFile != "" {
		b, err := os.ReadFile(tc.codeFile)
		if err != nil {
			return fmt.Errorf("failed to read --code-file: %w", err)
		}
		code = string(b)
	}

	requestJSON := tc.request
	if tc.requestFile != "" {
		b, err := os.ReadFile(tc.requestFile)
		if err != nil {
			return fmt.Errorf("failed to read --request-file: %w", err)
		}
		requestJSON = string(b)
	}

	var requestInput hookdeck.TransformationRunRequestInput
	if err := json.Unmarshal([]byte(requestJSON), &requestInput); err != nil {
		return fmt.Errorf("invalid --request JSON: %w", err)
	}
	if requestInput.Headers == nil {
		requestInput.Headers = make(map[string]string)
	}

	envMap, err := parseEnvFlag(tc.env)
	if err != nil {
		return err
	}

	req := &hookdeck.TransformationRunRequest{
		Request:     &requestInput,
		Env:         envMap,
		WebhookID:   tc.connectionID,
	}
	if code != "" {
		req.Code = code
	}
	if tc.transformationID != "" {
		req.TransformationID = tc.transformationID
	}

	result, err := client.RunTransformation(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to run transformation: %w", err)
	}

	if tc.output == "json" {
		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal result to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	fmt.Printf("✔ Transformation run completed\n\n")
	if result.Result != nil {
		fmt.Printf("Result: %v\n", result.Result)
	}
	return nil
}
