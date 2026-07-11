package middleware

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

// JAB-140: an expired token is rejected on every request (401), read or write.
func TestRequireAutomationHMAC_RejectsExpiredToken(t *testing.T) {
	repo := &fakeAutoTokenRepo{tokens: map[string]*models.AutomationToken{}}
	k := newTestKey(t)
	id, secret := mintTestToken(t, repo, k, models.AutomationScopes{"read:*"})
	past := time.Now().Add(-time.Minute)
	repo.tokens[id].ExpiresAt = &past
	r := setupHMACRouter(repo, k)

	ts := fmt.Sprintf("%d", time.Now().Unix())
	if code := doSignedRequest(t, r, id, secret, ts, "/p"); code != http.StatusUnauthorized {
		t.Fatalf("expired token: want 401, got %d", code)
	}
}

// A future expiry still works.
func TestRequireAutomationHMAC_AllowsUnexpiredToken(t *testing.T) {
	repo := &fakeAutoTokenRepo{tokens: map[string]*models.AutomationToken{}}
	k := newTestKey(t)
	id, secret := mintTestToken(t, repo, k, models.AutomationScopes{"read:*"})
	future := time.Now().Add(time.Hour)
	repo.tokens[id].ExpiresAt = &future
	r := setupHMACRouter(repo, k)

	ts := fmt.Sprintf("%d", time.Now().Unix())
	if code := doSignedRequest(t, r, id, secret, ts, "/p"); code != http.StatusOK {
		t.Fatalf("unexpired token: want 200, got %d", code)
	}
}

// IP allowlist: httptest requests come from 192.0.2.1 — a matching CIDR passes,
// a non-matching one is rejected (401).
func TestRequireAutomationHMAC_IPAllowlist(t *testing.T) {
	k := newTestKey(t)

	// Allowed CIDR covers the test client → 200.
	repoOK := &fakeAutoTokenRepo{tokens: map[string]*models.AutomationToken{}}
	id, secret := mintTestToken(t, repoOK, k, models.AutomationScopes{"read:*"})
	repoOK.tokens[id].IPAllowlist = models.CIDRList{"192.0.2.0/24"}
	ts := fmt.Sprintf("%d", time.Now().Unix())
	if code := doSignedRequest(t, setupHMACRouter(repoOK, k), id, secret, ts, "/p"); code != http.StatusOK {
		t.Fatalf("allowed IP: want 200, got %d", code)
	}

	// Non-matching CIDR → 401.
	repoNo := &fakeAutoTokenRepo{tokens: map[string]*models.AutomationToken{}}
	id2, secret2 := mintTestToken(t, repoNo, k, models.AutomationScopes{"read:*"})
	repoNo.tokens[id2].IPAllowlist = models.CIDRList{"10.0.0.0/8"}
	if code := doSignedRequest(t, setupHMACRouter(repoNo, k), id2, secret2, ts, "/p"); code != http.StatusUnauthorized {
		t.Fatalf("blocked IP: want 401, got %d", code)
	}
}
