package mcpcore

import (
	"context"
	"time"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/project"
)

// projectDisplayLookupTimeout bounds the whole display-name lookup. The lookup
// runs before every tool call until it succeeds, including calls that never
// reach the API (help, a write refused in read-only mode, a rejected argument),
// and the API client has no deadline of its own. Without this bound one slow
// API response held the tool call open for as long as the API took: the client
// saw initialize answered and then nothing. The name is cosmetic, so giving up
// costs only an id where a name would have been, and the next call tries again.
// A variable so tests can shorten it.
var projectDisplayLookupTimeout = 5 * time.Second

// FillProjectDisplayNameIfNeeded sets target.ProjectOrg and target.ProjectName
// when target has an API key and project id but no cached org/name (typical
// after loading the profile from disk). Fails silently on API errors. Stdio MCP
// invokes tools sequentially, so this is safe without locking.
//
// The lookup gives up after projectDisplayLookupTimeout, or sooner when ctx is
// done, so it can never hold up the tool call it runs ahead of.
//
// lookup is the client account-level requests are made from, which is not always
// target: a product API served from its own host does not answer them, so the
// lookup has to go to the account API.
//
// Resolution order matters. Validating the key is tried first because it works
// for every credential and returns the name of the key's own project. Listing
// projects only works for a user-associated key, so a project-scoped key from
// `hookdeck ci` — a common way to configure an MCP server — would otherwise
// leave the name empty and callers with nothing but an opaque id to show.
func FillProjectDisplayNameIfNeeded(ctx context.Context, lookup, target *hookdeck.Client) {
	if lookup == nil || target == nil || target.APIKey == "" || target.ProjectID == "" {
		return
	}
	if target.ProjectName != "" || target.ProjectOrg != "" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, projectDisplayLookupTimeout)
	defer cancel()

	// The key's own project, available whatever kind of key it is.
	if response, err := lookup.ValidateAPIKeyContext(ctx); err == nil && response.ProjectID == target.ProjectID {
		target.ProjectName = response.ProjectName
		target.ProjectOrg = response.OrganizationName
		if target.ProjectName != "" || target.ProjectOrg != "" {
			return
		}
	}

	// Out of time: the listing would fail too.
	if ctx.Err() != nil {
		return
	}

	// The active project differs from the key's, so it has to be looked up.
	// Only a user-associated key can do this.
	projects, err := lookup.ListProjectsContext(ctx)
	if err != nil {
		return
	}
	items := project.NormalizeProjects(projects, target.ProjectID)
	for i := range items {
		if items[i].Id != target.ProjectID {
			continue
		}
		target.ProjectOrg = items[i].Org
		target.ProjectName = items[i].Project
		return
	}
}
