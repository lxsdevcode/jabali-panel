package eventsources

import "testing"

// GH #403: jabali's own systemctl management polls (the #280 cron last_run
// harvester, run as `sudo -u <tenant> systemctl --user show jabali-cron-*`)
// must NOT count toward a tenant's suspicious-exec burst, but a real
// exploit's shell/interpreter at a different exe path must still count.
func TestIsJabaliManagementExec(t *testing.T) {
	cases := []struct {
		name string
		ev   auditEventDTO
		want bool
	}{
		{"systemctl usr/bin", auditEventDTO{Exe: "/usr/bin/systemctl", Comm: "systemctl"}, true},
		{"systemctl bin", auditEventDTO{Exe: "/bin/systemctl", Comm: "systemctl"}, true},
		{"empty exe falls back to comm", auditEventDTO{Exe: "", Comm: "systemctl"}, true},
		{"real shell still counts", auditEventDTO{Exe: "/bin/bash", Comm: "bash"}, false},
		{"python interpreter still counts", auditEventDTO{Exe: "/usr/bin/python3", Comm: "python3"}, false},
		{"fake systemctl elsewhere still counts", auditEventDTO{Exe: "/home/u/.local/bin/systemctl", Comm: "systemctl"}, false},
		{"empty event", auditEventDTO{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isJabaliManagementExec(c.ev); got != c.want {
				t.Errorf("isJabaliManagementExec(%+v) = %v, want %v", c.ev, got, c.want)
			}
		})
	}
}
