package api

import (
	"strings"
	"testing"
)

// JAB-72: nginx rewrite patterns are rendered verbatim into the vhost and run
// under PCRE (backtracking). Validate length, reject nested-quantifier ReDoS
// shapes, and reject grossly invalid regex — with a stricter cap for tenants.
func TestValidateRewritePattern(t *testing.T) {
	redos := []string{"(a+)+", "(.*)*", `(\d+)*`, "(ab+)+", "(x*)* "}
	for _, p := range redos {
		if err := validateRewritePattern(p, 256); err == nil {
			t.Errorf("expected ReDoS pattern %q to be rejected", p)
		}
	}

	if err := validateRewritePattern("([unclosed", 256); err == nil {
		t.Errorf("expected invalid regex to be rejected")
	}

	ok := []string{"^/old/(.*)$", `^/foo/(\d+)/bar$`, "^/a/b/c$", "index\\.php"}
	for _, p := range ok {
		if err := validateRewritePattern(p, 256); err != nil {
			t.Errorf("expected %q to be allowed, got %v", p, err)
		}
	}

	// Length cap: a pattern between the tenant (128) and admin (256) caps is OK
	// for admin but rejected for a tenant.
	mid := "^/" + strings.Repeat("a", 200) + "$" // ~203 chars
	if err := validateRewritePattern(mid, 256); err != nil {
		t.Errorf("203-char pattern should pass admin cap, got %v", err)
	}
	if err := validateRewritePattern(mid, 128); err == nil {
		t.Errorf("203-char pattern should fail the tenant cap")
	}
}
