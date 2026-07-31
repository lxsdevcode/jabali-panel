package commands

import (
	"path/filepath"
	"strings"
	"testing"
)

// The agent reads the migration secret file as root. The original guard was a
// bare strings.HasPrefix on the caller-supplied path, which accepts traversal:
// "/etc/jabali-panel/migration-secrets/../../shadow" has the right prefix and
// resolves to /etc/shadow.
//
// Not reachable through shipped callers — every one builds the path from a
// server-generated genULID() job id — but it is a root-side arbitrary-file read
// sitting behind a string test, and the sibling command in this package
// (migration_admin_run.go) already does it properly by taking a job_id and
// constructing the path itself.
//
// This mirrors the accept/reject decision the handler makes so the rule is
// pinned independently of the surrounding rsync plumbing.
func secretPathAllowed(in string) bool {
	clean := filepath.Clean(in)
	base := strings.TrimSuffix(filepath.Base(clean), ".env")
	return filepath.Dir(clean) == migrationSecretsBaseDir &&
		strings.HasSuffix(clean, ".env") &&
		migrationJobIDRe.MatchString(base)
}

func TestMigrationSecretPath_RejectsTraversal(t *testing.T) {
	t.Parallel()
	const good = migrationSecretsBaseDir + "/01JABCDEFGHJKMNPQRSTVWXYZ0.env"

	if !secretPathAllowed(good) {
		t.Fatalf("a legitimate secret path was rejected: %s", good)
	}

	bad := []struct{ name, path string }{
		{"traversal to /etc/shadow", migrationSecretsBaseDir + "/../../shadow"},
		{"traversal with .env suffix", migrationSecretsBaseDir + "/../../etc/shadow.env"},
		{"nested subdirectory", migrationSecretsBaseDir + "/sub/01JABCDEFGHJKMNPQRSTVWXYZ0.env"},
		{"prefix-matching sibling dir", "/etc/jabali-panel/migration-secrets-evil/x.env"},
		{"absolute elsewhere", "/etc/shadow"},
		{"no .env suffix", migrationSecretsBaseDir + "/01JABCDEFGHJKMNPQRSTVWXYZ0"},
		{"basename not a job id", migrationSecretsBaseDir + "/passwd.env"},
		{"short basename", migrationSecretsBaseDir + "/abc.env"},
		{"empty", ""},
		{"dir itself", migrationSecretsBaseDir},
		{"trailing slash traversal", migrationSecretsBaseDir + "/../"},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			if secretPathAllowed(tc.path) {
				t.Errorf("accepted %q — the agent would os.ReadFile this as root", tc.path)
			}
		})
	}
}

// The specific string the old guard let through, called out on its own so the
// regression is unmistakable if anyone reverts to a prefix test.
func TestMigrationSecretPath_OldPrefixGuardWasInsufficient(t *testing.T) {
	t.Parallel()
	const attack = "/etc/jabali-panel/migration-secrets/../../shadow"

	if !strings.HasPrefix(attack, "/etc/jabali-panel/migration-secrets/") {
		t.Fatal("test premise wrong: the attack string should satisfy the old prefix check")
	}
	if got := filepath.Clean(attack); got != "/etc/shadow" {
		t.Fatalf("expected the traversal to resolve to /etc/shadow, got %s", got)
	}
	if secretPathAllowed(attack) {
		t.Error("the hardened guard still accepts the traversal")
	}
}
