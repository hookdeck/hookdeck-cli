package healthcheck

import (
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

	// Perform health check
	result := CheckServerHealth(serverURL, 3*time.Second)

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
	result := CheckServerHealth(targetURL, 1*time.Second)

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

			// Perform health check
			result := CheckServerHealth(targetURL, 1*time.Second)

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
	if !strings.Contains(msg, "✓") {
		t.Errorf("Expected message to contain ✓")
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
	if !strings.Contains(msg, "⚠") {
		t.Errorf("Expected message to contain ⚠")
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
