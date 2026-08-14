package mcpcore

import (
	"fmt"
	"sort"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// HelpTopic resolves a help topic against a product's topic map and returns the
// help text, or an error result listing the available topics.
//
// prefix is the server's tool-name prefix (e.g. "hookdeck_"), so both the
// qualified name ("hookdeck_events") and the bare resource ("events") resolve.
// suffix is appended to every topic that resolves — products use it to repeat
// shared documentation such as the JSON response shape.
func HelpTopic(prefix string, topics map[string]string, topic, suffix string) *mcpsdk.CallToolResult {
	if prefix != "" && !strings.HasPrefix(topic, prefix) {
		topic = prefix + topic
	}
	text, ok := topics[topic]
	if ok {
		if suffix != "" {
			return TextResult(text + "\n\n" + suffix)
		}
		return TextResult(text)
	}

	// If the topic doesn't match a tool name exactly, it may be a natural
	// language question. List all available tools so the caller can pick.
	var names []string
	for k := range topics {
		names = append(names, k)
	}
	sort.Strings(names)
	return ErrorResult(fmt.Sprintf(
		"No help found for %q. The topic parameter expects a tool name, not a question.\n\nAvailable tools: %s\n\nOmit the topic parameter for a general overview.",
		topic, strings.Join(names, ", "),
	))
}
