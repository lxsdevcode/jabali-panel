package tui

import "testing"

func TestValidateConfig(t *testing.T) {
	f := newConfigFields("")
	// all required empty except seeded php → hostname missing
	if e := validateConfig(f, false); e == "" {
		t.Error("expected error for empty required hostname")
	}
	f[0].input.SetValue("panel.example.com")
	f[1].input.SetValue("admin@example.com")
	if e := validateConfig(f, false); e != "" {
		t.Errorf("valid config rejected: %s", e)
	}
	f[0].input.SetValue("not a hostname")
	if e := validateConfig(f, false); e == "" {
		t.Error("bad hostname accepted")
	}
}

func TestConfigEnv_DNSGating(t *testing.T) {
	f := newConfigFields("h.example.com")
	f[1].input.SetValue("a@b.com")
	f[2].input.SetValue("ns1.example.com")
	f[3].input.SetValue("ns2.example.com")
	// dns off → NS fields excluded
	env := configEnv(f, false)
	for _, e := range env {
		if e[:9] == "JABALI_NS" {
			t.Errorf("NS env leaked with dns off: %s", e)
		}
	}
	// dns on → NS included
	envOn := configEnv(f, true)
	var hasNS bool
	for _, e := range envOn {
		if len(e) >= 9 && e[:9] == "JABALI_NS" {
			hasNS = true
		}
	}
	if !hasNS {
		t.Error("NS env missing with dns on")
	}
}
