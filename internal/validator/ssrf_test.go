package validator

import (
	"net"
	"testing"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip       string
		expected bool
	}{
		{"127.0.0.1", true},
		{"127.0.0.2", true},
		{"10.0.0.5", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"169.254.169.254", true}, // AWS / GCP metadata
		{"::1", true},             // IPv6 loopback
		{"0.0.0.0", true},
		{"8.8.8.8", false},        // Public Google DNS
		{"1.1.1.1", false},        // Public Cloudflare DNS
		{"93.184.216.34", false},  // example.com
	}

	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		if ip == nil {
			t.Fatalf("failed to parse IP %s", tt.ip)
		}
		got := IsPrivateIP(ip)
		if got != tt.expected {
			t.Errorf("IsPrivateIP(%s) = %v, expected %v", tt.ip, got, tt.expected)
		}
	}
}

func TestValidateURL(t *testing.T) {
	// Forbidden URLs
	blocked := []string{
		"http://127.0.0.1:8080/admin",
		"http://localhost:3000/secret",
		"http://169.254.169.254/latest/meta-data/",
		"http://192.168.1.1/router",
		"http://10.200.0.1/internal",
		"file:///etc/passwd",
		"ftp://example.com/file",
	}

	for _, u := range blocked {
		_, err := ValidateURL(u)
		if err == nil {
			t.Errorf("expected URL %q to be blocked by SSRF validator", u)
		}
	}

	// Allowed public URL syntax
	validURL := "https://raw.githubusercontent.com/favicon.ico"
	parsed, err := ValidateURL(validURL)
	if err != nil {
		t.Errorf("expected public URL %q to be valid, got: %v", validURL, err)
	}
	if parsed == nil || parsed.Host != "raw.githubusercontent.com" {
		t.Errorf("unexpected parsed URL: %+v", parsed)
	}
}
