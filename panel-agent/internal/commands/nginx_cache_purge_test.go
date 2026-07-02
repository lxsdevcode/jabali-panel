package commands

import "testing"

func TestKeyLineHostURI(t *testing.T) {
	cases := []struct {
		line, host, uri string
	}{
		{"KEY: httpsGETexample.com/blog/post-1", "example.com", "/blog/post-1"},
		{"KEY: httpGETa.com/", "a.com", "/"},
		{"KEY: httpsHEADsub.a.com/x?y=1", "sub.a.com", "/x?y=1"},
	}
	for _, c := range cases {
		if got := keyLineHost([]byte(c.line)); got != c.host {
			t.Errorf("keyLineHost(%q)=%q want %q", c.line, got, c.host)
		}
		if got := keyLineURI([]byte(c.line)); got != c.uri {
			t.Errorf("keyLineURI(%q)=%q want %q", c.line, got, c.uri)
		}
	}
}

func TestURIMatchesAny(t *testing.T) {
	cases := []struct {
		uri   string
		paths []string
		want  bool
	}{
		{"/blog/post", nil, true},                    // no paths = whole domain
		{"/blog/post", []string{"/blog"}, true},      // prefix
		{"/blog", []string{"/blog"}, true},           // exact
		{"/blogger", []string{"/blog"}, false},       // NOT a false prefix (/blog vs /blogger)
		{"/about", []string{"/blog"}, false},         // miss
		{"/anything", []string{"/"}, true},           // root matches all
		{"/blog/p?utm_source=x", []string{"/blog"}, true}, // query stripped before prefix
		{"/shop/item", []string{"/blog", "/shop"}, true},  // multiple paths
	}
	for _, c := range cases {
		if got := uriMatchesAny(c.uri, c.paths); got != c.want {
			t.Errorf("uriMatchesAny(%q,%v)=%v want %v", c.uri, c.paths, got, c.want)
		}
	}
}
