package version

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/go-github/v28/github"
	log "github.com/sirupsen/logrus"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
)

// Version of the CLI.
// This is set to the actual version by GoReleaser, identify by the
// git tag assigned to the release. Versions built from source will
// always show main.
var Version = "main"

// Template for the version string.
var Template = fmt.Sprintf("hookdeck version %s\n", Version)

// CheckLatestVersion makes a request to the GitHub API to pull the latest
// release of the CLI
func CheckLatestVersion() {
	// main is the dev version, we don't want to check against that every time
	if Version != "main" {
		s := ansi.StartNewSpinner("Checking for new versions...", os.Stdout)
		latest := getLatestVersion()

		ansi.StopSpinner(s, "", os.Stdout)

		if needsToUpgrade(Version, latest) {
			fmt.Println(ansi.Italic("A newer version of the Hookdeck CLI is available, please update to:"), ansi.Italic(latest))
		}
	}
}

func needsToUpgrade(current, latest string) bool {
	if latest == "" {
		return false
	}

	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")

	currentIsPreRelease := strings.Contains(current, "-")
	latestIsPreRelease := strings.Contains(latest, "-")

	// Don't suggest upgrading from a GA release to a pre-release version.
	if !currentIsPreRelease && latestIsPreRelease {
		return false
	}

	return semverGreater(latest, current)
}

// semverGreater returns true if a is semantically greater than b.
// Both a and b must not have a "v" prefix.
func semverGreater(a, b string) bool {
	aNums, aPre := parseVersion(a)
	bNums, bPre := parseVersion(b)

	maxLen := len(aNums)
	if len(bNums) > maxLen {
		maxLen = len(bNums)
	}
	for i := 0; i < maxLen; i++ {
		var av, bv int
		if i < len(aNums) {
			av = aNums[i]
		}
		if i < len(bNums) {
			bv = bNums[i]
		}
		if av != bv {
			return av > bv
		}
	}

	// Same base version: GA (no pre-release) beats any pre-release.
	if aPre == "" && bPre != "" {
		return true
	}
	if aPre != "" && bPre == "" {
		return false
	}

	return comparePreRelease(aPre, bPre) > 0
}

// parseVersion splits a version string (without "v" prefix) into its
// numeric components and an optional pre-release identifier.
func parseVersion(v string) (nums []int, pre string) {
	parts := strings.SplitN(v, "-", 2)
	if len(parts) > 1 {
		pre = parts[1]
	}
	for _, s := range strings.Split(parts[0], ".") {
		n, _ := strconv.Atoi(s)
		nums = append(nums, n)
	}
	return
}

// comparePreRelease compares two pre-release strings dot-by-dot.
// Returns a positive value if a > b, zero if equal, negative if a < b.
func comparePreRelease(a, b string) int {
	if a == b {
		return 0
	}
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}

	for i := 0; i < maxLen; i++ {
		if i >= len(aParts) {
			return -1
		}
		if i >= len(bParts) {
			return 1
		}
		aN, aErr := strconv.Atoi(aParts[i])
		bN, bErr := strconv.Atoi(bParts[i])
		if aErr == nil && bErr == nil {
			if aN != bN {
				if aN > bN {
					return 1
				}
				return -1
			}
		} else {
			if aParts[i] > bParts[i] {
				return 1
			}
			if aParts[i] < bParts[i] {
				return -1
			}
		}
	}
	return 0
}

func getLatestVersion() string {
	client := github.NewClient(nil)
	rep, _, err := client.Repositories.GetLatestRelease(context.Background(), "hookdeck", "hookdeck-cli")

	l := log.StandardLogger()

	if err != nil {
		// We don't want to fail any functionality or display errors for this
		// so fail silently and output to debug log
		l.Debug(err)
		return ""
	}

	return *rep.TagName
}
