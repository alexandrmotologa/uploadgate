package validator

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

var (
	// Blocked CIDR ranges for IPv4 and IPv6
	privateCIDRs []*net.IPNet
)

func init() {
	cidrs := []string{
		// IPv4 Loopback & Private
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16", // Link-local & Cloud Metadata (169.254.169.254)
		"0.0.0.0/8",
		"100.64.0.0/10", // Carrier-grade NAT
		"192.0.0.0/24",
		"192.0.2.0/24",
		"198.18.0.0/15",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"224.0.0.0/4",   // Multicast
		"240.0.0.0/4",   // Reserved

		// IPv6 Loopback & Private
		"::1/128",
		"fc00::/7",
		"fe80::/10",
		"ff00::/8",
	}

	for _, c := range cidrs {
		_, ipNet, err := net.ParseCIDR(c)
		if err == nil {
			privateCIDRs = append(privateCIDRs, ipNet)
		}
	}
}

// IsPrivateIP checks if an IP belongs to private, loopback, or metadata ranges.
func IsPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}

	// Unwrap IPv4-mapped IPv6
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}

	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}

	for _, cidr := range privateCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}

	return false
}

// ValidateURL checks if a target URL is safe for ingestion and does not resolve to private addresses.
func ValidateURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url syntax: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("unsupported url scheme %q: only http and https are permitted", scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return nil, errors.New("missing url hostname")
	}

	lowerHost := strings.ToLower(host)
	if lowerHost == "localhost" ||
		strings.HasSuffix(lowerHost, ".localhost") ||
		strings.HasSuffix(lowerHost, ".local") ||
		strings.HasSuffix(lowerHost, ".internal") ||
		strings.HasSuffix(lowerHost, ".arpa") {
		return nil, fmt.Errorf("forbidden internal hostname: %s", host)
	}

	// If hostname is directly an IP
	if ip := net.ParseIP(host); ip != nil {
		if IsPrivateIP(ip) {
			return nil, fmt.Errorf("forbidden private ip address: %s", host)
		}
		return parsed, nil
	}

	// Resolve hostname via DNS
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("dns resolution failed for %q: %w", host, err)
	}

	for _, ip := range ips {
		if IsPrivateIP(ip) {
			return nil, fmt.Errorf("dns resolved to forbidden private ip %s for host %s", ip.String(), host)
		}
	}

	return parsed, nil
}

// NewSafeHTTPClient creates an HTTP client protected against DNS rebinding and private network calls.
func NewSafeHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip != nil && IsPrivateIP(ip) {
				return fmt.Errorf("ssrf blocked: connection to private address %s rejected", host)
			}
			return nil
		},
	}

	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		ResponseHeaderTimeout: timeout,
		DisableKeepAlives:     true,
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("stopped after 5 redirects")
			}
			if _, err := ValidateURL(req.URL.String()); err != nil {
				return fmt.Errorf("redirect blocked by ssrf policy: %w", err)
			}
			return nil
		},
	}
}
