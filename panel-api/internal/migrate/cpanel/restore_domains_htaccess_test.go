package cpanel

import (
	"os"
	"path/filepath"
	"testing"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/models"
)

func TestApplyMigratedHtaccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".htaccess")
	content := `RewriteEngine On
RewriteBase /
RewriteCond %{REQUEST_FILENAME} !-f
RewriteCond %{REQUEST_FILENAME} !-d
RewriteRule . /index.php [L]
Redirect 301 /old https://example.com/new
Header set X-Frame-Options SAMEORIGIN
Require valid-user`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	d := &models.Domain{Name: "example.com"}
	res := &DomainImportResult{}
	applyMigratedHtaccess(path, "example.com", d, res)

	// front controller skipped; redirect + header converted = 2 rules.
	if res.HtaccessRulesImported != 2 {
		t.Fatalf("expected 2 imported rules, got %d: %+v", res.HtaccessRulesImported, d.NginxRules)
	}
	if len(d.NginxRules) != 2 {
		t.Fatalf("domain should carry 2 NginxRules, got %d", len(d.NginxRules))
	}
	// Require valid-user is auth → a warning, not a rule.
	if len(res.HtaccessWarnings) == 0 {
		t.Errorf("expected a warning for Require valid-user")
	}
}

func TestApplyMigratedHtaccessMissingFileIsNoOp(t *testing.T) {
	d := &models.Domain{Name: "x.com"}
	res := &DomainImportResult{}
	applyMigratedHtaccess(filepath.Join(t.TempDir(), "nope", ".htaccess"), "x.com", d, res)
	if len(d.NginxRules) != 0 || res.HtaccessRulesImported != 0 || len(res.HtaccessWarnings) != 0 {
		t.Errorf("missing .htaccess must be a silent no-op, got rules=%v warns=%v", d.NginxRules, res.HtaccessWarnings)
	}
}

func TestApplyMigratedHtaccessRuleCap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".htaccess")
	var b []byte
	for i := 0; i < 520; i++ {
		b = append(b, []byte("Header set X-N-"+itoa(i)+" v\n")...)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	d := &models.Domain{Name: "big.com"}
	res := &DomainImportResult{}
	applyMigratedHtaccess(path, "big.com", d, res)
	if len(d.NginxRules) != 500 {
		t.Errorf("rules must be capped at 500, got %d", len(d.NginxRules))
	}
	foundCap := false
	for _, w := range res.HtaccessWarnings {
		if len(w) > 0 && containsSub(w, "only the first 500") {
			foundCap = true
		}
	}
	if !foundCap {
		t.Errorf("expected a cap warning, got %v", res.HtaccessWarnings)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

func containsSub(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
