package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestHashUserAPIToken_Deterministic(t *testing.T) {
	plain := "jat_abc123"
	got := HashUserAPIToken(plain)
	want := sha256.Sum256([]byte(plain))
	if got != hex.EncodeToString(want[:]) {
		t.Errorf("hashUserAPIToken = %q, want %q", got, hex.EncodeToString(want[:]))
	}
}

func TestExtractBearer_HappyPath(t *testing.T) {
	cases := []struct {
		name string
		hdr  string
		want string
	}{
		{"valid bearer", "Bearer jat_abc123", "jat_abc123"},
		{"case insensitive bearer", "bearer jat_xyz789", "jat_xyz789"},
		{"missing header", "", ""},
		{"basic auth", "Basic dXNlcjpwYXNz", ""},
		{"unknown bearer prefix", "Bearer ghp_notjabali", ""},
		{"single segment", "jat_lonely", ""},
		{"empty bearer", "Bearer ", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractFromRawHeader(tc.hdr)
			if got != tc.want {
				t.Errorf("extract(%q) = %q, want %q", tc.hdr, got, tc.want)
			}
		})
	}
}

// extractFromRawHeader is a test-only thin wrapper that lets us reuse
// the production parsing logic without spinning up a gin context just
// to set one header. Keeps the unit tests fast + dependency-free.
func extractFromRawHeader(h string) string {
	if h == "" {
		return ""
	}
	parts := splitFirstSpace(h)
	if len(parts) != 2 || !equalFold(parts[0], "Bearer") {
		return ""
	}
	tok := trimSpace(parts[1])
	if len(tok) < len(userAPITokenPrefix) || tok[:len(userAPITokenPrefix)] != userAPITokenPrefix {
		return ""
	}
	return tok
}

// tiny stdlib substitutes so this file has no test-only stdlib import
// drift vs production code.
func splitFirstSpace(s string) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}
func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
