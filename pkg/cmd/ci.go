package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/login"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type ciCmd struct {
	cmd    *cobra.Command
	apiKey string
	name   string
	local  bool
}

func newCICmd() *ciCmd {
	lc := &ciCmd{}

	lc.cmd = &cobra.Command{
		Use:   "ci",
		Args:  validators.NoArgs,
		Short: "Login to your Hookdeck project in CI",
		Long:  `If you want to use Hookdeck in CI for tests or any other purposes, you can use your HOOKDECK_API_KEY to authenticate and start forwarding events.`,
		Example: `$ hookdeck ci --api-key $HOOKDECK_API_KEY
Done! The Hookdeck CLI is configured in project MyProject

$ hookdeck listen 3000 shopify orders

●── HOOKDECK CLI ──●

Listening on 1 source • 1 connection • [i] Collapse

Shopify Source
│  Requests to → https://hkdk.events/src_DAjaFWyyZXsFdZrTOKpuHnOH
└─ Forwards to → http://localhost:3000/webhooks/shopify/orders (Orders Service)

💡 View dashboard to inspect, retry & bookmark events: https://dashboard.hookdeck.com/events/cli?team_id=...

Events • [↑↓] Navigate ──────────────────────────────────────────────────────────

> 2025-10-12 14:42:55 [200] POST http://localhost:3000/webhooks/shopify/orders (34ms) → https://dashboard.hookdeck.com/events/evt_...

───────────────────────────────────────────────────────────────────────────────
> ✓ Last event succeeded with status 200 | [r] Retry • [o] Open in dashboard • [d] Show data`,
		RunE: lc.runCICmd,
	}
	lc.cmd.Flags().StringVar(&lc.apiKey, "api-key", os.Getenv("HOOKDECK_API_KEY"), "Your Hookdeck Project API key. The CLI reads from HOOKDECK_API_KEY if not provided.")
	lc.cmd.Flags().StringVar(&lc.name, "name", "", "Name of the CI run (ex: GITHUB_REF) for identification in the dashboard")
	lc.cmd.Flags().BoolVar(&lc.local, "local", false, "Save credentials to current directory (.hookdeck/config.toml)")

	return lc
}

func (lc *ciCmd) runCICmd(cmd *cobra.Command, args []string) error {
	if lc.local && Config.ConfigFileFlag != "" {
		return fmt.Errorf("Error: --local and --config flags cannot be used together\n  --local creates config at: .hookdeck/config.toml\n  --config uses custom path: %s", Config.ConfigFileFlag)
	}

	err := validators.APIKey(lc.apiKey)
	if err != nil {
		if err == validators.ErrAPIKeyNotConfigured {
			return fmt.Errorf("Provide a project API key using the --api-key flag. Example: hookdeck ci --api-key YOUR_KEY")
		}
		return err
	}

	if err := login.CILogin(&Config, lc.apiKey, lc.name); err != nil {
		return err
	}

	if lc.local {
		return saveLocalConfig()
	}

	return nil
}
