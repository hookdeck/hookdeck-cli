package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// printPaginationInfo displays pagination cursors for text output
func printPaginationInfo(pagination hookdeck.PaginationResponse, commandExample string) {
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
		fmt.Printf("  %s --next %s\n", commandExample, *pagination.Next)
	}
}

// marshalListResponseWithPagination marshals a list response including pagination
func marshalListResponseWithPagination(models interface{}, pagination hookdeck.PaginationResponse) ([]byte, error) {
	response := map[string]interface{}{
		"models":     models,
		"pagination": pagination,
	}
	return json.MarshalIndent(response, "", "  ")
}
