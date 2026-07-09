package commands

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// JAB-45: rsync must refuse a source path outside the source account home,
// including ../ traversal, and require a bare src_account.
func TestRsyncRemoteHome_RejectsCrossAccountSource(t *testing.T) {
	base := map[string]any{
		"job_id": "j1", "host": "src.example.com", "ssh_user": "root",
		"secret_path": "/etc/jabali-panel/migration-secrets/j1.env",
		"dest_path":   "/home/alice/domains/d/public_html", "dest_user": "alice",
	}
	cases := []struct {
		name, account, src string
		wantErr            string
	}{
		{"cross-account", "alice", "/home/victim/public_html", "under /home/<src_account>"},
		{"traversal", "alice", "/home/alice/../victim/public_html", "under /home/<src_account>"},
		{"empty-account", "", "/home/alice/public_html", "src_account required"},
		{"slashy-account", "a/b", "/home/a/b/public_html", "bare account name"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := map[string]any{}
			for k, v := range base {
				m[k] = v
			}
			m["src_account"] = c.account
			m["src_path"] = c.src
			raw, _ := json.Marshal(m)
			_, err := migrationRsyncRemoteHomeHandler(context.Background(), raw)
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("got err=%v, want contains %q", err, c.wantErr)
			}
		})
	}
}
