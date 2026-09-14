// Package summary builds the one-line "Listening on …" banner for listen
// output. Like pkg/listen/links it is a leaf shared by the interactive TUI and
// the compact/quiet printer, so the two output modes cannot drift apart.
package summary

import "fmt"

// Listening returns the session summary line, e.g.
// "Listening on 1 source • 2 connections".
//
// The compact printer used to print the bare preposition "Listening on" and
// nothing else, because it never had the counts the TUI header carried (#402).
// A dangling preposition is the line most CI logs carry, so the counts live
// here where both renderers reach them.
func Listening(numSources, numConnections int) string {
	return fmt.Sprintf("Listening on %s • %s",
		pluralize(numSources, "source"),
		pluralize(numConnections, "connection"),
	)
}

// pluralize renders a count with its noun, adding a plural "s" for anything
// other than exactly one.
func pluralize(count int, noun string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, noun)
	}
	return fmt.Sprintf("%d %ss", count, noun)
}
