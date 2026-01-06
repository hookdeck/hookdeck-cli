package listen

import (
	"fmt"
	"net"
	"net/url"
	"time"
)

// ServerHealthStatus represents the health status of the target server
type ServerHealthStatus int

const (
	HealthHealthy     ServerHealthStatus = iota // TCP connection successful
	HealthUnreachable                           // Connection refused or timeout
)

// HealthCheckResult contains the result of a health check
type HealthCheckResult struct {
	Status    ServerHealthStatus
	Healthy   bool
	Error     error
	Timestamp time.Time
	Duration  time.Duration
}

// CheckServerHealth performs a TCP connection check to the target URL
func CheckServerHealth(targetURL *url.URL, timeout time.Duration) HealthCheckResult {
	start := time.Now()

	host := targetURL.Hostname()
	port := targetURL.Port()

	// Default ports if not specified
	if port == "" {
		if targetURL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	address := net.JoinHostPort(host, port)

	conn, err := net.DialTimeout("tcp", address, timeout)
	duration := time.Since(start)

	result := HealthCheckResult{
		Timestamp: start,
		Duration:  duration,
	}

	if err != nil {
		result.Healthy = false
		result.Error = err
		result.Status = HealthUnreachable
		return result
	}

	// Successfully connected - server is healthy
	conn.Close()
	result.Healthy = true
	result.Status = HealthHealthy
	return result
}

// FormatHealthMessage creates a user-friendly health status message
func FormatHealthMessage(result HealthCheckResult, targetURL *url.URL) string {
	if result.Healthy {
		return fmt.Sprintf("✓ Local server is reachable at %s", targetURL.String())
	}

	return fmt.Sprintf("⚠ Warning: Cannot connect to local server at %s\n  %s\n  The server may not be running. Webhooks will fail until the server starts.",
		targetURL.String(),
		result.Error.Error())
}
