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
