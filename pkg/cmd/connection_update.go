package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type connectionUpdateCmd struct {
	cmd *cobra.Command

	output string

	// Connection fields (update-by-ID only; no inline source/destination)
	name          string
	description   string
	sourceID      string
	destinationID string

	// Rule flags shared with create/upsert
	connectionRuleFlags
}

func newConnectionUpdateCmd() *connectionUpdateCmd {
	cu := &connectionUpdateCmd{}

	cu.cmd = &cobra.Command{
		Use:   "update <connection-id>",
		Args:  validators.ExactArgs(1),
		Short: ShortUpdate(ResourceConnection),
		Long: LongUpdateIntro(ResourceConnection) + `

Unlike upsert (which uses name as identifier), update takes a connection ID
and allows changing any field including the connection name.

Examples:
  # Rename a connection
  hookdeck gateway connection update web_abc123 --name "new-name"

  # Update description
  hookdeck gateway connection update web_abc123 --description "Updated description"

  # A connection's source and destination cannot be changed. --source-id and
  # --destination-id are accepted only when they match the stored values;
  # to use a different one, create a new connection.

  # Update rules
  hookdeck gateway connection update web_abc123 \
    --rule-retry-strategy linear --rule-retry-count 5

  # Update with JSON output
  hookdeck gateway connection update web_abc123 --name "new-name" --output json`,
		PreRunE: cu.validateFlags,
		RunE:    cu.runConnectionUpdateCmd,
	}
	cu.cmd.Annotations = map[string]string{
		"cli.arguments": `[{"name":"connection-id","type":"string","description":"Connection ID","required":true}]`,
	}

	// Connection fields
	cu.cmd.Flags().StringVar(&cu.name, "name", "", "New connection name")
	cu.cmd.Flags().StringVar(&cu.description, "description", "", "Connection description")

	// Resource references
	cu.cmd.Flags().StringVar(&cu.sourceID, "source-id", "", "Source ID the connection already uses (a different value is rejected: a connection's source cannot be changed)")
	cu.cmd.Flags().StringVar(&cu.destinationID, "destination-id", "", "Destination ID the connection already uses (a different value is rejected: a connection's destination cannot be changed)")

	addConnectionRuleFlags(cu.cmd, &cu.connectionRuleFlags)

	// Output
	cu.cmd.Flags().StringVar(&cu.output, "output", "", "Output format (json)")

	return cu
}

func (cu *connectionUpdateCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := Config.Profile.ValidateAPIKey(); err != nil {
		return err
	}
	return nil
}

func (cu *connectionUpdateCmd) runConnectionUpdateCmd(cmd *cobra.Command, args []string) error {
	connectionID := args[0]
	client := Config.GetAPIClient()
	ctx := context.Background()

	req := &hookdeck.ConnectionCreateRequest{}
	hasChanges := false

	if cu.name != "" {
		req.Name = &cu.name
		hasChanges = true
	}

	if cu.description != "" {
		req.Description = &cu.description
		hasChanges = true
	}

	// The API never changes an existing connection's source or destination
	// (PUT /connections/{id} drops source_id and destination_id), so a value
	// that differs from the stored one is an error rather than a silent no-op
	// reported as success (#466). A value equal to the stored one is not sent.
	if cu.sourceID != "" || cu.destinationID != "" {
		current, err := client.GetConnection(ctx, connectionID)
		if err != nil {
			return fmt.Errorf("failed to get connection: %w", err)
		}
		if err := hookdeck.CheckConnectionSidesUnchanged(current, cu.sourceID, "", cu.destinationID, ""); err != nil {
			return err
		}
	}

	// Build rules if any rule flags are set
	rules, err := buildConnectionRules(&cu.connectionRuleFlags)
	if err != nil {
		return err
	}
	if err := resolveRuleTransformation(ctx, client, &cu.connectionRuleFlags, rules); err != nil {
		return err
	}
	// Keyed off whether the flag was given rather than whether it produced any
	// rules, so that an explicit empty array reaches the API and clears the
	// ruleset instead of being read as "no rules mentioned".
	if cmd.Flags().Changed("rules") || cmd.Flags().Changed("rules-file") {
		if rules == nil {
			rules = []hookdeck.Rule{}
		}
		req.Rules = &rules
		hasChanges = true
	} else if len(rules) > 0 {
		req.Rules = &rules
		hasChanges = true
	}

	if !hasChanges {
		// No flags provided; get and display current state
		conn, err := client.GetConnection(ctx, connectionID)
		if err != nil {
			return fmt.Errorf("failed to get connection: %w", err)
		}
		cu.displayConnection(conn, false)
		return nil
	}

	conn, err := client.UpdateConnection(ctx, connectionID, req)
	if err != nil {
		return fmt.Errorf("failed to update connection: %w", err)
	}

	cu.displayConnection(conn, true)
	return nil
}

func (cu *connectionUpdateCmd) displayConnection(conn *hookdeck.Connection, updated bool) {
	if cu.output == "json" {
		jsonBytes, err := json.MarshalIndent(conn, "", "  ")
		if err != nil {
			fmt.Printf("failed to marshal connection to json: %v\n", err)
			return
		}
		fmt.Println(string(jsonBytes))
		return
	}

	if updated {
		fmt.Println(SuccessCheck + " Connection updated successfully")
	} else {
		fmt.Println("No changes specified. Current connection state:")
	}
	fmt.Println()

	if conn.Name != nil {
		fmt.Printf("Connection:  %s (%s)\n", *conn.Name, conn.ID)
	} else {
		fmt.Printf("Connection:  (unnamed) (%s)\n", conn.ID)
	}

	if conn.Source != nil {
		fmt.Printf("Source:      %s (%s)\n", conn.Source.Name, conn.Source.ID)
		fmt.Printf("Source Type: %s\n", conn.Source.Type)
	}

	if conn.Destination != nil {
		fmt.Printf("Destination: %s (%s)\n", conn.Destination.Name, conn.Destination.ID)
		fmt.Printf("Destination Type: %s\n", conn.Destination.Type)

		switch strings.ToUpper(conn.Destination.Type) {
		case "HTTP":
			if url := conn.Destination.GetHTTPURL(); url != nil {
				fmt.Printf("Destination URL: %s\n", *url)
			}
		case "CLI":
			if path := conn.Destination.GetCLIPath(); path != nil {
				fmt.Printf("Destination Path: %s\n", *path)
			}
		}
	}
}
