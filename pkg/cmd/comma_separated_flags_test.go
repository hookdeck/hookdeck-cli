package cmd

import (
	"regexp"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// A flag whose help says "comma-separated" is a promise that a list reaches the
// API. Twice that promise was broken the same way: the value went out as one
// query parameter, which the API matched against nothing, so the command
// returned zero rows with exit 0. --id did it (#411), and --delivery-group did
// it again after that fix because nothing tied the promise to the encoding.
//
// So every such flag must appear below, with how its value is actually sent,
// and a query filter must be one the client expands into a list. A new
// comma-separated flag fails here until someone checks it and says how.

type commaFlagHandling int

const (
	// A filter on a list endpoint, sent through hookdeck.listQuery. The param
	// must be list-valued there, or a comma-joined value goes out as one.
	sentAsQueryList commaFlagHandling = iota
	// Split into an array in the JSON request body before sending.
	splitIntoBodyArray
	// Split into a slice the client sends as a list (metrics measures and
	// dimensions; every Outpost filter, via splitCommaList).
	splitIntoList
	// Sent as one string on purpose: the API turns the commas into spaces
	// (OAuth2 scopes, which the token endpoint expects space-separated).
	sentAsStringAPIConvertsCommas
)

type commaFlag struct {
	kind  commaFlagHandling
	param string // for sentAsQueryList: the query parameter the flag sets
}

var commaSeparatedFlags = map[string]commaFlag{
	"hookdeck gateway event list --delivery-group":                          {kind: sentAsQueryList, param: "delivery_group"},
	"hookdeck gateway event list --id":                                      {kind: sentAsQueryList, param: "id"},
	"hookdeck gateway request events --delivery-group":                      {kind: sentAsQueryList, param: "delivery_group"},
	"hookdeck gateway request list --id":                                    {kind: sentAsQueryList, param: "id"},
	"hookdeck connection create --rule-deduplicate-exclude-fields":          {kind: splitIntoBodyArray},
	"hookdeck connection create --rule-deduplicate-include-fields":          {kind: splitIntoBodyArray},
	"hookdeck connection create --rule-retry-response-status-codes":         {kind: splitIntoBodyArray},
	"hookdeck connection create --source-allowed-http-methods":              {kind: splitIntoBodyArray},
	"hookdeck connection update --rule-deduplicate-exclude-fields":          {kind: splitIntoBodyArray},
	"hookdeck connection update --rule-deduplicate-include-fields":          {kind: splitIntoBodyArray},
	"hookdeck connection update --rule-retry-response-status-codes":         {kind: splitIntoBodyArray},
	"hookdeck connection upsert --rule-deduplicate-exclude-fields":          {kind: splitIntoBodyArray},
	"hookdeck connection upsert --rule-deduplicate-include-fields":          {kind: splitIntoBodyArray},
	"hookdeck connection upsert --rule-retry-response-status-codes":         {kind: splitIntoBodyArray},
	"hookdeck connection upsert --source-allowed-http-methods":              {kind: splitIntoBodyArray},
	"hookdeck gateway connection create --rule-deduplicate-exclude-fields":  {kind: splitIntoBodyArray},
	"hookdeck gateway connection create --rule-deduplicate-include-fields":  {kind: splitIntoBodyArray},
	"hookdeck gateway connection create --rule-retry-response-status-codes": {kind: splitIntoBodyArray},
	"hookdeck gateway connection create --source-allowed-http-methods":      {kind: splitIntoBodyArray},
	"hookdeck gateway connection update --rule-deduplicate-exclude-fields":  {kind: splitIntoBodyArray},
	"hookdeck gateway connection update --rule-deduplicate-include-fields":  {kind: splitIntoBodyArray},
	"hookdeck gateway connection update --rule-retry-response-status-codes": {kind: splitIntoBodyArray},
	"hookdeck gateway connection upsert --rule-deduplicate-exclude-fields":  {kind: splitIntoBodyArray},
	"hookdeck gateway connection upsert --rule-deduplicate-include-fields":  {kind: splitIntoBodyArray},
	"hookdeck gateway connection upsert --rule-retry-response-status-codes": {kind: splitIntoBodyArray},
	"hookdeck gateway connection upsert --source-allowed-http-methods":      {kind: splitIntoBodyArray},
	"hookdeck gateway request retry --connection-ids":                       {kind: splitIntoBodyArray},
	"hookdeck gateway source create --allowed-http-methods":                 {kind: splitIntoBodyArray},
	"hookdeck gateway source update --allowed-http-methods":                 {kind: splitIntoBodyArray},
	"hookdeck gateway source upsert --allowed-http-methods":                 {kind: splitIntoBodyArray},
	"hookdeck connection create --destination-oauth2-scopes":                {kind: sentAsStringAPIConvertsCommas},
	"hookdeck connection upsert --destination-oauth2-scopes":                {kind: sentAsStringAPIConvertsCommas},
	"hookdeck gateway connection create --destination-oauth2-scopes":        {kind: sentAsStringAPIConvertsCommas},
	"hookdeck gateway connection upsert --destination-oauth2-scopes":        {kind: sentAsStringAPIConvertsCommas},
	"hookdeck gateway metrics attempts --dimensions":                        {kind: splitIntoList},
	"hookdeck gateway metrics attempts --measures":                          {kind: splitIntoList},
	"hookdeck gateway metrics events --dimensions":                          {kind: splitIntoList},
	"hookdeck gateway metrics events --measures":                            {kind: splitIntoList},
	"hookdeck gateway metrics requests --dimensions":                        {kind: splitIntoList},
	"hookdeck gateway metrics requests --measures":                          {kind: splitIntoList},
	"hookdeck gateway metrics transformations --dimensions":                 {kind: splitIntoList},
	"hookdeck gateway metrics transformations --measures":                   {kind: splitIntoList},
	"hookdeck outpost attempt get --include":                                {kind: splitIntoList},
	"hookdeck outpost attempt list --destination-id":                        {kind: splitIntoList},
	"hookdeck outpost attempt list --destination-type":                      {kind: splitIntoList},
	"hookdeck outpost attempt list --event-id":                              {kind: splitIntoList},
	"hookdeck outpost attempt list --include":                               {kind: splitIntoList},
	"hookdeck outpost attempt list --tenant-id":                             {kind: splitIntoList},
	"hookdeck outpost attempt list --topic":                                 {kind: splitIntoList},
	"hookdeck outpost destination create --topics":                          {kind: splitIntoList},
	"hookdeck outpost destination list --topics":                            {kind: splitIntoList},
	"hookdeck outpost destination list --type":                              {kind: splitIntoList},
	"hookdeck outpost destination update --topics":                          {kind: splitIntoList},
	"hookdeck outpost event list --destination-id":                          {kind: splitIntoList},
	"hookdeck outpost event list --id":                                      {kind: splitIntoList},
	"hookdeck outpost event list --tenant-id":                               {kind: splitIntoList},
	"hookdeck outpost event list --topic":                                   {kind: splitIntoList},
	"hookdeck outpost metrics attempts --dimensions":                        {kind: splitIntoList},
	"hookdeck outpost metrics attempts --measures":                          {kind: splitIntoList},
	"hookdeck outpost metrics events --dimensions":                          {kind: splitIntoList},
	"hookdeck outpost metrics events --measures":                            {kind: splitIntoList},
	"hookdeck outpost tenant list --id":                                     {kind: splitIntoList},
}

var commaSeparated = regexp.MustCompile(`(?i)comma[- ]separated`)

func discoverCommaSeparatedFlags() map[string]bool {
	found := map[string]bool{}
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		// Flags() and PersistentFlags(), not LocalFlags(): LocalFlags merges
		// every parent's persistent flags into the command as a side effect,
		// which mutates the shared rootCmd for every later test in the package.
		// It put root's hidden --api-key onto outpost mcp and failed
		// TestOutpostMCPCommandIsRegistered.
		visit := func(f *pflag.Flag) {
			if commaSeparated.MatchString(f.Usage) {
				found[c.CommandPath()+" --"+f.Name] = true
			}
		}
		c.Flags().VisitAll(visit)
		c.PersistentFlags().VisitAll(visit)
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(rootCmd)
	return found
}

func TestEveryCommaSeparatedFlagSendsAList(t *testing.T) {
	found := discoverCommaSeparatedFlags()
	if len(found) < 40 {
		t.Fatalf("found only %d comma-separated flags; the walk is probably broken and the guard would pass vacuously", len(found))
	}

	for flag := range found {
		handling, ok := commaSeparatedFlags[flag]
		if !assert.True(t, ok,
			"%s says comma-separated but is not in commaSeparatedFlags. Check how its value "+
				"reaches the API -- a query filter sent as one value returns nothing (#411) -- "+
				"and add it with its handling.", flag) {
			continue
		}
		if handling.kind == sentAsQueryList {
			assert.True(t, hookdeck.IsListValuedParam(handling.param),
				"%s is a comma-separated query filter, but %q is not list-valued in hookdeck.listQuery, "+
					"so \"a,b\" is sent as one value and matches nothing", flag, handling.param)
		}
	}

	// A stale entry would quietly cover a different flag added under the same
	// name later, so every entry must still exist.
	for flag := range commaSeparatedFlags {
		assert.True(t, found[flag], "%s is in commaSeparatedFlags but no longer exists or no longer says comma-separated", flag)
	}
}
