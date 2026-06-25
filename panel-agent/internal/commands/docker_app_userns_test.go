package commands

import "testing"

// GH #284: under docker userns-remap the app data tree must be chowned into the
// dockremap subuid range (bind mounts aren't auto-fixed by docker), and base==0
// (remap off) must reproduce the legacy behaviour byte-for-byte.
func TestResolveInstallOwner(t *testing.T) {
	cases := []struct {
		name        string
		volumeOwner string
		base        int
		wantUID     int
		wantGID     int
		wantApply   bool
	}{
		{"non-userns, no owner = legacy no-op", "", 0, 0, 0, false},
		{"non-userns, explicit owner unchanged", "1000:1000", 0, 1000, 1000, true},
		{"userns, no owner -> base:base (memos)", "", 100000, 100000, 100000, true},
		{"userns, explicit owner shifted", "1000:1000", 100000, 101000, 101000, true},
	}
	for _, c := range cases {
		uid, gid, apply, err := resolveInstallOwner(c.volumeOwner, c.base)
		if err != nil {
			t.Errorf("%s: unexpected err %v", c.name, err)
			continue
		}
		if uid != c.wantUID || gid != c.wantGID || apply != c.wantApply {
			t.Errorf("%s: got (%d,%d,%v) want (%d,%d,%v)", c.name, uid, gid, apply, c.wantUID, c.wantGID, c.wantApply)
		}
	}
	if _, _, _, err := resolveInstallOwner("bogus", 0); err == nil {
		t.Error("bad volume_owner should error")
	}
}

func TestParseDockremapSubuidBase(t *testing.T) {
	if got := parseDockremapSubuidBase("dockremap:100000:65536\n"); got != 100000 {
		t.Errorf("dockremap base = %d, want 100000", got)
	}
	if got := parseDockremapSubuidBase("# c\nroot:0:65536\nfoo:1000:1\n"); got != 0 {
		t.Errorf("no dockremap entry should be 0, got %d", got)
	}
	if got := parseDockremapSubuidBase(""); got != 0 {
		t.Errorf("empty should be 0, got %d", got)
	}
}
