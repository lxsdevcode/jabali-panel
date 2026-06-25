package commands

import "testing"

func TestComputeNginxHTTP2Form(t *testing.T) {
	cases := []struct {
		out        string
		wantParam  string
		wantDirect string
	}{
		{"nginx version: nginx/1.26.0", "", "http2 on;"},        // trixie
		{"nginx version: nginx/1.25.1", "", "http2 on;"},        // exactly the boundary
		{"nginx version: nginx/1.25.0", " http2", ""},           // just below
		{"nginx version: nginx/1.24.0 (Ubuntu)", " http2", ""},  // noble
		{"nginx version: nginx/1.22.1", " http2", ""},           // bookworm
		{"nginx version: nginx/1.18.0 (Ubuntu)", " http2", ""},  // jammy
		{"nginx version: nginx/2.0.0", "", "http2 on;"},         // future major
		{"garbage", " http2", ""},                                // unparseable -> safe portable form
	}
	for _, c := range cases {
		got := computeNginxHTTP2Form(c.out)
		if got.Param != c.wantParam || got.Directive != c.wantDirect {
			t.Errorf("%q -> {%q,%q}, want {%q,%q}", c.out, got.Param, got.Directive, c.wantParam, c.wantDirect)
		}
	}
}
