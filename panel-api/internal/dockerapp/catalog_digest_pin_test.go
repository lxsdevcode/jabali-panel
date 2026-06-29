package dockerapp

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// catalogRoot locates install/docker-apps from the test working dir
// (panel-api/internal/dockerapp → repo root is three levels up).
func catalogRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join("..", "..", "..", "install", "docker-apps")
	if _, err := os.Stat(root); err != nil {
		t.Skipf("catalog dir not found at %s: %v", root, err)
	}
	return root
}

var sha256RefRe = regexp.MustCompile(`@sha256:[0-9a-f]{64}`)

// TestCatalogImagesDigestPinned enforces GH #458: every image reference in the
// shipped catalog — the primary image_channel in each app.yaml AND every
// concrete (non-templated) image: in each compose.yml.tmpl — must be
// digest-pinned (@sha256:<digest>). A floating/:latest/tag-only ref fails here
// in CI before it can ship.
func TestCatalogImagesDigestPinned(t *testing.T) {
	root := catalogRoot(t)
	apps, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var unpinned []string
	for _, a := range apps {
		if !a.IsDir() {
			continue
		}
		// app.yaml image_channel
		if b, err := os.ReadFile(filepath.Join(root, a.Name(), "app.yaml")); err == nil {
			for _, ln := range strings.Split(string(b), "\n") {
				t := strings.TrimSpace(ln)
				if strings.HasPrefix(t, "image_channel:") && !sha256RefRe.MatchString(t) {
					unpinned = append(unpinned, a.Name()+"/app.yaml: "+t)
				}
			}
		}
		// compose.yml.tmpl concrete image: lines (skip templated {{ ... }})
		if b, err := os.ReadFile(filepath.Join(root, a.Name(), "compose.yml.tmpl")); err == nil {
			for _, ln := range strings.Split(string(b), "\n") {
				t := strings.TrimSpace(ln)
				if strings.HasPrefix(t, "image:") && !strings.Contains(t, "{{") && !sha256RefRe.MatchString(t) {
					unpinned = append(unpinned, a.Name()+"/compose.yml.tmpl: "+t)
				}
			}
		}
	}
	if len(unpinned) > 0 {
		t.Fatalf("catalog images not digest-pinned (GH #458):\n  %s", strings.Join(unpinned, "\n  "))
	}
}

// Gitea #530: validateComposeImages rejects a literal sidecar image with a
// mutable (non-digest) tag, and accepts pinned + templated images.
func TestValidateComposeImages_RejectsMutableSidecar(t *testing.T) {
	bad := "services:\n  app:\n    image: {{ .ImageChannel }}\n  db:\n    image: postgres:17-alpine\n"
	if err := validateComposeImages(bad); err == nil {
		t.Error("mutable sidecar tag must be rejected (#530)")
	}
	good := "services:\n  app:\n    image: {{ .ImageChannel }}\n  db:\n    image: postgres:17-alpine@sha256:" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" + "\n"
	if err := validateComposeImages(good); err != nil {
		t.Errorf("pinned + templated images must pass: %v", err)
	}
}
