// Package webmailsso mints HS256 JWTs for Bulwark's
// GET /api/auth/impersonate?token=<jwt> endpoint (Bulwark 1.7.1+).
//
// Wire contract (extracted from bulwarkmail/webmail
// lib/impersonation/jwt.ts):
//
//   - alg: HS256
//   - typ: JWT
//   - claims:
//   - iss (string, required) — must match BULWARK_JWT_AUTH_ISSUER
//   - iat (number, required) — issued-at (unix seconds)
//   - exp (number, required) — expiry; max lifetime 300s
//   - nbf (number, optional) — not-before
//   - jti (string, required) — unique token id; replay-protected
//     server-side by Bulwark
//   - mailbox (string, required) — target mailbox email
//   - tenant_id (string, optional) — logged for audit
//   - actor_user_id (string, optional) — logged for audit
//
//   - secret: ≥32 chars (MIN_SECRET_LENGTH)
//
// Bulwark issue #296 (closed by maintainer with this endpoint) is the
// reason this exists: the prior /api/auth/session SSO path was both
// gate-blocked (serverUrl trust list) AND broken at the SPA layer
// (cookie ↔ localStorage drift). Impersonate uses Stalwart's
// master-user `target%master` Basic header, sets the correct
// IMPERSONATION_SLOT cookies, and 303s to /. See
// plans/m6.6-webmail-impersonate-sso.md for the full design.
package webmailsso

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// MinSecretLength mirrors Bulwark's lib/impersonation/jwt.ts:
//
//	const MIN_SECRET_LENGTH = 32;
//
// Shorter secrets are rejected at Minter construction so we fail
// fast at startup instead of producing tokens Bulwark refuses.
const MinSecretLength = 32

// MaxLifetime mirrors Bulwark's MAX_TOKEN_LIFETIME_SEC (300s).
// Tokens minted with `lifetime > MaxLifetime` are clamped down at
// the boundary so callers can't accidentally widen the window.
const MaxLifetime = 5 * time.Minute

// DefaultIssuer matches our Bulwark env BULWARK_JWT_AUTH_ISSUER —
// Bulwark's expectedIssuer falls back to "platform-api/webmail" if
// the env is unset, so we use a Jabali-specific issuer string to
// rule out cross-tenant impersonation if the same Bulwark ever ends
// up shared between two panels (today: not a thing; future-proof).
const DefaultIssuer = "jabali-panel/webmail-sso"

// Minter builds + signs HS256 JWTs for one issuer + key pair.
//
// Created once at server boot (load secret from
// /etc/jabali-panel/bulwark-jwt-auth.secret, pass into webmail_sso
// handler config). Safe for concurrent use.
type Minter struct {
	secret []byte
	issuer string
	now    func() time.Time // overridable for tests
}

// New returns a configured Minter or an error if secret < MinSecretLength.
// Empty issuer falls back to DefaultIssuer.
func New(secret []byte, issuer string) (*Minter, error) {
	if len(secret) < MinSecretLength {
		return nil, fmt.Errorf("webmailsso: secret too short: %d (need >= %d)",
			len(secret), MinSecretLength)
	}
	if issuer == "" {
		issuer = DefaultIssuer
	}
	return &Minter{secret: secret, issuer: issuer, now: time.Now}, nil
}

// MintInput is the per-call payload. `Mailbox` is the only required
// field; `JTI` must be unique per call (ULID is fine — Bulwark only
// uses it for replay logging, no monotonicity required). `Lifetime`
// is clamped to MaxLifetime.
type MintInput struct {
	Mailbox     string
	JTI         string
	Lifetime    time.Duration
	TenantID    string // optional, audit-only
	ActorUserID string // optional, audit-only
}

// Mint builds + HS256-signs a JWT for input. Errors only on bad input
// (missing mailbox / JTI, marshalling failure) — crypto failures
// cannot happen with the std-lib HMAC.
func (m *Minter) Mint(in MintInput) (string, error) {
	if in.Mailbox == "" {
		return "", errors.New("webmailsso: mailbox is required")
	}
	if in.JTI == "" {
		return "", errors.New("webmailsso: jti is required")
	}
	lifetime := in.Lifetime
	if lifetime <= 0 || lifetime > MaxLifetime {
		lifetime = MaxLifetime
	}

	now := m.now().UTC()
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	claims := map[string]any{
		"iss":     m.issuer,
		"iat":     now.Unix(),
		"exp":     now.Add(lifetime).Unix(),
		"jti":     in.JTI,
		"mailbox": in.Mailbox,
	}
	if in.TenantID != "" {
		claims["tenant_id"] = in.TenantID
	}
	if in.ActorUserID != "" {
		claims["actor_user_id"] = in.ActorUserID
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("webmailsso: marshal header: %w", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("webmailsso: marshal claims: %w", err)
	}

	signingInput := base64URLEncode(headerJSON) + "." + base64URLEncode(claimsJSON)
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(signingInput))
	sig := mac.Sum(nil)
	return signingInput + "." + base64URLEncode(sig), nil
}

// base64URLEncode is the JWT standard's base64url (RFC 7519 §3) —
// strip the trailing '=' padding so the token survives URL params.
func base64URLEncode(in []byte) string {
	return base64.RawURLEncoding.EncodeToString(in)
}
