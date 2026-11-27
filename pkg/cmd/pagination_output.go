package cmd

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// printPaginationInfo displays pagination cursors for text output
func printPaginationInfo(cmd *cobra.Command, args []string, pagination hookdeck.PaginationResponse) {
	if pagination.Next == nil && pagination.Prev == nil {
		return
	}

	fmt.Println()
	fmt.Println("Pagination:")
	if pagination.Prev != nil {
		fmt.Printf("  Prev: %s\n", *pagination.Prev)
	}
	if pagination.Next != nil {
		fmt.Printf("  Next: %s\n", *pagination.Next)
	}

	if pagination.Next != nil {
		fmt.Println()
		fmt.Println("To get the next page:")
		fmt.Printf("  %s\n", nextPageCommandLine(cmd, args, *pagination.Next))
	}
}

// nextPageCommandLine rebuilds the command that was run, pointed at the next
// cursor, so that the next page is filtered and sorted like the current one.
// Of the global flags only those selecting the account are repeated; the others
// include credentials, which must not be echoed.
func nextPageCommandLine(cmd *cobra.Command, args []string, next string) string {
	parts := []string{cmd.CommandPath()}
	for _, arg := range args {
		parts = append(parts, quoteShellArg(arg))
	}

	cmd.Flags().Visit(func(f *pflag.Flag) {
		switch f.Name {
		case "next", "prev":
			return
		case "profile", "hookdeck-config":
		default:
			if cmd.Root().PersistentFlags().Lookup(f.Name) != nil {
				return
			}
		}

		switch {
		case f.Value.Type() != "bool":
			parts = append(parts, "--"+f.Name, quoteShellArg(f.Value.String()))
		case f.Value.String() == "true":
			parts = append(parts, "--"+f.Name)
		default:
			parts = append(parts, "--"+f.Name+"="+f.Value.String())
		}
	})

	return strings.Join(append(parts, "--next", quoteShellArg(next)), " ")
}

var shellSafeArg = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

// quoteShellArg single-quotes a value that a POSIX shell would otherwise alter.
func quoteShellArg(value string) string {
	if shellSafeArg.MatchString(value) {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// marshalListResponseWithPagination marshals a list response including pagination
func marshalListResponseWithPagination(models interface{}, pagination hookdeck.PaginationResponse) ([]byte, error) {
	response := map[string]interface{}{
		"models":     models,
		"pagination": pagination,
	}
	return json.MarshalIndent(response, "", "  ")
}
