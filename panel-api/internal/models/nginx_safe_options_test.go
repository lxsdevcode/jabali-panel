package models

import (
	"database/sql/driver"
	"strings"
	"testing"
)

func TestNginxSafeOptions_Render(t *testing.T) {
	o := NginxSafeOptions{MaxBodyMB: 64, HSTS: true, SecurityHeaders: true, Gzip: true}
	out := o.Render()
	for _, want := range []string{
		"client_max_body_size 64m;",
		"Strict-Transport-Security",
		"X-Frame-Options",
		"X-Content-Type-Options",
		"Referrer-Policy",
		"gzip on;",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("render missing %q in:\n%s", want, out)
		}
	}
	// Nothing enabled → empty.
	if (NginxSafeOptions{}).Render() != "" {
		t.Error("empty options should render nothing")
	}
	// MaxBodyMB=0 omits the directive.
	if strings.Contains((NginxSafeOptions{HSTS: true}).Render(), "client_max_body_size") {
		t.Error("max_body_mb=0 must not emit client_max_body_size")
	}
	// Rendered directives are fixed/safe — no raw injection vectors.
	full := o.Render()
	for _, bad := range []string{"proxy_pass", "root ", "alias ", "access_log", "auth_basic"} {
		if strings.Contains(full, bad) {
			t.Errorf("render must never contain %q", bad)
		}
	}
}

func TestNginxSafeOptions_Validate(t *testing.T) {
	if err := (NginxSafeOptions{MaxBodyMB: 64}).Validate(); err != nil {
		t.Errorf("64MB valid: %v", err)
	}
	if err := (NginxSafeOptions{MaxBodyMB: -1}).Validate(); err == nil {
		t.Error("negative max_body_mb must be rejected")
	}
	if err := (NginxSafeOptions{MaxBodyMB: 99999}).Validate(); err == nil {
		t.Error("over-cap max_body_mb must be rejected")
	}
}

func TestNginxSafeOptions_ValueScan(t *testing.T) {
	o := NginxSafeOptions{MaxBodyMB: 32, Gzip: true}
	v, err := o.Value()
	if err != nil {
		t.Fatal(err)
	}
	var back NginxSafeOptions
	if err := back.Scan(v.(driver.Value)); err != nil {
		t.Fatal(err)
	}
	if back.MaxBodyMB != 32 || !back.Gzip || back.HSTS {
		t.Errorf("round-trip mismatch: %+v", back)
	}
	var empty NginxSafeOptions
	if err := empty.Scan(nil); err != nil {
		t.Fatalf("nil scan: %v", err)
	}
}
