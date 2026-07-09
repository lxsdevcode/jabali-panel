package commands

import (
	"strings"
	"testing"
)

// JAB-68: the caller-facing nginx-test error must never carry raw `nginx -t`
// output (host filesystem paths, module/config internals). Only the fixed
// operation label is safe to surface; the raw output goes to the agent log.
func TestNginxTestFailure_NoRawLeak(t *testing.T) {
	raw := `nginx: [emerg] open() "/etc/nginx/sites-available/tenant-secret.conf" failed (2: No such file or directory)`
	err := nginxTestFailure("domain.vhost", raw)

	for _, leak := range []string{"/etc/nginx", "sites-available", "tenant-secret", "emerg", "open()"} {
		if strings.Contains(err.Message, leak) {
			t.Fatalf("raw nginx output leaked into Message (%q): %q", leak, err.Message)
		}
	}
	if !strings.Contains(err.Message, "domain.vhost") {
		t.Fatalf("expected the operation label in the message, got %q", err.Message)
	}
}
