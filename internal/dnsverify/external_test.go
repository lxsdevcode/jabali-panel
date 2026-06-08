package dnsverify

import (
	"context"
	"testing"
	"time"
)

// Smoke: a well-known publicly resolvable host returns at least one
// address. Doubles as a guard against accidental localhost-fallback.
func TestLookupHostExternal_KnownPublicName(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	addrs := LookupHostExternal(ctx, "one.one.one.one")
	if len(addrs) == 0 {
		t.Fatalf("expected at least one A/AAAA record for one.one.one.one; got none")
	}
	// At minimum the canonical 1.1.1.1 should be present.
	found := false
	for _, a := range addrs {
		if a == "1.1.1.1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 1.1.1.1 in addrs for one.one.one.one; got %v", addrs)
	}
}

// Smoke: a name that has no public DNS at all returns nil. This is the
// key guarantee — it's how resolvableSANs decides to skip a SAN.
func TestLookupHostExternal_NoPublicRecords(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	// invalid TLD; will NXDOMAIN at every public resolver.
	addrs := LookupHostExternal(ctx, "definitely-not-a-real-host.invalid")
	if len(addrs) != 0 {
		t.Errorf("expected nil for .invalid; got %v", addrs)
	}
}
