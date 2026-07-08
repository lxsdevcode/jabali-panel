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

func TestPHPSelectDefault(t *testing.T) {
	f := newConfigFields("h.example.com")
	f[1].input.SetValue("a@b.com")
	env := configEnv(f, false)
	var got string
	for _, e := range env {
		if len(e) >= 20 && e[:20] == "JABALI_PHP_VERSIONS=" {
			got = e[20:]
		}
	}
	if got != "8.4" {
		t.Errorf("default PHP env = %q, want 8.4", got)
	}
}

func TestConfigEnv_NSAlwaysPresent(t *testing.T) {
	f := newConfigFields("h.example.com")
	f[1].input.SetValue("a@b.com")
	f[2].input.SetValue("ns1.example.com")
	f[3].input.SetValue("ns2.example.com")
	// NS fields are no longer dns-gated — always emitted.
	env := configEnv(f, false)
	var ns1, ns2 bool
	for _, e := range env {
		if e == "JABALI_NS1_NAME=ns1.example.com" {
			ns1 = true
		}
		if e == "JABALI_NS2_NAME=ns2.example.com" {
			ns2 = true
		}
	}
	if !ns1 || !ns2 {
		t.Errorf("NS env missing when dns off: %v", env)
	}
}
