package version

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-github/v28/github"
	log "github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"

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

func needsToUpgrade(version, latest string) bool {
	if latest == "" {
		return false
	}

	// Normalize versions to include 'v' prefix for semver package
	currentVersion := version
	if !strings.HasPrefix(currentVersion, "v") {
		currentVersion = "v" + currentVersion
	}
	latestVersion := latest
	if !strings.HasPrefix(latestVersion, "v") {
		latestVersion = "v" + latestVersion
	}

	// Validate versions are valid semver
	if !semver.IsValid(currentVersion) || !semver.IsValid(latestVersion) {
		// Fallback to string comparison if not valid semver
		return strings.TrimPrefix(latest, "v") != strings.TrimPrefix(version, "v")
	}

	// Use semver.Compare which returns:
	// -1 if current < latest (upgrade needed)
	//  0 if current == latest (no upgrade)
	//  1 if current > latest (no upgrade, user is ahead)
	comparison := semver.Compare(currentVersion, latestVersion)

	// Upgrade needed if current version is less than latest
	// Special handling for pre-release versions:
	// - If on beta/RC and newer stable exists, suggest upgrade
	// - If on beta/RC and newer beta/RC exists, suggest upgrade
	// - If on stable and newer beta/RC exists, don't suggest upgrade
	if comparison < 0 {
		// Current is older than latest
		currentPrerelease := semver.Prerelease(currentVersion)
		latestPrerelease := semver.Prerelease(latestVersion)

		// If current is a prerelease, always suggest upgrade (to newer prerelease or stable)
		if currentPrerelease != "" {
			return true
		}

		// If current is stable and latest is prerelease, don't suggest upgrade
		if currentPrerelease == "" && latestPrerelease != "" {
			return false
		}

		// Both stable, or current stable and latest stable - suggest upgrade
		return true
	}

	return false
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
