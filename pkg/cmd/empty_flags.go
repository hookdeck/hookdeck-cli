package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// flagsRejectingEmptyValues lists flags for which an explicitly empty value has
// no meaning. Passing `--webhook-secret ""` never cleared a secret: the API
// expresses "no verification" as a null auth object, and the CLI's own
// sourceConfigFlags.hasAny() drops empty strings, so the flag was silently
// ignored — on create the source was left with no auth at all. Clearing is still
// available through the JSON escape hatches (`--config '{"auth": null}'`), which
// is why those flags only reject an empty *value*, not an explicit null.
//
// Identity flags are included for the same reason Cobra's MarkFlagRequired does
// not help: it tests whether a flag was Changed, not whether it has a value, so
// `--name ""` passes today.
//
// The trigger in practice is an unexported shell variable — `"$UNSET_VAR"`
// expands to an empty string, and the CLI is the last place that can catch it.
var flagsRejectingEmptyValues = map[string]bool{
	// Resource identity
	"name":             true,
	"type":             true,
	"url":              true,
	"source-name":      true,
	"source-type":      true,
	"source-id":        true,
	"destination-name": true,
	"destination-type": true,
	"destination-id":   true,
	"destination-url":  true,

	// Source authentication
	"webhook-secret":         true,
	"api-key":                true,
	"hmac-secret":            true,
	"hmac-algo":              true,
	"basic-auth-user":        true,
	"basic-auth-pass":        true,
	"source-webhook-secret":  true,
	"source-api-key":         true,
	"source-hmac-secret":     true,
	"source-hmac-algo":       true,
	"source-basic-auth-user": true,
	"source-basic-auth-pass": true,

	// Destination authentication
	"bearer-token":                        true,
	"api-key-header":                      true,
	"custom-signature-secret":             true,
	"custom-signature-key":                true,
	"destination-bearer-token":            true,
	"destination-api-key":                 true,
	"destination-api-key-header":          true,
	"destination-basic-auth-user":         true,
	"destination-basic-auth-pass":         true,
	"destination-custom-signature-secret": true,
	"destination-custom-signature-key":    true,
	"destination-oauth2-auth-server":      true,
	"destination-oauth2-client-id":        true,
	"destination-oauth2-client-secret":    true,
	"destination-oauth2-refresh-token":    true,
	"destination-aws-access-key-id":       true,
	"destination-aws-secret-access-key":   true,
	"destination-aws-region":              true,
	"destination-gcp-service-account-key": true,

	// Outpost identity flags. A tenant or event id that expands to an empty
	// string would silently address a different path rather than fail, so these
	// are rejected the same way the identity flags above are.
	"tenant-id": true,
	"event-id":  true,
	"topic":     true,
	"topics":    true,
	"theme":     true,
	"hostname":  true,

	// A filter rather than an identifier, but the failure is worse: an empty
	// value drops the filter, so `--id "$UNSET"` silently widens the query to
	// everything instead of narrowing it to one record. Only commands that call
	// rejectEmptyFlags are affected.
	"id": true,

	// JSON configuration escape hatches
	"config":                  true,
	"config-file":             true,
	"source-config":           true,
	"source-config-file":      true,
	"destination-config":      true,
	"destination-config-file": true,
	"rules-file":              true,
}

// rejectEmptyFlags returns an error if the caller explicitly passed an empty
// value to a flag where that cannot mean anything.
//
// It inspects only flags the user actually set — pflag's Visit walks changed
// flags — so an omitted flag is untouched and optional behaviour is preserved.
func rejectEmptyFlags(cmd *cobra.Command) error {
	var offenders []string

	cmd.Flags().Visit(func(f *pflag.Flag) {
		if !flagsRejectingEmptyValues[f.Name] {
			return
		}
		if strings.TrimSpace(f.Value.String()) == "" {
			offenders = append(offenders, f.Name)
		}
	})

	if len(offenders) == 0 {
		return nil
	}

	sort.Strings(offenders)

	flags := make([]string, 0, len(offenders))
	for _, name := range offenders {
		flags = append(flags, "--"+name)
	}

	// Name the likely cause, precisely. The reported case was a secret living in
	// a workspace .env that application code loaded but the shell never did, so
	// the variable was simply not set in the shell running the command. "Check it
	// is exported" would misdiagnose that, since exporting only matters for a
	// variable that is set here but not visible to child processes.
	if len(offenders) == 1 {
		return fmt.Errorf(
			"%s was empty. If you passed a shell variable, check it is set in this shell "+
				"(values in a .env file are not loaded automatically)",
			flags[0],
		)
	}

	return fmt.Errorf(
		"%s were empty. If you passed shell variables, check they are set in this shell "+
			"(values in a .env file are not loaded automatically)",
		strings.Join(flags, ", "),
	)
}
