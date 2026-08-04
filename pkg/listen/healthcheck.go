package listen

import (
	"net/url"
	"time"

	"github.com/hookdeck/hookdeck-cli/pkg/listen/healthcheck"
)

// Re-export types and constants from healthcheck subpackage for convenience
type ServerHealthStatus = healthcheck.ServerHealthStatus
type HealthCheckResult = healthcheck.HealthCheckResult

const (
	HealthHealthy     = healthcheck.HealthHealthy
	HealthUnreachable = healthcheck.HealthUnreachable
)

// CheckServerHealth performs a connection check to the target URL
// For HTTPS URLs, it performs a TLS handshake with optional certificate verification skip.
// This is a wrapper around the healthcheck package function for backward compatibility
func CheckServerHealth(targetURL *url.URL, timeout time.Duration, insecure bool) HealthCheckResult {
	return healthcheck.CheckServerHealth(targetURL, timeout, insecure)
}

// FormatHealthMessage creates a user-friendly health status message
// This is a wrapper around the healthcheck package function for backward compatibility
func FormatHealthMessage(result HealthCheckResult, targetURL *url.URL) string {
	return healthcheck.FormatHealthMessage(result, targetURL)
}
