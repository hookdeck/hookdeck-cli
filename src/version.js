const semver = require('semver');

function shouldPromptUpgrade(currentVersion, latestVersion) {
  // Remove 'v' prefix if present
  const cleanCurrent = currentVersion.replace(/^v/, '');
  const cleanLatest = latestVersion.replace(/^v/, '');
  
  // Parse versions
  const current = semver.parse(cleanCurrent);
  const latest = semver.parse(cleanLatest);
  
  if (!current || !latest) {
    return false;
  }
  
  // If current is a prerelease (beta), only prompt for:
  // 1. Newer prerelease of same version (1.10.0-beta.4 -> 1.10.0-beta.5)
  // 2. Stable release of same or newer version (1.10.0-beta.4 -> 1.10.0)
  if (current.prerelease.length > 0) {
    // Current is beta
    if (latest.prerelease.length > 0) {
      // Latest is also beta - only upgrade if it's newer
      return semver.gt(cleanLatest, cleanCurrent);
    } else {
      // Latest is stable - upgrade if same version or newer
      return semver.gte(cleanLatest, `${current.major}.${current.minor}.${current.patch}`);
    }
  }
  
  // If current is stable, only prompt for newer stable releases
  // Don't prompt to upgrade to beta versions
  if (latest.prerelease.length > 0) {
    return false;
  }
  
  // Both are stable - normal comparison
  return semver.gt(cleanLatest, cleanCurrent);
}

module.exports = { shouldPromptUpgrade };