package cmd

import (
	"errors"
	"strconv"

	"github.com/spf13/cobra"
)

// includeDataDefaultLimit is the page size sent when --include is set without
// --limit, matching the MCP list tools.
//
// The usual default of 100 is fine for IDs and metadata. With payloads it is a
// hundred webhook bodies in one response, which is more than a terminal or a
// script piping to jq wants by accident. An explicit --limit is honoured.
const includeDataDefaultLimit = 10

// includeFlagUsage is the --include help for event list and request list.
func includeFlagUsage(resource string) string {
	return `Set to "data" to include each ` + resource + `'s payload (headers, body, path, parsed query) in the results. ` +
		`Requires --output json. Responses grow with every payload: when --limit is not set it defaults to ` +
		strconv.Itoa(includeDataDefaultLimit) + ` instead of 100`
}

// applyIncludeData adds include to the list query and returns the limit to
// send.
//
// The API declares include on GET /events and GET /requests with one value,
// "data"; any other value is left for the API to reject. --include is refused
// without --output json because the list output prints IDs and a summary
// line, so the payloads would be fetched and then not shown.
func applyIncludeData(cmd *cobra.Command, include, output string, limit int, params map[string]string) (int, error) {
	if include == "" {
		return limit, nil
	}
	if output != "json" {
		return 0, errors.New("--include requires --output json: the list output does not show payloads")
	}
	params["include"] = include
	if !cmd.Flags().Changed("limit") {
		return includeDataDefaultLimit, nil
	}
	return limit, nil
}
