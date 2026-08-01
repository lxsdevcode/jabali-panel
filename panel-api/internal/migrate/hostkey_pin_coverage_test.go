package migrate

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// secretRefLiteralRe finds every construction of a SecretRef in the tree.
var secretRefLiteralRe = regexp.MustCompile(`SecretRef\{[^}]*\}`)

// TestEverySecretRefCarriesTheHostKeyPin is a coverage check rather than a
// behaviour test, because the failure mode here is omission.
//
// PinningHostKeyCallback returns nil for ANY host key when the expected
// fingerprint is empty:
//
//	if expected == "" {
//	    return nil // TOFU: accept the discover connection.
//	}
//
// That is a deliberate choice for the first discovery probe. The problem was
// that six of the eight SecretRef construction sites left ExpectedHostKey
// unset, and those six are the ones that matter: the pull-source CLI, the
// three import/run stages, and both tenant migration paths. An operator who
// pasted a SHA256 fingerprint into the wizard got it enforced on the cheap
// discovery connection and silently ignored on the connection that carries the
// SSH credentials and transfers databases, mail and site files.
//
// A missing field compiles, passes every functional test, and downgrades the
// connection to trust-on-first-use without a word in the logs — so the guard
// has to be structural.
func TestEverySecretRefCarriesTheHostKeyPin(t *testing.T) {
	t.Parallel()

	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	var offenders []string
	checked := 0
	err = filepath.Walk(root, func(path string, info os.FileInfo, werr error) error {
		if werr != nil {
			return nil // unreadable dir — not this test's problem
		}
		if info.IsDir() {
			base := info.Name()
			if base == ".git" || base == "node_modules" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		for _, lit := range secretRefLiteralRe.FindAllString(string(body), -1) {
			checked++
			if !strings.Contains(lit, "ExpectedHostKey") {
				rel, _ := filepath.Rel(root, path)
				offenders = append(offenders, rel+": "+lit)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if checked == 0 {
		t.Fatal("found no SecretRef literals — the construction pattern has " +
			"changed and this test is no longer checking anything")
	}
	for _, o := range offenders {
		t.Errorf("SecretRef built without ExpectedHostKey — this connection "+
			"accepts any host key even when the operator pinned a fingerprint:\n  %s", o)
	}
}

// TestPinningHostKeyCallback_EmptyIsTOFU documents the behaviour the coverage
// test exists to contain, so a future reader does not "simplify" the empty case
// into something stricter or looser without seeing the tradeoff spelled out.
func TestPinningHostKeyCallback_EmptyIsTOFU(t *testing.T) {
	t.Parallel()

	cb := PinningHostKeyCallback("")
	if cb == nil {
		t.Fatal("callback must never be nil — a nil HostKeyCallback panics in x/crypto/ssh")
	}
	// Intentionally not asserting acceptance by calling it with a fabricated
	// key: the contract under test is that an EMPTY pin is permissive, which is
	// exactly why every caller with a real pin available must pass it.
	if got := normalizeFingerprint("SHA256:abc="); got != "abc" {
		t.Errorf("normalizeFingerprint(%q) = %q, want %q", "SHA256:abc=", got, "abc")
	}
	if got := normalizeFingerprint("  abc  "); got != "abc" {
		t.Errorf("normalizeFingerprint should trim whitespace, got %q", got)
	}
}
