package htaccess

import "testing"

func countType(r Result, typ string) int {
	n := 0
	for _, x := range r.Rules {
		if x.Type == typ {
			n++
		}
	}
	return n
}

// Full stock WordPress must yield ZERO rewrite rules (index.php passthrough +
// front controller are both no-ops) and no security warnings.
func TestStockWordPressFullFile(t *testing.T) {
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
	if countType(r, "rewrite") != 0 {
		t.Fatalf("stock WP should yield 0 rewrite rules, got %d: %+v", countType(r, "rewrite"), r.Rules)
	}
	if hasSecurityWarning(r) {
		t.Fatalf("stock WP should produce no security warnings: %+v", r.Warnings)
	}
}

// BLOCKER regression: Order Deny,Allow + Deny from all must DENY everyone,
// not allow everyone.
func TestDenyAllowDenyFromAllDeniesEveryone(t *testing.T) {
	in := `Order Deny,Allow
Deny from all`
	r := Convert(in, "/private/")
	ru, ok := firstRule(r, "ip_access")
	if !ok {
		t.Fatalf("Deny,Allow + Deny from all must emit a deny-all rule, got %+v", r)
	}
	if ru.Mode != "allow_list" || len(ru.IPs) != 0 {
		t.Errorf("must be deny-all (allow_list, no IPs), got %+v", ru)
	}
}

// The canonical "deny everyone except my office IP", written in Deny,Allow
// order, must whitelist that IP.
func TestDenyAllExceptIP(t *testing.T) {
	in := `Order Deny,Allow
Deny from all
Allow from 203.0.113.10`
	r := Convert(in, "/wp-login.php")
	ru, ok := firstRule(r, "ip_access")
	if !ok {
		t.Fatalf("expected an allow_list whitelist, got %+v", r)
	}
	if ru.Mode != "allow_list" || len(ru.IPs) != 1 || ru.IPs[0] != "203.0.113.10" {
		t.Errorf("deny-all-except-IP should whitelist that IP, got %+v", ru)
	}
}

// Allow,Deny with no Allow at all must deny everyone (default deny).
func TestAllowDenyNoAllowDeniesEveryone(t *testing.T) {
	in := `Order Allow,Deny
Deny from 1.2.3.4`
	r := Convert(in, "/")
	ru, ok := firstRule(r, "ip_access")
	if !ok {
		t.Fatalf("Allow,Deny with no Allow must deny everyone, got %+v", r)
	}
	if ru.Mode != "allow_list" || len(ru.IPs) != 0 {
		t.Errorf("expected deny-all, got %+v", ru)
	}
}

func TestScopedContainerSkippedAndWarned(t *testing.T) {
	in := `<Files "wp-config.php">
Order Allow,Deny
Deny from all
</Files>
Header set X-Test 1`
	r := Convert(in, "/")
	// The <Files> contents must NOT be applied globally...
	if _, ok := firstRule(r, "ip_access"); ok {
		t.Errorf("scoped <Files> access rule must not be applied globally")
	}
	if !hasSecurityWarning(r) {
		t.Errorf("a skipped scoped container must raise a security warning")
	}
	// ...but a directive AFTER the closing tag must still convert.
	if _, ok := firstRule(r, "custom_header"); !ok {
		t.Errorf("directive after </Files> should still convert; processing resumed wrong: %+v", r.Rules)
	}
}

func TestIfModuleWrapperContentsProcessed(t *testing.T) {
	in := `<IfModule mod_headers.c>
Header set X-Frame-Options SAMEORIGIN
</IfModule>`
	r := Convert(in, "/")
	if _, ok := firstRule(r, "custom_header"); !ok {
		t.Errorf("contents of <IfModule> must be processed, got %+v", r.Rules)
	}
	if len(r.Warnings) != 0 {
		t.Errorf("<IfModule> wrapper itself should not warn: %+v", r.Warnings)
	}
}

func TestQSAWarns(t *testing.T) {
	r := Convert(`RewriteRule ^p/([0-9]+)$ /product.php?id=$1 [QSA,L]`, "/")
	if _, ok := firstRule(r, "rewrite"); !ok {
		t.Fatalf("rule should still convert")
	}
	found := false
	for _, w := range r.Warnings {
		if !w.Security {
			found = true
		}
	}
	if !found {
		t.Errorf("[QSA] should produce a (non-security) warning")
	}
}

func TestNoSubstitutionDashIsNoOp(t *testing.T) {
	r := Convert(`RewriteRule ^index\.php$ - [L]`, "/")
	if len(r.Rules) != 0 {
		t.Errorf(`"-" substitution must emit no rule, got %+v`, r.Rules)
	}
}

func TestPoolPinnedPHPWarns(t *testing.T) {
	for _, name := range []string{"open_basedir", "disable_functions"} {
		r := Convert("php_value "+name+" /tmp", "/")
		if len(r.Rules) != 0 {
			t.Errorf("%s is pool-pinned (php_admin_value); must not convert", name)
		}
		if len(r.Warnings) == 0 {
			t.Errorf("%s should warn that it can't be overridden", name)
		}
	}
}

func TestHeaderNameInjectionRejected(t *testing.T) {
	r := Convert(`Header set "X-Evil; add_header X-Inject bad" 1`, "/")
	if _, ok := firstRule(r, "custom_header"); ok {
		t.Errorf("header name with injection chars must be rejected")
	}
}
