package commands

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAppArtifactSHA256RejectsMismatch proves each catalog app installer that
// downloads a release archive actually enforces its pinned SHA-256 and rejects
// content that does not match (GH security pinning batch: #439/#451/#452/#453/
// #454). A temp file with arbitrary bytes cannot match any pinned release hash,
// so every verifier must return a non-nil error — guarding against a
// regression that reintroduces the old silent-accept behaviour.
func TestAppArtifactSHA256RejectsMismatch(t *testing.T) {
	verifiers := map[string]func(string) error{
		"phpbb":      verifyPhpbbSHA256,
		"opencart":   verifyOpenCartSHA256,
		"drupal":     verifyDrupalSHA256,
		"joomla":     verifyJoomlaSHA256,
		"prestashop": verifyPrestaShopSHA256,
	}
	for name, verify := range verifiers {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "artifact")
			if err := os.WriteFile(p, []byte("not the real "+name+" release archive"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := verify(p); err == nil {
				t.Fatalf("%s: verifier accepted a mismatched artifact — integrity check skipped", name)
			}
		})
	}
}
