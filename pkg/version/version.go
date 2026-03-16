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

// parsedVersion holds the numeric components and optional pre-release string
// for a semver-style version (major.minor.patch[-prerelease]).
type parsedVersion struct {
	major      int
	minor      int
	patch      int
	prerelease string // empty string means GA release
}

// parseVersion strips the optional "v" prefix and parses "major.minor.patch"
// or "major.minor.patch-prerelease". Returns ok=false for unrecognised formats.
func parseVersion(v string) (parsedVersion, bool) {
	v = strings.TrimPrefix(v, "v")

	// Split off pre-release tag (everything after the first "-")
	prerelease := ""
	if idx := strings.Index(v, "-"); idx >= 0 {
		prerelease = v[idx+1:]
		v = v[:idx]
	}

	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return parsedVersion{}, false
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return parsedVersion{}, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return parsedVersion{}, false
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return parsedVersion{}, false
	}

	return parsedVersion{major, minor, patch, prerelease}, true
}

// compareVersions returns -1, 0, or 1 depending on whether a is less than,
// equal to, or greater than b, using numeric major/minor/patch comparison.
// Pre-release versions are considered lower than the corresponding GA release
// (semver §9: "1.0.0-alpha < 1.0.0").
func compareVersions(a, b parsedVersion) int {
	for _, pair := range [][2]int{
		{a.major, b.major},
		{a.minor, b.minor},
		{a.patch, b.patch},
	} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}

	// Same major.minor.patch — compare pre-release:
	// GA (empty prerelease) > any pre-release string
	switch {
	case a.prerelease == "" && b.prerelease == "":
		return 0
	case a.prerelease == "" && b.prerelease != "":
		return 1 // GA is higher than pre-release
	case a.prerelease != "" && b.prerelease == "":
		return -1
	default:
		// Both have pre-release: lexicographic comparison
		if a.prerelease < b.prerelease {
			return -1
		}
		if a.prerelease > b.prerelease {
			return 1
		}
		return 0
	}
}

// needsToUpgrade returns true if latest is a newer version than version and
// the upgrade makes sense based on pre-release promotion rules:
//   - GA → newer GA: prompt
//   - GA → beta: do not prompt (stable users should not be pushed to pre-releases)
//   - beta → newer beta: prompt
//   - beta → newer GA: prompt
func needsToUpgrade(version, latest string) bool {
	if latest == "" {
		return false
	}

	current, currentOK := parseVersion(version)
	latestParsed, latestOK := parseVersion(latest)

	if !currentOK || !latestOK {
		// Fall back to simple string comparison for unrecognised version formats
		return strings.TrimPrefix(latest, "v") != strings.TrimPrefix(version, "v")
	}

	// Only prompt if latest is strictly newer
	if compareVersions(latestParsed, current) <= 0 {
		return false
	}

	// Do not prompt when upgrading from GA to a pre-release version
	if current.prerelease == "" && latestParsed.prerelease != "" {
		return false
	}

	return true
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
