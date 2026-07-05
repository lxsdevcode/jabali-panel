package commands

import "testing"

// GH #338: the SSH-session scan must work regardless of the sshd listen port
// (operators commonly move it off 22). sshPeerRe extracts the PEER ip + owning
// sshd process from `ss -tnHp state established` lines; this locks that it
// matches on a custom local port and pulls the peer (not the local address).
func TestSSHPeerRegex_AnyLocalPort(t *testing.T) {
	cases := []struct {
		name, line, peer, proc, pid string
	}{
		{
			name: "default port 22",
			line: `0 0 192.168.100.86:22 192.168.100.200:51944 users:(("sshd-session",pid=537947,fd=7),("sshd-session",pid=537933,fd=7))`,
			peer: "192.168.100.200", proc: "sshd-session", pid: "537947",
		},
		{
			name: "custom port 2222 (the #338 fix)",
			line: `0 0 10.0.0.5:2222 203.0.113.9:40122 users:(("sshd-session",pid=8123,fd=7))`,
			peer: "203.0.113.9", proc: "sshd-session", pid: "8123",
		},
		{
			name: "legacy sshd comm",
			line: `0 0 10.0.0.5:22 198.51.100.4:33000 users:(("sshd",pid=42,fd=4))`,
			peer: "198.51.100.4", proc: "sshd", pid: "42",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := sshPeerRe.FindStringSubmatch(c.line)
			if m == nil {
				t.Fatalf("regex did not match: %s", c.line)
			}
			if m[1] != c.peer || m[2] != c.proc || m[3] != c.pid {
				t.Fatalf("got peer=%s proc=%s pid=%s; want %s/%s/%s", m[1], m[2], m[3], c.peer, c.proc, c.pid)
			}
		})
	}
}

// A non-sshd established connection (e.g. an outbound DB link) must NOT be
// mistaken for an SSH session now that the :22 port filter is gone.
func TestSSHPeerRegex_IgnoresNonSSHD(t *testing.T) {
	line := `0 0 10.0.0.5:44012 10.0.0.9:3306 users:(("mariadbd",pid=900,fd=30))`
	if m := sshPeerRe.FindStringSubmatch(line); m != nil {
		t.Fatalf("non-sshd connection matched as an SSH session: %v", m)
	}
}
