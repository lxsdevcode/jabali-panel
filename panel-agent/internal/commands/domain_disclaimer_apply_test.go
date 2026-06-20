package commands

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestDisclaimerDomainRe keeps the strict DNS-name guard at this privileged
// trust boundary (input validation; ADR-0143 moved rendering to the hook).
func TestDisclaimerDomainRe(t *testing.T) {
	valid := []string{"example.com", "sub.domain.co.uk", "a-b.example.org", "x1.y2.z3.io"}
	for _, d := range valid {
		if !disclaimerDomainRe.MatchString(d) {
			t.Errorf("valid domain rejected: %q", d)
		}
	}
	bad := []string{
		``,
		`example`,     // no dot (FQDN required)
		`Example.com`, // uppercase
		`-bad.com`,    // leading dash
		`bad-.com`,    // trailing dash on label
		`a..b.com`,    // empty label
		"evil.com\n",  // newline
	}
	for _, d := range bad {
		if disclaimerDomainRe.MatchString(d) {
			t.Errorf("invalid domain accepted: %q", d)
		}
	}
}

func TestDisclaimerApply_RejectsBadParams(t *testing.T) {
	cases := []json.RawMessage{
		nil,
		json.RawMessage(`{}`),                       // missing domain_name
		json.RawMessage(`{"domain_name":"nope"}`),   // not an FQDN
		json.RawMessage(`{"domain_name":"BAD.COM"}`), // uppercase
	}
	for _, c := range cases {
		if _, err := domainDisclaimerApplyHandler(context.Background(), c); err == nil {
			t.Errorf("expected error for params %s", string(c))
		}
	}
}

func TestReadMailhookToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mailhook.token")
	if err := os.WriteFile(path, []byte("  sekret-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MAILHOOK_TOKEN_PATH", path)
	tok, err := readMailhookToken()
	if err != nil {
		t.Fatal(err)
	}
	if tok != "sekret-token" {
		t.Fatalf("token not trimmed: %q", tok)
	}

	// Empty file → error.
	if err := os.WriteFile(path, []byte("   \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readMailhookToken(); err == nil {
		t.Fatal("expected error for empty token file")
	}
}
