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

type outpostPublishCmd struct {
	cmd *cobra.Command

	apiKey           string
	tenantID         string
	topic            string
	destinationID    string
	eventID          string
	data             string
	dataFile         string
	metadata         []string
	eligibleForRetry bool
	output           string
}

func newOutpostPublishCmd() *outpostPublishCmd {
	pc := &outpostPublishCmd{}

	pc.cmd = &cobra.Command{
		Use:   "publish",
		Args:  validators.NoArgs,
		Short: ShortBeta("Publish an event"),
		Long: LongBeta(`Publish an event to a topic, for delivery to a tenant's matching destinations.

Publishing is asynchronous: a successful response means the event was accepted,
not that it has been delivered.

This command needs a Hookdeck Project API key, which is different from every
other outpost command. The credentials stored by 'hookdeck login' are not
accepted by the publish API, so pass --api-key or set HOOKDECK_API_KEY. You can
create a Project API key in the Hookdeck dashboard under project settings.`),
		PreRunE: pc.validateFlags,
		RunE:    pc.run,
		Example: `  # Publish an event
  hookdeck outpost publish --tenant-id acme --topic user.created \
    --data '{"user_id":"123"}' --api-key $HOOKDECK_API_KEY

  # Publish to one specific destination
  hookdeck outpost publish --tenant-id acme --topic user.created \
    --data '{"user_id":"123"}' --destination-id des_abc123

  # Idempotent publish: repeating the same --event-id will not duplicate
  hookdeck outpost publish --tenant-id acme --topic user.created \
    --event-id my-unique-id --data-file ./payload.json`,
	}

	// The env var is read at run time rather than used as the flag default:
	// pflag prints a non-empty string default in --help, so a key already in
	// the environment would be echoed back out — and the reference-doc
	// generator reads flag defaults too.
	pc.cmd.Flags().StringVar(&pc.apiKey, "api-key", "", "Hookdeck Project API key. Read from HOOKDECK_API_KEY when not provided.")
	pc.cmd.Flags().StringVar(&pc.tenantID, "tenant-id", "", "Tenant to publish for (required)")
	pc.cmd.Flags().StringVar(&pc.topic, "topic", "", "Topic to publish to (required)")
	pc.cmd.Flags().StringVar(&pc.destinationID, "destination-id", "", "Deliver only to this destination")
	pc.cmd.Flags().StringVar(&pc.eventID, "event-id", "", "Event ID, for idempotent publishing")
	pc.cmd.Flags().StringVar(&pc.data, "data", "", "Event payload as a JSON object")
	pc.cmd.Flags().StringVar(&pc.dataFile, "data-file", "", "Path to a JSON file containing the event payload")
	pc.cmd.Flags().StringArrayVar(&pc.metadata, "metadata", nil, "Metadata as key=value (repeatable)")
	pc.cmd.Flags().BoolVar(&pc.eligibleForRetry, "eligible-for-retry", true, "Whether failed deliveries should be retried")
	pc.cmd.Flags().StringVar(&pc.output, "output", "", "Output format (json)")

	pc.cmd.MarkFlagRequired("tenant-id")
	pc.cmd.MarkFlagRequired("topic")

	return pc
}

func (pc *outpostPublishCmd) validateFlags(cmd *cobra.Command, args []string) error {
	if err := rejectEmptyFlags(cmd); err != nil {
		return err
	}
	if pc.data != "" && pc.dataFile != "" {
		return fmt.Errorf("--data and --data-file cannot be used together")
	}

	if pc.apiKey == "" {
		pc.apiKey = envAPIKey()
	}

	// Fail here with the reason rather than letting this surface as a bare 401,
	// which the generic handler would rewrite into "your API key is invalid or
	// expired" — true, but useless, since the stored key is never valid here.
	if pc.apiKey == "" {
		return newActionableError(fmt.Errorf(
			"publishing requires a Hookdeck Project API key.\n\n" +
				"Unlike other outpost commands, the publish API does not accept the credentials\n" +
				"stored by 'hookdeck login'. Pass one explicitly:\n\n" +
				"  hookdeck outpost publish --api-key <project-api-key> ...\n\n" +
				"or set HOOKDECK_API_KEY. Create a Project API key in the Hookdeck dashboard\n" +
				"under your project's settings."))
	}

	return nil
}

func (pc *outpostPublishCmd) run(cmd *cobra.Command, args []string) error {
	payload, err := pc.resolveData()
	if err != nil {
		return err
	}

	metadata := make(map[string]string, len(pc.metadata))
	for _, entry := range pc.metadata {
		key, value, found := strings.Cut(entry, "=")
		key = strings.TrimSpace(key)
		if !found || key == "" {
			return fmt.Errorf("--metadata %q must be in key=value form", entry)
		}
		metadata[key] = value
	}

	req := &hookdeck.OutpostPublishRequest{
		ID:            pc.eventID,
		TenantID:      pc.tenantID,
		Topic:         pc.topic,
		DestinationID: pc.destinationID,
		Metadata:      metadata,
		Data:          payload,
	}
	// Only send the flag when the caller set it, so the API default stands.
	if cmd.Flags().Changed("eligible-for-retry") {
		req.EligibleForRetry = &pc.eligibleForRetry
	}

	client := Config.GetOutpostAPIClient()

	resp, err := client.PublishOutpostEvent(context.Background(), pc.apiKey, req)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	if pc.output == "json" {
		return printJSONIndented(resp)
	}

	if resp.Duplicate {
		fmt.Printf("%s Event %s already existed; nothing was published again\n", SuccessCheck, resp.ID)
		return nil
	}

	fmt.Printf("%s Event %s accepted\n", SuccessCheck, resp.ID)
	if len(resp.DestinationIDs) > 0 {
		fmt.Printf("  Matched destinations: %s\n", strings.Join(resp.DestinationIDs, ", "))
	} else {
		fmt.Println("  No destinations matched this topic, so it will not be delivered.")
	}

	return nil
}

func (pc *outpostPublishCmd) resolveData() (map[string]interface{}, error) {
	raw := pc.data
	if pc.dataFile != "" {
		contents, err := os.ReadFile(pc.dataFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read --data-file: %w", err)
		}
		raw = string(contents)
	}
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("the event payload must be a JSON object: %w", err)
	}
	return payload, nil
}
