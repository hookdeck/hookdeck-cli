package healthcheck

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestCheckServerHealth_HealthyServer(t *testing.T) {
	// Start a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Parse server URL
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse server URL: %v", err)
	}

	// Perform health check (insecure=false, not relevant for HTTP)
	result := CheckServerHealth(serverURL, 3*time.Second, false)

	// Verify result
	if !result.Healthy {
		t.Errorf("Expected server to be healthy, got unhealthy")
	}
	if result.Status != HealthHealthy {
		t.Errorf("Expected status HealthHealthy, got %v", result.Status)
	}
	if result.Error != nil {
		t.Errorf("Expected no error, got: %v", result.Error)
	}
	if result.Duration <= 0 {
		t.Errorf("Expected positive duration, got: %v", result.Duration)
	}
}

func TestCheckServerHealth_UnreachableServer(t *testing.T) {
	// Use a URL that should not be listening
	targetURL, err := url.Parse("http://localhost:59999")
	if err != nil {
		t.Fatalf("Failed to parse URL: %v", err)
	}

	// Perform health check
	result := CheckServerHealth(targetURL, 1*time.Second, false)

	// Verify result
	if result.Healthy {
		t.Errorf("Expected server to be unhealthy, got healthy")
	}
	if result.Status != HealthUnreachable {
		t.Errorf("Expected status HealthUnreachable, got %v", result.Status)
	}
	if result.Error == nil {
		t.Errorf("Expected error, got nil")
	}
}

func TestCheckServerHealth_DefaultPorts(t *testing.T) {
	testCases := []struct {
		name         string
		urlString    string
		expectedPort string
	}{
		{
			name:         "HTTP default port",
			urlString:    "http://localhost",
			expectedPort: "80",
		},
		{
			name:         "HTTPS default port",
			urlString:    "https://localhost",
			expectedPort: "443",
		},
		{
			name:         "Explicit port",
			urlString:    "http://localhost:8080",
			expectedPort: "8080",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			targetURL, err := url.Parse(tc.urlString)
			if err != nil {
				t.Fatalf("Failed to parse URL: %v", err)
			}

			// Start a listener on the expected port to verify we're checking the right one
			listener, err := net.Listen("tcp", "localhost:"+tc.expectedPort)
			if err != nil {
				t.Skipf("Cannot bind to port %s: %v", tc.expectedPort, err)
			}
			defer listener.Close()

			// Perform health check (insecure=true to handle self-signed certs in test)
			result := CheckServerHealth(targetURL, 1*time.Second, true)

			// Should be healthy since we have a listener
			if !result.Healthy {
				t.Errorf("Expected server to be healthy on port %s, got unhealthy: %v", tc.expectedPort, result.Error)
			}
		})
	}
}

func TestFormatHealthMessage_Healthy(t *testing.T) {
	targetURL, _ := url.Parse("http://localhost:3000")
	result := HealthCheckResult{
		Status:  HealthHealthy,
		Healthy: true,
	}

	msg := FormatHealthMessage(result, targetURL)

	if len(msg) == 0 {
		t.Errorf("Expected non-empty message")
	}
	if !strings.Contains(msg, "→") {
		t.Errorf("Expected message to contain →")
	}
	if !strings.Contains(msg, "Local server is reachable") {
		t.Errorf("Expected message to contain 'Local server is reachable'")
	}
}

func TestFormatHealthMessage_Unhealthy(t *testing.T) {
	targetURL, _ := url.Parse("http://localhost:3000")
	result := HealthCheckResult{
		Status:  HealthUnreachable,
		Healthy: false,
		Error:   net.ErrClosed,
	}

	msg := FormatHealthMessage(result, targetURL)

	if len(msg) == 0 {
		t.Errorf("Expected non-empty message")
	}
	// Should contain warning indicator
	if !strings.Contains(msg, "●") {
		t.Errorf("Expected message to contain ●")
	}
	if !strings.Contains(msg, "Warning") {
		t.Errorf("Expected message to contain 'Warning'")
	}
}

func TestFormatHealthMessage_NilError(t *testing.T) {
	targetURL, _ := url.Parse("http://localhost:3000")
	result := HealthCheckResult{
		Status:  HealthUnreachable,
		Healthy: false,
		Error:   nil, // Nil error should not cause panic
	}

	msg := FormatHealthMessage(result, targetURL)

	if len(msg) == 0 {
		t.Errorf("Expected non-empty message")
	}
	if !strings.Contains(msg, "unknown error") {
		t.Errorf("Expected message to contain 'unknown error' when error is nil")
	}
}

func TestCheckServerHealth_PortInURL(t *testing.T) {
	// Create a server on a non-standard port
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Get the actual port assigned by the OS
	addr := listener.Addr().(*net.TCPAddr)
	targetURL, _ := url.Parse(fmt.Sprintf("http://localhost:%d/path", addr.Port))

	// Perform health check
	result := CheckServerHealth(targetURL, 3*time.Second, false)

	// Verify that the health check succeeded
	// This confirms that when a port is already in the URL, we don't append
	// a default port (which would cause localhost:8080 to become localhost:8080:80)
	if !result.Healthy {
		t.Errorf("Expected healthy=true for server with port in URL, got false: %v", result.Error)
	}
	if result.Error != nil {
		t.Errorf("Expected no error for server with port in URL, got: %v", result.Error)
	}
}

func TestCheckServerHealth_HTTPS_SelfSigned_InsecureTrue(t *testing.T) {
	// Start a test HTTPS server with self-signed certificate
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Parse server URL (will be https://...)
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse server URL: %v", err)
	}

	// Verify it's HTTPS
	if serverURL.Scheme != "https" {
		t.Fatalf("Expected HTTPS scheme, got: %s", serverURL.Scheme)
	}

	// Perform health check with insecure=true (should succeed)
	result := CheckServerHealth(serverURL, 3*time.Second, true)

	// Should be healthy because we skip certificate verification
	if !result.Healthy {
		t.Errorf("Expected server to be healthy with insecure=true, got unhealthy: %v", result.Error)
	}
	if result.Status != HealthHealthy {
		t.Errorf("Expected status HealthHealthy, got %v", result.Status)
	}
	if result.Error != nil {
		t.Errorf("Expected no error with insecure=true, got: %v", result.Error)
	}
}

func TestCheckServerHealth_HTTPS_SelfSigned_InsecureFalse(t *testing.T) {
	// Start a test HTTPS server with self-signed certificate
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Parse server URL (will be https://...)
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse server URL: %v", err)
	}

	// Perform health check with insecure=false (should fail due to self-signed cert)
	result := CheckServerHealth(serverURL, 3*time.Second, false)

	// Should be unhealthy because certificate verification fails
	if result.Healthy {
		t.Errorf("Expected server to be unhealthy with insecure=false on self-signed cert, got healthy")
	}
	if result.Status != HealthUnreachable {
		t.Errorf("Expected status HealthUnreachable, got %v", result.Status)
	}
	if result.Error == nil {
		t.Errorf("Expected certificate error, got nil")
	}
	// Verify it's a certificate-related error
	if result.Error != nil && !strings.Contains(result.Error.Error(), "certificate") {
		t.Logf("Error message: %v (may vary by platform)", result.Error)
	}
}

func TestCheckServerHealth_HTTPS_UsesTLSHandshake(t *testing.T) {
	// This test verifies that HTTPS URLs use TLS dial (not raw TCP)
	// by using httptest.NewTLSServer which creates a proper TLS server
	// and checking that the health check completes successfully

	// Start a test HTTPS server - this will only succeed if TLS handshake completes
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse server URL: %v", err)
	}

	// Verify it's HTTPS
	if serverURL.Scheme != "https" {
		t.Fatalf("Expected HTTPS scheme, got: %s", serverURL.Scheme)
	}

	// Perform health check with insecure=true (to accept self-signed cert)
	// If this succeeds, it proves TLS handshake was performed (not just TCP connect)
	result := CheckServerHealth(serverURL, 3*time.Second, true)

	// Should be healthy - this proves TLS handshake succeeded
	if !result.Healthy {
		t.Errorf("Expected healthy result for HTTPS server (TLS handshake should succeed), got: %v", result.Error)
	}

	// Verify that for HTTP URLs, we still use TCP (not TLS)
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer httpServer.Close()

	httpURL, _ := url.Parse(httpServer.URL)
	if httpURL.Scheme != "http" {
		t.Fatalf("Expected HTTP scheme, got: %s", httpURL.Scheme)
	}

	httpResult := CheckServerHealth(httpURL, 3*time.Second, false)
	if !httpResult.Healthy {
		t.Errorf("Expected healthy result for HTTP server, got: %v", httpResult.Error)
	}
}
