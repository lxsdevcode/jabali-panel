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
	// The gate is now driven by CacheGate (buildCacheGate output), computed in
	// writeVhost from cache_path/cache_paths. Render with the same computation.
	render := func(paths []string, cachePath string) string {
		var b bytes.Buffer
		_ = tmpl.Execute(&b, vhostData{
			Domain: "ex.com", DocRoot: "/home/u/public_html/ex.com", HasPHP: true,
			PHPVersion: "8.3", Username: "u", IsEnabled: true,
			CacheEnabled: true, CacheKeyZone: "jabali", CacheTTL: "60s",
			CachePath: cachePath, CacheGate: buildCacheGate(paths, cachePath),
		})
		return b.String()
	}
	// whole-domain ("/" or empty, or any union containing a root install): no gate.
	for _, root := range []string{"/", ""} {
		if strings.Contains(render(nil, root), `!~ "^`) {
			t.Errorf("CachePath=%q must not emit a path gate", root)
		}
	}
	if strings.Contains(render([]string{"/blog", "/"}, "/"), `!~ "^`) {
		t.Error("a root install in the union must drop the gate (whole domain)")
	}
	// single subdir: gate scopes to that prefix.
	if !strings.Contains(render(nil, "/blog"), `if ($uri !~ "^(/blog)(/|$)") { set $jabali_skip 1; }`) {
		t.Errorf("CachePath=/blog must scope cache to ^(/blog)\n%s", render(nil, "/blog"))
	}
	// GH #601 multi-path: union of two subdir installs → alternation gate.
	if !strings.Contains(render([]string{"/blog", "/shop"}, "/"), `if ($uri !~ "^(/blog|/shop)(/|$)") { set $jabali_skip 1; }`) {
		t.Errorf("cache_paths [/blog /shop] must gate ^(/blog|/shop)\n%s", render([]string{"/blog", "/shop"}, "/"))
	}
}

// SECURITY (nginx template/config injection): the agent must reject any cache
// path that isn't "/" or a single safe segment, falling back to "/".
func TestSanitizeCachePath(t *testing.T) {
	safe := map[string]string{"/": "/", "/blog": "/blog", "/shop2": "/shop2", "": "/"}
	for in, want := range safe {
		if got := sanitizeCachePath(in); got != want {
			t.Errorf("sanitizeCachePath(%q) = %q, want %q", in, got, want)
		}
	}
	for _, bad := range []string{
		`/a" { return 444; } location /x {`, "/a/b", "/UP", "/a;b", "/a\n}", "/../etc", "/a ", "blog", "/-x",
	} {
		if got := sanitizeCachePath(bad); got != "/" {
			t.Errorf("sanitizeCachePath(%q) = %q, want \"/\" (unsafe must fall back)", bad, got)
		}
	}
}

// GH #616: a sanitized URL-exclusion suffix must appear inside the built-in
// bypass regex (both PHP location blocks), so those paths always bypass.
func TestVhostTemplate_ExtraBypass(t *testing.T) {
	t.Parallel()
	tmpl := template.Must(template.New("v").Parse(vhostTemplate))
	var b bytes.Buffer
	_ = tmpl.Execute(&b, vhostData{
		Domain: "ex.com", DocRoot: "/home/u/public_html/ex.com", HasPHP: true,
		PHPVersion: "8.3", Username: "u", IsEnabled: true,
		CacheEnabled: true, CacheKeyZone: "jabali", CacheTTL: "60s",
		CacheExtraBypass: sanitizeBypassPaths([]string{"/private", "/members"}),
	})
	out := b.String()
	// both location blocks carry the merged bypass regex
	if n := strings.Count(out, `/edd-api/|/private|/members") { set $jabali_skip 1; }`); n != 2 {
		t.Errorf("extra bypass folded into %d blocks, want 2\n%s", n, out)
	}
}
