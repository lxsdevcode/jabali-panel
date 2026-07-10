package cpanel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sslRecorder records ssl.install_custom calls.
type sslRecorder struct {
	calls []map[string]any
}

func (a *sslRecorder) Call(_ context.Context, command string, params any) (json.RawMessage, error) {
	if command == "ssl.install_custom" {
		if m, ok := params.(map[string]any); ok {
			a.calls = append(a.calls, m)
		}
	}
	return json.RawMessage(`{}`), nil
}

// GH #327: HestiaCP stores installed certs flat under hestia/ssl/<domain>.{crt,key,ca}.
// scanHestiaSSL must push cert(+chain)+key per web domain that has them, append the
// CA chain, and skip domains with no cert.
func TestScanHestiaSSL(t *testing.T) {
	extract := t.TempDir()
	sslDir := filepath.Join(extract, "hestia", "ssl")
	if err := os.MkdirAll(sslDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// site.example has crt+key+ca; noncert.example has nothing.
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(sslDir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("site.example.crt", "LEAFCERT\n")
	write("site.example.key", "PRIVKEY\n")
	write("site.example.ca", "CHAINCERT\n")

	parsed := &ParsedTarball{ExtractDir: extract, DomainNames: []string{"site.example", "noncert.example"}}
	rec := &sslRecorder{}
	res := &SSLResult{}
	scanHestiaSSL(context.Background(), rec, parsed, res)

	if res.Installed != 1 {
		t.Fatalf("Installed = %d, want 1 (only site.example has a cert)", res.Installed)
	}
	if len(rec.calls) != 1 {
		t.Fatalf("install_custom calls = %d, want 1", len(rec.calls))
	}
	c := rec.calls[0]
	if c["domain"] != "site.example" {
		t.Errorf("domain = %v, want site.example", c["domain"])
	}
	cert, _ := c["cert_pem"].(string)
	if !strings.Contains(cert, "LEAFCERT") || !strings.Contains(cert, "CHAINCERT") {
		t.Errorf("cert_pem missing leaf or chain: %q", cert)
	}
	if k, _ := c["key_pem"].(string); k != "PRIVKEY" {
		t.Errorf("key_pem = %q, want PRIVKEY", k)
	}
}

// No hestia/ssl dir → no-op, no calls, no error.
func TestScanHestiaSSL_NoDir(t *testing.T) {
	parsed := &ParsedTarball{ExtractDir: t.TempDir(), DomainNames: []string{"x.example"}}
	rec := &sslRecorder{}
	res := &SSLResult{}
	scanHestiaSSL(context.Background(), rec, parsed, res)
	if res.Installed != 0 || len(rec.calls) != 0 {
		t.Errorf("expected no-op, got Installed=%d calls=%d", res.Installed, len(rec.calls))
	}
}
