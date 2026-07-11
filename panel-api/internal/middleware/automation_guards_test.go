package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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

// JAB-140 follow-up: behind the unix-socket nginx proxy the real client IP is in
// X-Real-IP; the allowlist must match THAT, not the socket peer. Here the request
// "arrives" from 192.0.2.1 (httptest default) but X-Real-IP says 203.0.113.5, and
// the allowlist covers 203.0.113.0/24 → allowed.
func TestRequireAutomationHMAC_IPAllowlistUsesXRealIP(t *testing.T) {
	repo := &fakeAutoTokenRepo{tokens: map[string]*models.AutomationToken{}}
	k := newTestKey(t)
	id, secret := mintTestToken(t, repo, k, models.AutomationScopes{"read:*"})
	repo.tokens[id].IPAllowlist = models.CIDRList{"203.0.113.0/24"}
	r := setupHMACRouter(repo, k)

	ts := fmt.Sprintf("%d", time.Now().Unix())
	sig := signRequest(secret, "GET", "/p", ts, "")
	req := httptest.NewRequest(http.MethodGet, "/p", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Jabali-HMAC kid=%s, ts=%s, sig=%s", id, ts, sig))
	req.Header.Set("X-Real-IP", "203.0.113.5")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("X-Real-IP in allowlist: want 200, got %d", w.Code)
	}

	// A forwarded IP outside the allowlist is rejected even if the socket peer would pass.
	req2 := httptest.NewRequest(http.MethodGet, "/p", nil)
	sig2 := signRequest(secret, "GET", "/p", ts, "")
	req2.Header.Set("Authorization", fmt.Sprintf("Jabali-HMAC kid=%s, ts=%s, sig=%s", id, ts, sig2))
	req2.Header.Set("X-Real-IP", "10.9.9.9")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("X-Real-IP outside allowlist: want 401, got %d", w2.Code)
	}
}
