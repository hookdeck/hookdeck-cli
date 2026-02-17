package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type transformationCreateCmd struct {
	cmd       *cobra.Command
	name      string
	code      string
	codeFile  string
	env       string
	output    string
}

func newTransformationCreateCmd() *transformationCreateCmd {
	tc := &transformationCreateCmd{}

	tc.cmd = &cobra.Command{
		Use:   "create",
		Args:  validators.NoArgs,
		Short: ShortCreate(ResourceTransformation),
		Long: `Create a new transformation.

Requires --name and --code (or --code-file). Use --env for key-value environment variables.

Examples:
  hookdeck gateway transformation create --name my-transform --code "module.exports = async (req) => req;"
  hookdeck gateway transformation create --name my-transform --code-file ./transform.js --env FOO=bar,BAZ=qux`,
		PreRunE: tc.validateFlags,
		RunE:    tc.runTransformationCreateCmd,
	}

	tc.cmd.Flags().StringVar(&tc.name, "name", "", "Transformation name (required)")
	tc.cmd.Flags().StringVar(&tc.code, "code", "", "JavaScript code string (required if --code-file not set)")
	tc.cmd.Flags().StringVar(&tc.codeFile, "code-file", "", "Path to JavaScript file (required if --code not set)")
	tc.cmd.Flags().StringVar(&tc.env, "env", "", "Environment variables as KEY=value,KEY2=value2")
	tc.cmd.Flags().StringVar(&tc.output, "output", "", "Output format (json)")

	return tc
}

func (tc *transformationCreateCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	if tc.code == "" && tc.codeFile == "" {
		return fmt.Errorf("either --code or --code-file is required")
	}
	if tc.code != "" && tc.codeFile != "" {
		return fmt.Errorf("cannot use both --code and --code-file")
	}
	if tc.name == "" {
		return fmt.Errorf("--name is required")
	}
	return nil
}

func (tc *transformationCreateCmd) runTransformationCreateCmd(cmd *cobra.Command, args []string) error {
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

	envMap, err := parseEnvFlag(tc.env)
	if err != nil {
		return err
	}

	req := &hookdeck.TransformationCreateRequest{
		Name: tc.name,
		Code: code,
		Env:  envMap,
	}

	t, err := client.CreateTransformation(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create transformation: %w", err)
	}

	if tc.output == "json" {
		jsonBytes, err := json.MarshalIndent(t, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal transformation to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	fmt.Printf("✔ Transformation created successfully\n\n")
	fmt.Printf("Transformation: %s (%s)\n", t.Name, t.ID)
	return nil
}

// parseEnvFlag parses KEY=value,KEY2=value2 into map[string]string.
func parseEnvFlag(s string) (map[string]string, error) {
	if s == "" {
		return nil, nil
	}
	out := make(map[string]string)
	for _, pair := range strings.Split(s, ",") {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid env pair %q (expected KEY=value)", pair)
		}
		out[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	return out, nil
}
