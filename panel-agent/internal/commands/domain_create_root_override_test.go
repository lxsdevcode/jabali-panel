package commands

import (
	"bytes"
	"strings"
	"testing"

	templ "text/template"
)

func TestCustomDirectivesOverrideRoot_NoDirectives(t *testing.T) {
	if customDirectivesOverrideRoot("") {
		t.Error("empty string must not override root")
	}
	if customDirectivesOverrideRoot("   \n\t") {
		t.Error("whitespace-only must not override root")
	}
}

func TestCustomDirectivesOverrideRoot_PrefixForm(t *testing.T) {
	body := `location / {
    proxy_pass http://127.0.0.1:3200;
}`
	if !customDirectivesOverrideRoot(body) {
		t.Errorf("plain prefix `location /` should override; got false for:\n%s", body)
	}
}

func TestCustomDirectivesOverrideRoot_ExactForm(t *testing.T) {
	body := `location = / { proxy_pass http://127.0.0.1:3200; }`
	if !customDirectivesOverrideRoot(body) {
		t.Error("exact `location = /` should override")
	}
}

func TestCustomDirectivesOverrideRoot_PreferentialPrefix(t *testing.T) {
	body := `location ^~ / { proxy_pass http://127.0.0.1:3200; }`
	if !customDirectivesOverrideRoot(body) {
		t.Error("`location ^~ /` should override")
	}
}

func TestCustomDirectivesOverrideRoot_Subpath(t *testing.T) {
	// `location /api` is a SUBPATH override, not the root. Default
	// `location /` must still render so non-/api requests fall through
	// to the PHP docroot.
	body := `location /api {
    proxy_pass http://127.0.0.1:3200;
}`
	if customDirectivesOverrideRoot(body) {
		t.Errorf("subpath `location /api` must NOT count as root override")
	}
}

func TestCustomDirectivesOverrideRoot_CommentLine(t *testing.T) {
	// A comment line that contains the literal text must not trip
	// the detector — only an actual block opening counts.
	body := `# location / { proxy_pass ... } -- this is commented out
location /healthz { return 200 "ok"; }`
	if customDirectivesOverrideRoot(body) {
		t.Errorf("commented-out `# location /` must not trip detector")
	}
}

func TestCustomDirectivesOverrideRoot_RegexExactRoot(t *testing.T) {
	// `location ~ /` is regex form — nginx treats / as "match any URI
	// containing a slash" (i.e. all). Counts as a full override.
	body := `location ~ / { proxy_pass http://127.0.0.1:3200; }`
	if !customDirectivesOverrideRoot(body) {
		t.Error("`location ~ /` should override (regex matches all)")
	}
}

func TestVhost_RootOverriddenOmitsDefaultLocations(t *testing.T) {
	// Walks the rendered template body to confirm:
	//   - default `try_files $uri $uri/ /index.php?$query_string;` is GONE
	//   - PHP fastcgi block is GONE
	//   - the user's `location / { proxy_pass ... }` is the only `location /`
	tmpl := mustRenderVhost(t, vhostData{
		Domain:           "demo.test",
		DocRoot:          "/home/u/demo",
		HasPHP:           true,
		Username:         "u",
		IndexDirective:   "index index.html;",
		IsEnabled:        true,
		SSLCertPath:      "/etc/ssl/x.crt",
		SSLKeyPath:       "/etc/ssl/x.key",
		ListenIPv4:       "1.2.3.4",
		CustomDirectives: "location / { proxy_pass http://127.0.0.1:3200; }",
		RootOverridden:   true,
	})
	if strings.Contains(tmpl, "try_files $uri $uri/ /index.php?$query_string") {
		t.Errorf("default location / still rendered when RootOverridden=true:\n%s", tmpl)
	}
	if strings.Contains(tmpl, "fastcgi_pass") {
		t.Errorf("PHP fastcgi block still rendered when RootOverridden=true:\n%s", tmpl)
	}
	// Custom directive should appear exactly once.
	if c := strings.Count(tmpl, "proxy_pass http://127.0.0.1:3200"); c != 1 {
		t.Errorf("expected exactly 1 proxy_pass, got %d:\n%s", c, tmpl)
	}
}

func TestVhost_DefaultUnchangedWhenNoOverride(t *testing.T) {
	tmpl := mustRenderVhost(t, vhostData{
		Domain:         "demo.test",
		DocRoot:        "/home/u/demo",
		HasPHP:         true,
		Username:       "u",
		IndexDirective: "index index.php;",
		IsEnabled:      true,
		SSLCertPath:    "/etc/ssl/x.crt",
		SSLKeyPath:     "/etc/ssl/x.key",
		ListenIPv4:     "1.2.3.4",
		RootOverridden: false, // no override
	})
	if !strings.Contains(tmpl, "try_files $uri $uri/ /index.php?$query_string") {
		t.Errorf("default location / missing when RootOverridden=false:\n%s", tmpl)
	}
	if !strings.Contains(tmpl, "fastcgi_pass unix:/run/php/jabali-u/fpm.sock") {
		t.Errorf("PHP fastcgi block missing when RootOverridden=false:\n%s", tmpl)
	}
}

func mustRenderVhost(t *testing.T, vd vhostData) string {
	t.Helper()
	tmpl, err := tmplParse(vhostTemplate)
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vd); err != nil {
		t.Fatalf("execute template: %v", err)
	}
	return buf.String()
}

func tmplParse(s string) (*templ.Template, error) {
	return templ.New("vhost").Parse(s)
}
