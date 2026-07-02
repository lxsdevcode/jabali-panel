package commands

import (
	"regexp"
	"testing"
)

// buildCacheGate is the correctness-critical #601 multi-path gate: a wrong body
// serves the wrong page cross-visitor or breaks nginx. Cover the branches.
func TestBuildCacheGate(t *testing.T) {
	cases := []struct {
		name     string
		paths    []string
		fallback string
		want     string
	}{
		{"empty+root fallback", nil, "/", ""},
		{"empty+subdir fallback", nil, "/blog", "(/blog)"},
		{"empty+invalid fallback", nil, "/bad path", ""}, // sanitizeCachePath → "/" → no gate
		{"root install alone", []string{"/"}, "/", ""},
		{"two subdirs", []string{"/blog", "/shop"}, "/", "(/blog|/shop)"},
		{"root present wins", []string{"/blog", "/"}, "/", ""},
		{"nested subdir allowed", []string{"/shop/eu"}, "/", "(/shop/eu)"},
		{"invalid skipped not collapsed", []string{"/bad path", "/blog"}, "/", "(/blog)"},
		{"dedupe", []string{"/blog", "/blog"}, "/", "(/blog)"},
		{"all invalid → fallback", []string{"/bad path"}, "/shop", "(/shop)"},
		{"all invalid + root fallback → no gate", []string{"/bad path"}, "/", ""},
	}
	for _, c := range cases {
		if got := buildCacheGate(c.paths, c.fallback); got != c.want {
			t.Errorf("%s: buildCacheGate(%v,%q)=%q want %q", c.name, c.paths, c.fallback, got, c.want)
		}
	}
}

// The rendered gate ^<body>(/|$) must match a path and its children but not a
// same-prefix sibling (/blog vs /blogger) — the #619/#420 boundary.
func TestCacheGateRegexBoundary(t *testing.T) {
	body := buildCacheGate([]string{"/blog", "/shop"}, "/")
	re := regexp.MustCompile("^" + body + "(/|$)")
	match := map[string]bool{
		"/blog": true, "/blog/": true, "/blog/post": true,
		"/shop": true, "/shop/item": true,
		"/blogger": false, "/shopping": false, "/about": false, "/": false,
	}
	for uri, want := range match {
		if got := re.MatchString(uri); got != want {
			t.Errorf("gate %q match(%q)=%v want %v", body, uri, got, want)
		}
	}
}

// SECURITY: user-provided cache bypass rules must never break out of the nginx
// regex string. Injection payloads must be dropped; safe entries QuoteMeta'd.
func TestSanitizeBypassRules(t *testing.T) {
	// paths → "|<quoted>" suffix
	pathCases := map[string]string{
		"":                              "",
		"/private":                      "|/private",
		"/shop/eu":                      "|/shop/eu",
		"/my.account":                   `|/my\.account`, // "." escaped, still safe
		`/a" { return 444; } location `: "",              // quote+space+brace → dropped
		"/a|b":                          "",              // pipe → dropped
		"/a$(x)":                        "",              // $ ( ) → dropped
		"/a\nb":                         "",              // newline → dropped
		"noslash":                       "",              // must start with /
	}
	for in, want := range pathCases {
		if got := sanitizeBypassPaths([]string{in}); got != want {
			t.Errorf("sanitizeBypassPaths(%q)=%q want %q", in, got, want)
		}
	}
	// multiple + dedupe
	if got := sanitizeBypassPaths([]string{"/a", "/a", `/bad"`, "/b"}); got != "|/a|/b" {
		t.Errorf("multi paths = %q want |/a|/b", got)
	}
}
