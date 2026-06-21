package main

import "testing"

func TestMergeUsernsRemap_AddsWhenAbsent(t *testing.T) {
	in := []byte(`{"live-restore": true, "log-driver": "journald"}`)
	out, changed, err := mergeUsernsRemap(in)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("expected changed=true")
	}
	if !contains(out, `"userns-remap": "default"`) {
		t.Errorf("remap not added:\n%s", out)
	}
	// Existing keys preserved.
	if !contains(out, `"live-restore"`) || !contains(out, `"log-driver"`) {
		t.Errorf("existing keys lost:\n%s", out)
	}
}

func TestMergeUsernsRemap_NoOpWhenPresent(t *testing.T) {
	in := []byte(`{"userns-remap": "default", "live-restore": true}`)
	_, changed, err := mergeUsernsRemap(in)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("expected changed=false when remap already set")
	}
}

func TestMergeUsernsRemap_EmptyInput(t *testing.T) {
	out, changed, err := mergeUsernsRemap(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || !contains(out, `"userns-remap": "default"`) {
		t.Errorf("empty daemon.json should gain remap:\n%s", out)
	}
}

func TestParseDockremapBase(t *testing.T) {
	body := "# comment\nsomeuser:200000:65536\ndockremap:100000:65536\n"
	base, err := parseDockremapBase(body)
	if err != nil {
		t.Fatal(err)
	}
	if base != 100000 {
		t.Errorf("base = %d, want 100000", base)
	}
}

func TestParseDockremapBase_Missing(t *testing.T) {
	if _, err := parseDockremapBase("alice:100000:65536\n"); err == nil {
		t.Fatal("missing dockremap entry must error")
	}
}

func contains(b []byte, sub string) bool {
	return len(b) >= len(sub) && indexOf(string(b), sub) >= 0
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
