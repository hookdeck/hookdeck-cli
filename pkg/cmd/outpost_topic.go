package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

type outpostTopicCmd struct {
	cmd *cobra.Command
}

func newOutpostTopicCmd() *outpostTopicCmd {
	tc := &outpostTopicCmd{}

	tc.cmd = &cobra.Command{
		Use:     "topic",
		Aliases: []string{"topics"},
		Args:    validators.NoArgs,
		Short:   ShortBeta("Inspect available topics"),
		Long: LongBeta(`Inspect the topics destinations can subscribe to.

Topics are project configuration rather than a resource, so there is no create
command. Change them with 'hookdeck outpost config set TOPICS=a,b,c'.`),
	}

	tc.cmd.AddCommand(newOutpostTopicListCmd().cmd)

	return tc
}

type outpostTopicListCmd struct {
	cmd *cobra.Command

	output string
}

func newOutpostTopicListCmd() *outpostTopicListCmd {
	tc := &outpostTopicListCmd{}

	tc.cmd = &cobra.Command{
		Use:   "list",
		Args:  validators.NoArgs,
		Short: ShortList(ResourceTopic),
		Long:  `List the topics configured for this project.`,
		RunE:  tc.run,
		Example: `  # List topics
  hookdeck outpost topic list`,
	}

	tc.cmd.Flags().StringVar(&tc.output, "output", "", "Output format (json)")

	return tc
}

func (tc *outpostTopicListCmd) run(cmd *cobra.Command, args []string) error {
	client := Config.GetOutpostAPIClient()

	topics, err := client.ListOutpostTopics(context.Background())
	if err != nil {
		return fmt.Errorf("failed to list topics: %w", err)
	}

	if tc.output == "json" {
		return printJSONIndented(topics)
	}

	if len(topics) == 0 {
		// An empty list is valid but leaves the project unusable, so say what to
		// do rather than printing nothing.
		fmt.Println("No topics configured.")
		fmt.Println("\nDestinations cannot subscribe to anything until topics are set:")
		fmt.Println("  hookdeck outpost config set TOPICS=user.created,user.updated")
		return nil
	}

	color := ansi.Color(os.Stdout)
	fmt.Printf("\nFound %d topic(s):\n\n", len(topics))
	for _, topic := range topics {
		fmt.Printf("  %s\n", color.Green(topic))
	}
	fmt.Println()

	return nil
}
