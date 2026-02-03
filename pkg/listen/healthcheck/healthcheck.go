package healthcheck

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"os"
	"time"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
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

// CheckServerHealth performs a connection check to verify a server is listening.
// For HTTPS URLs, it performs a TLS handshake to avoid incomplete handshake warnings
// on the server side. The insecure parameter controls whether to skip TLS certificate
// verification (matching the --insecure flag behavior for webhook forwarding).
// The timeout parameter should be appropriate for the deployment context:
// - Local development: 3s is typically sufficient
// - Production/edge: May require longer timeouts due to network conditions
func CheckServerHealth(targetURL *url.URL, timeout time.Duration, insecure bool) HealthCheckResult {
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

	var conn net.Conn
	var err error

	if targetURL.Scheme == "https" {
		// Use TLS connection for HTTPS endpoints to complete handshake properly
		// and avoid TLS handshake warnings on the server
		dialer := &net.Dialer{Timeout: timeout}
		tlsConfig := &tls.Config{
			InsecureSkipVerify: insecure,
			ServerName:         host,
		}
		conn, err = tls.DialWithDialer(dialer, "tcp", address, tlsConfig)
	} else {
		// Use plain TCP for HTTP endpoints
		conn, err = net.DialTimeout("tcp", address, timeout)
	}

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
		return fmt.Sprintf("→ Local server is reachable at %s", targetURL.String())
	}

	color := ansi.Color(os.Stdout)
	errorMessage := "unknown error"
	if result.Error != nil {
		errorMessage = result.Error.Error()
	}
	return fmt.Sprintf("%s Cannot connect to local server at %s\n  %s\n  The server may not be running. Events will fail until the server starts.",
		color.Yellow("● Warning:"),
		targetURL.String(),
		errorMessage)
}
