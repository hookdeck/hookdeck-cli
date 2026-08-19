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
// qualified name ("gateway_events") and the bare resource ("events") resolve.
// suffix is appended to every topic that resolves — products use it to repeat
// shared documentation such as the JSON response shape.
func HelpTopic(prefix string, topics map[string]string, topic, suffix string) *mcpsdk.CallToolResult {
	// An exact tool name always wins. Platform tools (hookdeck_login,
	// hookdeck_projects) do not carry the product prefix, so prepending it
	// unconditionally would turn a valid topic into a miss.
	//
	// A bare resource ("events", "projects") is qualified by trying the
	// product prefix first and the platform prefix second — otherwise "projects"
	// would resolve on a server whose product prefix happens to be "hookdeck"
	// and fail on every other one.
	text, ok := topics[topic]
	if !ok {
		for _, p := range []string{prefix, DefaultPlatformPrefix + "_"} {
			if p == "" || strings.HasPrefix(topic, p) {
				continue
			}
			if text, ok = topics[p+topic]; ok {
				topic = p + topic
				break
			}
		}
	}
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
