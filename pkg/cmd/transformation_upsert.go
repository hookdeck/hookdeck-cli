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

type transformationUpsertCmd struct {
	cmd      *cobra.Command
	name     string
	code     string
	codeFile string
	env      string
	dryRun   bool
	output   string
}

func newTransformationUpsertCmd() *transformationUpsertCmd {
	tc := &transformationUpsertCmd{}

	tc.cmd = &cobra.Command{
		Use:   "upsert <name>",
		Args:  validators.ExactArgs(1),
		Short: ShortUpsert(ResourceTransformation),
		Long: LongUpsertIntro(ResourceTransformation) + `

Examples:
  hookdeck gateway transformation upsert my-transform --code "addHandler(\"transform\", (request, context) => { return request; });"
  hookdeck gateway transformation upsert my-transform --code-file ./transform.js --env FOO=bar
  hookdeck gateway transformation upsert my-transform --code "addHandler(\"transform\", (request, context) => { return request; });" --dry-run`,
		PreRunE: tc.validateFlags,
		RunE:    tc.runTransformationUpsertCmd,
	}

	tc.cmd.Flags().StringVar(&tc.code, "code", "", "JavaScript code string")
	tc.cmd.Flags().StringVar(&tc.codeFile, "code-file", "", "Path to JavaScript file")
	tc.cmd.Flags().StringVar(&tc.env, "env", "", "Environment variables as KEY=value,KEY2=value2")
	tc.cmd.Flags().BoolVar(&tc.dryRun, "dry-run", false, "Preview changes without applying")
	tc.cmd.Flags().StringVar(&tc.output, "output", "", "Output format (json)")

	return tc
}

func (tc *transformationUpsertCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	tc.name = args[0]
	if tc.code != "" && tc.codeFile != "" {
		return fmt.Errorf("cannot use both --code and --code-file")
	}
	return nil
}

func (tc *transformationUpsertCmd) runTransformationUpsertCmd(cmd *cobra.Command, args []string) error {
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

	// API requires name + code on PUT. When user didn't provide code (partial update), fetch existing and merge.
	if req.Code == "" {
		params := map[string]string{"name": tc.name}
		listResp, err := client.ListTransformations(ctx, params)
		if err != nil || listResp.Models == nil || len(listResp.Models) == 0 {
			return fmt.Errorf("upsert requires --code or --code-file when creating a new transformation; no existing transformation named %q", tc.name)
		}
		existing := listResp.Models[0]
		existingFull, err := client.GetTransformation(ctx, existing.ID)
		if err != nil {
			return fmt.Errorf("failed to load existing transformation for merge: %w", err)
		}
		req.Code = existingFull.Code
		if len(req.Env) == 0 && len(existingFull.Env) > 0 {
			req.Env = existingFull.Env
		}
	}

	if tc.dryRun {
		params := map[string]string{"name": tc.name}
		existing, err := client.ListTransformations(ctx, params)
		if err != nil {
			return fmt.Errorf("dry-run: failed to check existing transformation: %w", err)
		}
		if existing.Models != nil && len(existing.Models) > 0 {
			fmt.Printf("-- Dry Run: UPDATE --\nTransformation '%s' (%s) would be updated.\n", tc.name, existing.Models[0].ID)
		} else {
			fmt.Printf("-- Dry Run: CREATE --\nTransformation '%s' would be created.\n", tc.name)
		}
		return nil
	}

	t, err := client.UpsertTransformation(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to upsert transformation: %w", err)
	}

	if tc.output == "json" {
		jsonBytes, err := json.MarshalIndent(t, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal transformation to json: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	fmt.Printf("✔ Transformation upserted successfully\n\n")
	fmt.Printf("Transformation: %s (%s)\n", t.Name, t.ID)
	return nil
}
