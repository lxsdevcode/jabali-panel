package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFPMWorkerStatus_CountsPerUserNotMaskedGlobal(t *testing.T) {
	dir := t.TempDir()
	// alice + bob pinned to 8.4, carol to 8.3
	for u, v := range map[string]string{"alice": "8.4", "bob": "8.4", "carol": "8.3"} {
		if err := os.WriteFile(filepath.Join(dir, u), []byte(v+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	origDir, origActive, origPinned := userPhpverDir, fpmInstanceActive, userPinnedToVersion
	defer func() { userPhpverDir, fpmInstanceActive, userPinnedToVersion = origDir, origActive, origPinned }()
	userPhpverDir = dir
	userPinnedToVersion = func(user, version string) bool {
		b, err := os.ReadFile(filepath.Join(dir, user))
		return err == nil && string(b) == version+"\n"
	}
	// alice active, bob stopped
	fpmInstanceActive = func(user string) bool { return user == "alice" }

	running, total := fpmWorkerStatus("8.4")
	if total != 2 {
		t.Errorf("8.4 total = %d, want 2 (alice+bob)", total)
	}
	if running != 1 {
		t.Errorf("8.4 running = %d, want 1 (alice)", running)
	}
	if !checkFPMRunning("8.4") {
		t.Error("checkFPMRunning(8.4) should be true (alice active)")
	}

	r83, t83 := fpmWorkerStatus("8.3")
	if t83 != 1 || r83 != 0 {
		t.Errorf("8.3 = %d/%d, want 0/1 (carol pinned, stopped)", r83, t83)
	}
	// version with no pinned users
	if r, tot := fpmWorkerStatus("7.4"); r != 0 || tot != 0 {
		t.Errorf("7.4 = %d/%d, want 0/0", r, tot)
	}
}
