package commands

import (
	"regexp"
	"testing"
)

// TestWordPressCoreVersionPinned guards against regressing to an unpinned
// WordPress core download (GH #456). The installer must request a specific
// Jabali-reviewed version, never `latest`, so installs are reproducible and
// `wp core verify-checksums` has a concrete version to validate against.
func TestWordPressCoreVersionPinned(t *testing.T) {
	if wordpressCoreVersion == "" || wordpressCoreVersion == "latest" {
		t.Fatalf("wordpressCoreVersion must be a pinned release, got %q", wordpressCoreVersion)
	}
	if !regexp.MustCompile(`^[0-9]+(\.[0-9]+){1,2}$`).MatchString(wordpressCoreVersion) {
		t.Fatalf("wordpressCoreVersion %q is not a numeric version (x.y or x.y.z)", wordpressCoreVersion)
	}
}
