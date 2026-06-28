package commands

import "testing"

// TestValidateMoodleSubdir guards against path traversal via a tenant-supplied
// subdirectory (GH #310 security review). The agent is the trust boundary, so
// "../../etc"-style values must be rejected even though the panel validates at
// create time.
func TestValidateMoodleSubdir(t *testing.T) {
	const docroot = "/home/alice/domains/example.com/public_html"
	cases := []struct {
		name    string
		subdir  string
		wantErr bool
	}{
		{"root install (empty)", "", false},
		{"simple subdir", "moodle", false},
		{"nested subdir", "apps/moodle", false},
		{"trailing slash", "moodle/", false},
		{"dotted but safe", "moodle.v5", false},
		{"escape with ..", "../../../etc", true},
		{"escape single level", "..", true},
		{"escape mid path", "moodle/../../secret", true},
		// A leading slash is stripped by Trim + neutralized by filepath.Join,
		// so it resolves inside the docroot (docroot/etc/passwd) — not an escape.
		{"absolute neutralized to within docroot", "/etc/passwd", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMoodleSubdir(docroot, tc.subdir)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for subdir %q, got nil", tc.subdir)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for subdir %q: %v", tc.subdir, err)
			}
		})
	}
}
