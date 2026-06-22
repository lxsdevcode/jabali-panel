package main

import (
	"context"
	"strings"
	"testing"
)

// runCronHTTPTrigger must reject non-routable targets, bad schemes, and
// non-web ports BEFORE issuing any request. These paths don't hit the
// network (scheme/port fail pre-resolve; "localhost" resolves via /etc/hosts
// to loopback, which the SSRF guard blocks).
func TestRunCronHTTPTrigger_Rejects(t *testing.T) {
	cases := []struct {
		desc      string
		url       string
		wantInErr string
	}{
		{"bad scheme", "ftp://example.com/x", "scheme must be"},
		{"non-web port", "http://localhost:8080/x", "ports 80/443"},
		{"loopback host (SSRF)", "http://localhost/x", "non-routable"},
		{"empty host", "http:///x", "no host"},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			err := runCronHTTPTrigger(context.Background(), tc.url)
			if err == nil {
				t.Fatalf("expected error for %q, got nil", tc.url)
			}
			if !strings.Contains(err.Error(), tc.wantInErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantInErr)
			}
		})
	}
}
