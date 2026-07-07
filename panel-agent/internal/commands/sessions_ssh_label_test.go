package commands

import "testing"

// GH #338: the real logged-in user + SSH-vs-SFTP channel come from the sshd
// process label, not the root privsep-monitor owner.
func TestParseSSHDLabel(t *testing.T) {
	cases := []struct {
		cmdline, user, channel string
	}{
		{"sshd: alice@pts/0", "alice", "ssh"},
		{"sshd: alice@pts/0   ", "alice", "ssh"},
		{"sshd-session: bob@notty", "bob", "sftp"},
		{"sshd: root@pts/1", "root", "ssh"},
		{"sshd: carol@internal-sftp", "carol", "sftp"},
		{"sshd: [accepted]", "", ""},   // pre-auth
		{"sshd: [priv]", "", ""},       // privsep monitor
		{"sshd: [net]", "", ""},        // pre-auth net
		{"/usr/sbin/sshd -D", "", ""},  // listener (no "user@")
		{"", "", ""},
	}
	for _, c := range cases {
		u, ch := parseSSHDLabel(c.cmdline)
		if u != c.user || ch != c.channel {
			t.Errorf("parseSSHDLabel(%q) = (%q,%q), want (%q,%q)", c.cmdline, u, ch, c.user, c.channel)
		}
	}
}
