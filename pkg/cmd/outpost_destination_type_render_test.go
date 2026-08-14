package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/cmd/outposttypes"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func testKafkaSchema() outposttypes.Schema {
	return outposttypes.Schema{
		Type:  "kafka",
		Label: "Apache Kafka",
		ConfigFields: []outposttypes.Field{
			{Key: "brokers", Label: "Brokers", Required: true},
			{Key: "tls", Label: "TLS", Default: "true"},
			{Key: "sasl_mechanism", Label: "SASL Mechanism", Required: true, Options: []hookdeck.OutpostDestinationTypeOption{
				{Label: "PLAIN", Value: "plain"},
				{Label: "SCRAM 256", Value: "scram-sha-256"},
			}},
		},
		CredentialFields: []outposttypes.Field{
			{Key: "password", Label: "Password", Required: true, Sensitive: true},
		},
	}
}

func TestWriteOutpostDestinationTypeFields(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	writeOutpostDestinationTypeFields(&buf, testKafkaSchema(), true)
	out := buf.String()

	assert.Contains(t, out, "--config fields:")
	assert.Contains(t, out, "--credential fields:")
	assert.Contains(t, out, "brokers")
	assert.Contains(t, out, "required")
	assert.Contains(t, out, "one of: plain, scram-sha-256")
	assert.Contains(t, out, "default: true")
	assert.Contains(t, out, "sensitive", "a sensitive field must be flagged as such")

	// The example must contain every required field and no optional one, so it
	// can be pasted and run.
	assert.Contains(t, out, "--config brokers=<brokers>")
	assert.Contains(t, out, "--config sasl_mechanism=<sasl_mechanism>")
	assert.Contains(t, out, "--credential password=<password>")
	assert.NotContains(t, out, "--config tls=", "optional fields should stay out of the example")
}

func TestDescribeOutpostField(t *testing.T) {
	t.Parallel()

	assert.Contains(t, describeOutpostField(outposttypes.Field{Key: "x"}), "optional")
	assert.Contains(t, describeOutpostField(outposttypes.Field{Key: "x", Required: true}), "required")
}

// TestOutpostHelpDoesNotMutateCommandMetadata is the guard for REFERENCE.md.
//
// The generator reads Long and the flag definitions directly rather than
// invoking help, so dynamic help output cannot reach it — but only for as long
// as the help function keeps its changes to the output stream.
func TestOutpostHelpDoesNotMutateCommandMetadata(t *testing.T) {
	t.Parallel()

	destType := "kafka"
	cmd := &cobra.Command{
		Use:  "create",
		Long: "Static long text.",
		Run:  func(cmd *cobra.Command, args []string) {},
	}
	cmd.Flags().StringVar(&destType, "type", "kafka", "Destination type")

	longBefore := cmd.Long
	usageBefore := cmd.Flags().Lookup("type").Usage

	addOutpostDestinationTypeHelp(cmd, &destType)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.Help()

	assert.Equal(t, longBefore, cmd.Long, "help must not rewrite Long; REFERENCE.md is generated from it")
	assert.Equal(t, usageBefore, cmd.Flags().Lookup("type").Usage, "help must not rewrite flag usage strings")
}

func TestOutpostHelpWithoutTypeShowsPointer(t *testing.T) {
	t.Parallel()

	empty := ""
	cmd := &cobra.Command{Use: "create", Run: func(cmd *cobra.Command, args []string) {}}
	addOutpostDestinationTypeHelp(cmd, &empty)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.Help()

	out := buf.String()
	require.NotEmpty(t, out)
	assert.Contains(t, out, "--type <type> --help", "plain help should say how to get per-type fields")
	assert.Contains(t, out, "destination-type list")
	// No schema lookup should be attempted without a type, so nothing can block.
	assert.False(t, strings.Contains(out, "--config fields:"))
}
