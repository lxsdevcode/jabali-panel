package htaccess

import (
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

func boolp(b bool) *bool { return &b }

// findRule returns the first rule of the given type, or false.
func firstRule(r Result, typ string) (models.NginxRule, bool) {
	for _, x := range r.Rules {
		if x.Type == typ {
			return x, true
		}
	}
	return models.NginxRule{}, false
}

func hasSecurityWarning(r Result) bool {
	for _, w := range r.Warnings {
		if w.Security {
			return true
		}
	}
	return false
}

func TestFrontControllerSkipped(t *testing.T) {
	in := `RewriteEngine On
RewriteBase /
RewriteCond %{REQUEST_FILENAME} !-f
RewriteCond %{REQUEST_FILENAME} !-d
RewriteRule . /index.php [L]`
	r := Convert(in, "/")
	if len(r.Rules) != 0 {
		t.Fatalf("front-controller should emit no rules, got %d: %+v", len(r.Rules), r.Rules)
	}
	if len(r.Notes) == 0 {
		t.Fatalf("front-controller should add an informational note")
	}
	if len(r.Warnings) != 0 {
		t.Fatalf("front-controller should not warn, got %+v", r.Warnings)
	}
}

func TestWordPressCanonicalIsNoOp(t *testing.T) {
	// Stock WordPress .htaccess.
	in := `# BEGIN WordPress
<IfModule mod_rewrite.c>
RewriteEngine On
RewriteBase /
RewriteRule ^index\.php$ - [L]
RewriteCond %{REQUEST_FILENAME} !-f
RewriteCond %{REQUEST_FILENAME} !-d
RewriteRule . /index.php [L]
</IfModule>
# END WordPress`
	r := Convert(in, "/")
	// The only rewrites are the index.php passthrough and the front
	// controller; neither should produce a typed rule, and nothing security
	// relevant should be dropped.
	if hasSecurityWarning(r) {
		t.Fatalf("stock WordPress should not produce security warnings: %+v", r.Warnings)
	}
	for _, ru := range r.Rules {
		if ru.Type == "rewrite" && ru.Replacement == "/index.php" {
			t.Fatalf("front controller leaked into a rule: %+v", ru)
		}
	}
}

func TestRedirect301PrefixAppend(t *testing.T) {
	r := Convert(`Redirect 301 /old-page https://example.com/new-page`, "/")
	ru, ok := firstRule(r, "rewrite")
	if !ok {
		t.Fatalf("expected a rewrite rule, got %+v", r)
	}
	if ru.Flag != "permanent" {
		t.Errorf("301 should map to permanent, got %q", ru.Flag)
	}
	if ru.Pattern != `^/old-page(.*)$` {
		t.Errorf("unexpected pattern %q", ru.Pattern)
	}
	if ru.Replacement != "https://example.com/new-page$1" {
		t.Errorf("unexpected replacement %q", ru.Replacement)
	}
}

func TestRedirectMatchRegex(t *testing.T) {
	r := Convert(`RedirectMatch 301 ^/blog/(.*)$ https://new.example.com/$1`, "/")
	ru, ok := firstRule(r, "rewrite")
	if !ok {
		t.Fatalf("expected rewrite, got %+v", r)
	}
	if ru.Pattern != `^/blog/(.*)$` || ru.Replacement != `https://new.example.com/$1` || ru.Flag != "permanent" {
		t.Errorf("unexpected RedirectMatch mapping: %+v", ru)
	}
}

func TestRewriteRuleExternalRedirect(t *testing.T) {
	r := Convert(`RewriteRule ^promo$ https://example.com/sale [R=301,L]`, "/")
	ru, ok := firstRule(r, "rewrite")
	if !ok {
		t.Fatalf("expected rewrite, got %+v", r)
	}
	if ru.Pattern != "^/promo$" {
		t.Errorf("pattern not rebased: %q", ru.Pattern)
	}
	if ru.Flag != "permanent" {
		t.Errorf("R=301 should be permanent, got %q", ru.Flag)
	}
}

func TestRewriteRuleInternalRebaseSubdir(t *testing.T) {
	// .htaccess living under /shop/ rewriting product URLs.
	r := Convert(`RewriteRule ^item/([0-9]+)$ product.php?id=$1 [L]`, "/shop/")
	ru, ok := firstRule(r, "rewrite")
	if !ok {
		t.Fatalf("expected rewrite, got %+v", r)
	}
	if ru.Pattern != "^/shop/item/([0-9]+)$" {
		t.Errorf("subdir pattern not rebased: %q", ru.Pattern)
	}
	if ru.Replacement != "/shop/product.php?id=$1" {
		t.Errorf("subdir replacement not rebased: %q", ru.Replacement)
	}
	if ru.Flag != "last" {
		t.Errorf("internal rewrite should be last, got %q", ru.Flag)
	}
}

func TestRewriteBaseOverridesArg(t *testing.T) {
	in := `RewriteBase /app/
RewriteRule ^go$ target.php [L]`
	r := Convert(in, "/")
	ru, _ := firstRule(r, "rewrite")
	if ru.Pattern != "^/app/go$" {
		t.Errorf("RewriteBase should override basePath: %q", ru.Pattern)
	}
}

func TestHeaderSet(t *testing.T) {
	r := Convert(`Header set X-Frame-Options "SAMEORIGIN"`, "/")
	ru, ok := firstRule(r, "custom_header")
	if !ok {
		t.Fatalf("expected custom_header, got %+v", r)
	}
	if ru.Name != "X-Frame-Options" || ru.Value != "SAMEORIGIN" {
		t.Errorf("unexpected header: %+v", ru)
	}
	if ru.Always == nil || *ru.Always {
		t.Errorf("non-always Header should set Always=false")
	}
}

func TestHeaderAlways(t *testing.T) {
	r := Convert(`Header always set Strict-Transport-Security "max-age=63072000"`, "/")
	ru, _ := firstRule(r, "custom_header")
	if ru.Always == nil || !*ru.Always {
		t.Errorf("`Header always set` should set Always=true: %+v", ru)
	}
	if ru.Value != "max-age=63072000" {
		t.Errorf("quoted value not preserved: %q", ru.Value)
	}
}

func TestHeaderUnsetWarns(t *testing.T) {
	r := Convert(`Header unset X-Powered-By`, "/")
	if len(r.Rules) != 0 {
		t.Errorf("Header unset has no typed equivalent, should emit no rule")
	}
	if len(r.Warnings) == 0 {
		t.Errorf("Header unset should warn")
	}
}

func TestPHPValueAndFlag(t *testing.T) {
	r := Convert("php_value upload_max_filesize 64M\nphp_flag display_errors off", "/")
	if len(r.Rules) != 2 {
		t.Fatalf("expected 2 php_setting rules, got %d", len(r.Rules))
	}
	if r.Rules[0].Name != "upload_max_filesize" || r.Rules[0].Value != "64M" {
		t.Errorf("php_value mapping wrong: %+v", r.Rules[0])
	}
	if r.Rules[1].Name != "display_errors" || r.Rules[1].Value != "off" {
		t.Errorf("php_flag mapping wrong: %+v", r.Rules[1])
	}
}

func TestOrderAllowDenyWhitelist(t *testing.T) {
	in := `Order Allow,Deny
Allow from 192.168.1.0/24
Allow from 10.0.0.5`
	r := Convert(in, "/admin/")
	ru, ok := firstRule(r, "ip_access")
	if !ok {
		t.Fatalf("expected ip_access, got %+v", r)
	}
	if ru.Mode != "allow_list" {
		t.Errorf("Order Allow,Deny should be allow_list, got %q", ru.Mode)
	}
	if ru.Path != "/admin/" {
		t.Errorf("ip_access should be scoped to base path, got %q", ru.Path)
	}
	if len(ru.IPs) != 2 {
		t.Errorf("expected 2 IPs, got %v", ru.IPs)
	}
}

func TestOrderDenyAllowBlacklist(t *testing.T) {
	// Subdir scope: a deny_list compiles to `location /uploads/ { … }`, which
	// does NOT clobber the default routing, so it's emitted.
	in := `Order Deny,Allow
Deny from 1.2.3.4`
	r := Convert(in, "/uploads/")
	ru, _ := firstRule(r, "ip_access")
	if ru.Mode != "deny_list" {
		t.Errorf("Order Deny,Allow should be deny_list, got %q", ru.Mode)
	}
	if len(ru.IPs) != 1 || ru.IPs[0] != "1.2.3.4" {
		t.Errorf("unexpected deny IPs: %v", ru.IPs)
	}
}

// Docroot-level (Path "/") allow_list / deny_list would compile to a
// `location /` that REPLACES the vhost's default routing (dropping try_files
// + the PHP location → PHP served as source). Must warn-and-skip, NOT emit.
func TestDocrootAllowListRefusedToProtectPHP(t *testing.T) {
	in := `Order Allow,Deny
Allow from 203.0.113.0/24`
	r := Convert(in, "/") // docroot
	if _, ok := firstRule(r, "ip_access"); ok {
		t.Fatalf("docroot allow_list must NOT be emitted (would serve PHP as source): %+v", r.Rules)
	}
	if !hasSecurityWarning(r) {
		t.Errorf("refusing a docroot whole-site restriction must raise a security warning")
	}
}

func TestDocrootDenyListRefused(t *testing.T) {
	in := `Order Deny,Allow
Deny from 1.2.3.4`
	r := Convert(in, "/")
	if _, ok := firstRule(r, "ip_access"); ok {
		t.Errorf("docroot deny_list must NOT be emitted (would clobber PHP routing)")
	}
	if !hasSecurityWarning(r) {
		t.Errorf("refusing a docroot deny_list must raise a security warning")
	}
}

// The one safe docroot case: `Deny from all` locks the whole site (everything
// 403s, no routing needed) → emitted as deny-all.
func TestDocrootDenyAllStillEmitted(t *testing.T) {
	in := `Order Allow,Deny
Deny from all`
	r := Convert(in, "/")
	ru, ok := firstRule(r, "ip_access")
	if !ok {
		t.Fatalf("docroot Deny-from-all is safe (fully locked) and should still emit, got %+v", r)
	}
	if ru.Mode != "allow_list" || len(ru.IPs) != 0 {
		t.Errorf("expected deny-all (allow_list, no IPs), got %+v", ru)
	}
}

func TestDenyFromAllProducesDenyAll(t *testing.T) {
	in := `Order Allow,Deny
Deny from all`
	r := Convert(in, "/private/")
	ru, ok := firstRule(r, "ip_access")
	if !ok {
		t.Fatalf("Deny from all should still emit a deny-all ip_access, got %+v", r)
	}
	if ru.Mode != "allow_list" || len(ru.IPs) != 0 {
		t.Errorf("deny-all should be allow_list with no IPs (compiles to `deny all`), got %+v", ru)
	}
}

// ----- FAIL-CLOSED negative tests: must DROP, never emit the permissive half -----

func TestRewriteCondNonFrontControllerDropped(t *testing.T) {
	// A hotlink/referer guard: dropping the cond but keeping the rule would
	// invert the protection. Must drop the whole thing.
	in := `RewriteCond %{HTTP_REFERER} !^https://example.com [NC]
RewriteRule \.(jpg|png)$ - [F]`
	r := Convert(in, "/")
	if len(r.Rules) != 0 {
		t.Fatalf("conditional rule must be dropped, got rules: %+v", r.Rules)
	}
	if !hasSecurityWarning(r) {
		t.Errorf("dropping a conditional protection must raise a security warning")
	}
}

func TestForbidRuleNotSilentlyDropped(t *testing.T) {
	r := Convert(`RewriteRule ^secret/ - [F]`, "/")
	if len(r.Rules) != 0 {
		t.Errorf("[F] has no typed equivalent, should emit no rule")
	}
	if !hasSecurityWarning(r) {
		t.Errorf("[F] (forbid) dropped → must be a SECURITY warning, not silent")
	}
}

func TestMixedAllowDenyDroppedFailClosed(t *testing.T) {
	in := `Order Allow,Deny
Allow from 10.0.0.0/8
Deny from 10.0.0.99`
	r := Convert(in, "/")
	if _, ok := firstRule(r, "ip_access"); ok {
		t.Errorf("mixed allow+deny can't be expressed as a flat list — must drop, not emit")
	}
	if !hasSecurityWarning(r) {
		t.Errorf("dropping a mixed access block must be a security warning")
	}
}

func TestAllowDenyWithoutOrderDropped(t *testing.T) {
	r := Convert(`Allow from 1.2.3.4`, "/")
	if _, ok := firstRule(r, "ip_access"); ok {
		t.Errorf("Allow with no Order → default action unknown → must drop")
	}
	if !hasSecurityWarning(r) {
		t.Errorf("ambiguous access must raise a security warning")
	}
}

func TestAllowHostnameDroppedFailClosed(t *testing.T) {
	// A hostname in a deny list: if we silently skipped it, the deny list
	// would be incomplete → access granted that should be denied.
	in := `Order Deny,Allow
Deny from evil.example.com`
	r := Convert(in, "/")
	if _, ok := firstRule(r, "ip_access"); ok {
		t.Errorf("unparseable host in deny list must drop the whole block")
	}
	if !hasSecurityWarning(r) {
		t.Errorf("dropping an incomplete deny list must be a security warning")
	}
}

func TestRequireValidUserWarns(t *testing.T) {
	in := `AuthType Basic
AuthName "Restricted"
AuthUserFile /home/u/.htpasswd
Require valid-user`
	r := Convert(in, "/")
	if len(r.Rules) != 0 {
		t.Errorf("auth can't be migrated (no creds in .htaccess) — emit no rule")
	}
	if !hasSecurityWarning(r) {
		t.Errorf("Require valid-user should warn to use Directory Privacy")
	}
}

func TestRequireIPAllowList(t *testing.T) {
	in := `Require ip 203.0.113.0/24 198.51.100.7`
	r := Convert(in, "/wp-admin/")
	ru, ok := firstRule(r, "ip_access")
	if !ok {
		t.Fatalf("Require ip should map to allow_list, got %+v", r)
	}
	if ru.Mode != "allow_list" || len(ru.IPs) != 2 {
		t.Errorf("unexpected Require ip mapping: %+v", ru)
	}
}

func TestRequireAllDenied(t *testing.T) {
	r := Convert(`Require all denied`, "/")
	ru, ok := firstRule(r, "ip_access")
	if !ok || ru.Mode != "allow_list" || len(ru.IPs) != 0 {
		t.Errorf("Require all denied → deny-all (allow_list, no IPs), got %+v", r.Rules)
	}
}

func TestCommentsAndContinuationsAndBlankLines(t *testing.T) {
	in := `# a comment
   # indented comment

Header set X-Test \
  "value spanning"`
	r := Convert(in, "/")
	ru, ok := firstRule(r, "custom_header")
	if !ok {
		t.Fatalf("continuation join failed: %+v", r)
	}
	if ru.Name != "X-Test" || ru.Value != "value spanning" {
		t.Errorf("unexpected continuation parse: %+v", ru)
	}
}

func TestUnknownDirectiveWarns(t *testing.T) {
	r := Convert(`SetEnvIf User-Agent "badbot" bad_bot`, "/")
	if len(r.Warnings) == 0 {
		t.Errorf("unknown/unsupported directive should warn")
	}
}

func TestRuleOrderPreserved(t *testing.T) {
	in := `Header set A 1
Redirect 301 /x /y
php_value memory_limit 256M`
	r := Convert(in, "/")
	if len(r.Rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(r.Rules))
	}
	if r.Rules[0].Type != "custom_header" || r.Rules[1].Type != "rewrite" || r.Rules[2].Type != "php_setting" {
		t.Errorf("rule order not preserved: %v", []string{r.Rules[0].Type, r.Rules[1].Type, r.Rules[2].Type})
	}
}
