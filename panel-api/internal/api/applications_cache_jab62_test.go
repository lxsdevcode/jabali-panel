package api

import (
	"strings"
	"testing"
)

// JAB-62: prove per-install ISSUANCE — distinct ACL user, per-install fence, and
// distinct token per install, plus that the agent derives the SAME username.
//
// This does NOT prove Redis ENFORCEMENT (A's credential is denied B's keys) —
// that is Redis's ACL engine, exercised by the live-Redis smoke, not a unit test.
func TestJAB62_PerInstallACLIssuance(t *testing.T) {
	// Two installs of the SAME os user must get distinct ACL users + fences.
	uA := installACLUser("alice", "01AAAAAAAAAAAAAAAAAAAAAAAA")
	uB := installACLUser("alice", "01BBBBBBBBBBBBBBBBBBBBBBBB")
	if uA == uB {
		t.Fatal("same-osUser installs must get distinct per-install ACL users")
	}
	if uA != "wp_alice_01AAAAAAAAAAAAAAAAAAAAAAAA" {
		t.Fatalf("unexpected ACL user %q", uA)
	}

	fA := installACLKeyPattern("alice", "01AAAAAAAAAAAAAAAAAAAAAAAA")
	if fA != "~jc:alice:01AAAAAAAAAAAAAAAAAAAAAAAA:*" {
		t.Fatalf("unexpected fence %q", fA)
	}
	// The fence MUST be per-install, never the broad ~jc:<osUser>:* hole.
	if fA == "~jc:alice:*" || !strings.Contains(fA, "01AAAAAAAAAAAAAAAAAAAAAAAA") {
		t.Fatalf("fence %q is not scoped to the install", fA)
	}

	// The agent derives the ACL username from p.Prefix ("<osuser>:<installID>").
	// It MUST equal the panel-provisioned name or the bundled cache can't auth.
	agentUser := "wp_" + strings.ReplaceAll("alice:01AAAAAAAAAAAAAAAAAAAAAAAA", ":", "_")
	if agentUser != uA {
		t.Fatalf("agent-derived %q != panel-provisioned %q — plugin would fail to auth", agentUser, uA)
	}

	// Per-install tokens differ; derivation is stable.
	tA := cacheInstallToken("secret", "alice", "01AAAAAAAAAAAAAAAAAAAAAAAA", "salt")
	tB := cacheInstallToken("secret", "alice", "01BBBBBBBBBBBBBBBBBBBBBBBB", "salt")
	if tA == tB {
		t.Fatal("per-install tokens must differ between sibling installs")
	}
	if tA != cacheInstallToken("secret", "alice", "01AAAAAAAAAAAAAAAAAAAAAAAA", "salt") {
		t.Fatal("token derivation must be stable for the same inputs")
	}
}
