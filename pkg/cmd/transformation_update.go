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

type transformationUpdateCmd struct {
	cmd      *cobra.Command
	name     string
	code     string
	codeFile string
	env      string
	output   string
}

func newTransformationUpdateCmd() *transformationUpdateCmd {
	tc := &transformationUpdateCmd{}

	tc.cmd = &cobra.Command{
		Use:   "update <transformation-id-or-name>",
		Args:  validators.ExactArgs(1),
		Short: ShortUpdate(ResourceTransformation),
		Long: LongUpdateIntro(ResourceTransformation) + `

Examples:
  hookdeck gateway transformation update trn_abc123 --name new-name
  hookdeck gateway transformation update my-transform --code-file ./transform.js
  hookdeck gateway transformation update trn_abc123 --env FOO=bar`,
		PreRunE: tc.validateFlags,
		RunE:    tc.runTransformationUpdateCmd,
	}

	tc.cmd.Flags().StringVar(&tc.name, "name", "", "New transformation name")
	tc.cmd.Flags().StringVar(&tc.code, "code", "", "New JavaScript code string")
	tc.cmd.Flags().StringVar(&tc.codeFile, "code-file", "", "Path to JavaScript file")
	tc.cmd.Flags().StringVar(&tc.env, "env", "", "Environment variables as KEY=value,KEY2=value2")
	tc.cmd.Flags().StringVar(&tc.output, "output", "", "Output format (json)")

	return tc
}

func (tc *transformationUpdateCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	if tc.code != "" && tc.codeFile != "" {
		return fmt.Errorf("cannot use both --code and --code-file")
	}
	return nil
}

func (tc *transformationUpdateCmd) runTransformationUpdateCmd(cmd *cobra.Command, args []string) error {
	idOrName := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	trnID, err := resolveTransformationID(ctx, client, idOrName)
	if err != nil {
		return err
	}

	// Partial update: only set fields that were provided
	req := &hookdeck.TransformationUpdateRequest{}
	if tc.name != "" {
		req.Name = tc.name
	}
	if tc.code != "" {
		req.Code = tc.code
	}
	if tc.codeFile != "" {
		b, err := os.ReadFile(tc.codeFile)
		if err != nil {
			return fmt.Errorf("failed to read --code-file: %w", err)
		}
		req.Code = string(b)
	}
	if tc.env != "" {
		envMap, err := parseEnvFlag(tc.env)
		if err != nil {
			return err
		}
		req.Env = envMap
	}

	// At least one field must change
	hasUpdate := req.Name != "" || req.Code != "" || len(req.Env) > 0
	if !hasUpdate {
		return fmt.Errorf("no updates specified (set at least one of --name, --code, --code-file, or --env)")
	}

	t, err := client.UpdateTransformation(ctx, trnID, req)
	if err != nil {
		return fmt.Errorf("failed to update transformation: %w", err)
	}

	if tc.output == "json" {
		jsonBytes, err := json.MarshalIndent(t, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal transformation to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	fmt.Printf("✔ Transformation updated successfully\n\n")
	fmt.Printf("Transformation: %s (%s)\n", t.Name, t.ID)
	return nil
}
