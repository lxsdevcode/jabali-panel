package commands

import (
	"bytes"
	"strings"
	"testing"
	"text/template"
)

// Gitea #416/#417/#419: the page-cache gate must be fail-CLOSED (default skip,
// allow only no-cookie / benign-analytics) and must NOT advertise public/CDN
// caching for dynamic PHP responses.
func TestVhostTemplate_PageCacheIsFailClosed(t *testing.T) {
	t.Parallel()
	tmpl, err := template.New("vhost").Parse(vhostTemplate)
	if err != nil {
		t.Fatal(err)
	}
	vd := vhostData{
		Domain: "example.com", DocRoot: "/home/u/public_html/example.com",
		HasPHP: true, PHPVersion: "8.3", Username: "u", IsEnabled: true,
		CacheEnabled: true, CacheKeyZone: "jabali", CacheTTL: "60s",
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vd); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "set $jabali_skip 1;") {
		t.Error("page cache must default to skip (fail-closed)")
	}
	if !strings.Contains(out, "_ga") || !strings.Contains(out, "wordpress_test_cookie") {
		t.Error("missing benign-cookie allowlist")
	}
	// PHP responses must be private/no-cache, never public to clients/CDNs.
	if !strings.Contains(out, "private, no-cache, max-age=0") {
		t.Error("PHP block must emit private/no-cache Cache-Control")
	}
	if strings.Contains(out, "set $jabali_cc") {
		t.Error("PHP block must not advertise public/CDN caching for dynamic responses")
	}
}

// Gitea #420: the page cache must be scoped to the install path prefix.
func TestVhostTemplate_PathScopedCache(t *testing.T) {
	t.Parallel()
	tmpl := template.Must(template.New("v").Parse(vhostTemplate))
	render := func(cachePath string) string {
		var b bytes.Buffer
		_ = tmpl.Execute(&b, vhostData{
			Domain: "ex.com", DocRoot: "/home/u/public_html/ex.com", HasPHP: true,
			PHPVersion: "8.3", Username: "u", IsEnabled: true,
			CacheEnabled: true, CacheKeyZone: "jabali", CacheTTL: "60s", CachePath: cachePath,
		})
		return b.String()
	}
	// whole-domain ("/" or empty): no path gate.
	for _, root := range []string{"/", ""} {
		if strings.Contains(render(root), `!~ "^`) {
			t.Errorf("CachePath=%q must not emit a path gate", root)
		}
	}
	// subdir: gate scopes to that prefix.
	out := render("/blog")
	if !strings.Contains(out, `if ($request_uri !~ "^/blog(/|$)") { set $jabali_skip 1; }`) {
		t.Errorf("CachePath=/blog must scope cache to ^/blog\n%s", out)
	}
}
