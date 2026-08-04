package listen

import (
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func TestResolveEffectiveProjectID(t *testing.T) {
	connections := []*hookdeck.Connection{
		{TeamID: "tm_from_connection"},
	}

	t.Run("prefers the profile's active project id", func(t *testing.T) {
		if got := resolveEffectiveProjectID("tm_profile", connections); got != "tm_profile" {
			t.Errorf("got %q, want tm_profile", got)
		}
	})

	t.Run("falls back to the connections' team id", func(t *testing.T) {
		if got := resolveEffectiveProjectID("", connections); got != "tm_from_connection" {
			t.Errorf("got %q, want tm_from_connection", got)
		}
	})

	t.Run("skips connections without a team id", func(t *testing.T) {
		mixed := []*hookdeck.Connection{nil, {TeamID: ""}, {TeamID: "tm_second"}}
		if got := resolveEffectiveProjectID("", mixed); got != "tm_second" {
			t.Errorf("got %q, want tm_second", got)
		}
	})

	t.Run("empty when nothing is known", func(t *testing.T) {
		if got := resolveEffectiveProjectID("", nil); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}
