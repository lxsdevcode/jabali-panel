package eventsources

import "testing"

// GH: 57 false service.down alarms fired during an AppArmor enforce-flip when
// `systemctl is-active` transiently returned EMPTY for every monitored unit.
// An empty (undeterminable) state must NOT be treated as an outage.
func TestServiceStateNotDown(t *testing.T) {
	notDown := []string{"", "active", "activating", "deactivating", "reloading"}
	for _, s := range notDown {
		if !serviceStateNotDown(s) {
			t.Errorf("state %q should be treated as not-down (skip), but was flagged as down", s)
		}
	}
	down := []string{"inactive", "failed", "not-found"}
	for _, s := range down {
		if serviceStateNotDown(s) {
			t.Errorf("state %q is a real outage signal and must not be skipped", s)
		}
	}
}
