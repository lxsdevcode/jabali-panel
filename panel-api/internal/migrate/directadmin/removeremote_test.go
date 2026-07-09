package directadmin

import (
	"context"
	"testing"
)

// JAB-50: RemoveRemote must refuse any path outside /tmp/cpmove-*.
func TestRemoveRemote_Allowlist(t *testing.T) {
	d := New()
	for _, bad := range []string{
		"/etc/passwd", "/home/victim/x", "/tmp/other.tar.gz",
		"/tmp/cpmove-../../etc/shadow", "/backup/u.tar",
	} {
		if err := d.RemoveRemote(context.Background(), &session{}, bad); err == nil {
			t.Errorf("expected refusal for %q", bad)
		}
	}
}
