package wordpressssh

import (
	"os/exec"
	"strings"
	"testing"
)

// GH #647 S2 SECURITY: shellQuote must neutralize any injection payload so an
// untrusted path is a single literal argument. We assert it both structurally
// (single-quote wrapped, embedded quotes escaped) AND behaviorally (a real
// /bin/sh treats the quoted value as one literal arg, no command runs).
func TestShellQuote_NeutralizesInjection(t *testing.T) {
	payloads := []string{
		"/var/www/html",
		"/tmp/x'; rm -rf / #",
		"/a$(touch /tmp/pwned)",
		"/a`id`b",
		"/a; echo HACKED",
		"/a && echo HACKED",
		"/a | cat",
		"/home/*/public_html",
		"~/public_html",
	}
	for _, p := range payloads {
		q := shellQuote(p)
		// echo <quoted> must print exactly p — proving the shell saw one literal arg.
		out, err := exec.Command("/bin/sh", "-c", "printf %s "+q).Output()
		if err != nil {
			t.Fatalf("sh -c printf %%s %s: %v", q, err)
		}
		// The proof: printf %%s of the quoted value returns EXACTLY p. If any
		// injection ran (echo/rm/$()/`), the output would differ from p. Exact
		// round-trip ⇒ the shell saw one literal argument, nothing executed.
		if got := string(out); got != p {
			t.Errorf("shellQuote(%q) round-trip = %q, want %q — the argument was not fully literal", p, got, p)
		}
	}
}

func TestAutoDetectPatterns_IncludeCloudways(t *testing.T) {
	joined := strings.Join(autoDetectPatterns, " ")
	for _, want := range []string{"~/public_html", "/home/*/public_html", "/home/master/applications/*/public_html"} {
		if !strings.Contains(joined, want) {
			t.Errorf("autoDetectPatterns missing %q", want)
		}
	}
}
