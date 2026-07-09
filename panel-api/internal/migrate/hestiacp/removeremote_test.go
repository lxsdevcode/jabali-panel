package hestiacp

import (
	"context"
	"testing"
)

// JAB-50: RemoveRemote must refuse paths outside /backup/<account>.*.
func TestRemoveRemote_Allowlist(t *testing.T) {
	d := New()
	for _, bad := range []string{
		"/etc/passwd", "/backup/victim.tar", "/tmp/alice.tar",
		"/backup/../etc/shadow", "/backup/alice.tar/../../x",
	} {
		if err := d.RemoveRemote(context.Background(), &session{}, "alice", bad); err == nil {
			t.Errorf("expected refusal for %q", bad)
		}
	}
}
