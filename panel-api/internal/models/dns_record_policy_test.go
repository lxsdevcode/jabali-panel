package models

import (
	"database/sql/driver"
	"testing"
)

func TestDNSPolicyAllows_FailClosed(t *testing.T) {
	// NS/SOA always denied regardless of stored perms.
	full := DefaultDNSUserRecordPolicy()
	full["NS"] = RecordPerm{Create: true, Edit: true, Delete: true}
	full["SOA"] = RecordPerm{Create: true, Edit: true, Delete: true}
	for _, t2 := range []string{"NS", "SOA", "ns", "soa"} {
		for _, op := range []string{"create", "edit", "delete"} {
			if full.Allows(t2, op) {
				t.Errorf("%s/%s must always be denied", t2, op)
			}
		}
	}
	// Unknown type key denied.
	if full.Allows("ANANME", "create") {
		t.Error("unknown type must be denied")
	}
	// Unknown op denied.
	if full.Allows("A", "frobnicate") {
		t.Error("unknown op must be denied")
	}
	// Case-insensitive type match.
	if !full.Allows("a", "create") {
		t.Error("type match should be case-insensitive")
	}
}

func TestDNSPolicyDefaultPermissive(t *testing.T) {
	p := DefaultDNSUserRecordPolicy()
	for _, ty := range UserManageableDNSTypes {
		for _, op := range []string{"create", "edit", "delete"} {
			if !p.Allows(ty, op) {
				t.Errorf("default should permit %s/%s", ty, op)
			}
		}
	}
}

func TestDNSPolicyPresets(t *testing.T) {
	if _, ok := DNSPolicyPreset("nope"); ok {
		t.Error("unknown preset must return ok=false")
	}
	hs, _ := DNSPolicyPreset("hosting-safe")
	if hs.Allows("CAA", "create") || hs.Allows("CAA", "edit") || hs.Allows("CAA", "delete") {
		t.Error("hosting-safe must lock CAA")
	}
	if !hs.Allows("A", "create") || !hs.Allows("MX", "edit") {
		t.Error("hosting-safe must allow A/MX")
	}
	ld, _ := DNSPolicyPreset("locked-down")
	for _, ty := range []string{"MX", "TXT", "SRV", "CAA"} {
		if ld.Allows(ty, "create") {
			t.Errorf("locked-down must deny %s", ty)
		}
	}
	for _, ty := range []string{"A", "AAAA", "CNAME"} {
		if !ld.Allows(ty, "create") {
			t.Errorf("locked-down must allow %s", ty)
		}
	}
}

func TestDNSPolicySanitizeDropsAdminTypes(t *testing.T) {
	p := DNSUserRecordPolicy{
		"a":   {Create: true},
		"NS":  {Create: true, Edit: true, Delete: true},
		"SOA": {Create: true},
		"BOGUS": {Create: true},
		"MX":  {Edit: true},
	}
	out := p.Sanitize()
	if _, ok := out["NS"]; ok {
		t.Error("Sanitize must drop NS")
	}
	if _, ok := out["SOA"]; ok {
		t.Error("Sanitize must drop SOA")
	}
	if _, ok := out["BOGUS"]; ok {
		t.Error("Sanitize must drop unknown types")
	}
	if _, ok := out["A"]; !ok {
		t.Error("Sanitize must upper-case and keep A")
	}
	if _, ok := out["MX"]; !ok {
		t.Error("Sanitize must keep MX")
	}
}

func TestDNSPolicyValueScanRoundTrip(t *testing.T) {
	p := DNSPolicyPresetMust(t, "locked-down")
	v, err := p.Value()
	if err != nil {
		t.Fatal(err)
	}
	var back DNSUserRecordPolicy
	if err := back.Scan(v.(driver.Value)); err != nil {
		t.Fatal(err)
	}
	if back.Allows("MX", "create") || !back.Allows("A", "create") {
		t.Errorf("round-trip lost policy: %+v", back)
	}
	// nil scan → empty, not error.
	var empty DNSUserRecordPolicy
	if err := empty.Scan(nil); err != nil {
		t.Fatalf("nil scan should not error: %v", err)
	}
}

func DNSPolicyPresetMust(t *testing.T, name string) DNSUserRecordPolicy {
	t.Helper()
	p, ok := DNSPolicyPreset(name)
	if !ok {
		t.Fatalf("preset %q not found", name)
	}
	return p
}
