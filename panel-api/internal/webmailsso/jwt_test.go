package webmailsso

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNew_RejectsShortSecret(t *testing.T) {
	if _, err := New([]byte("too-short"), "iss"); err == nil {
		t.Fatal("expected error for short secret")
	}
	exactly31 := make([]byte, MinSecretLength-1)
	if _, err := New(exactly31, "iss"); err == nil {
		t.Fatal("expected error for 31-byte secret")
	}
	exactly32 := make([]byte, MinSecretLength)
	if _, err := New(exactly32, "iss"); err != nil {
		t.Fatalf("32-byte secret rejected: %v", err)
	}
}

func TestNew_DefaultsIssuer(t *testing.T) {
	secret := make([]byte, MinSecretLength)
	m, err := New(secret, "")
	if err != nil {
		t.Fatal(err)
	}
	if m.issuer != DefaultIssuer {
		t.Errorf("empty issuer should default to %q; got %q", DefaultIssuer, m.issuer)
	}
}

func TestMint_RejectsMissingMailbox(t *testing.T) {
	m := testMinter(t)
	_, err := m.Mint(MintInput{JTI: "01J"})
	if err == nil {
		t.Fatal("expected error for missing mailbox")
	}
}

func TestMint_RejectsMissingJTI(t *testing.T) {
	m := testMinter(t)
	_, err := m.Mint(MintInput{Mailbox: "alice@example.com"})
	if err == nil {
		t.Fatal("expected error for missing jti")
	}
}

func TestMint_ProducesVerifiableJWT(t *testing.T) {
	m := testMinter(t)
	fixedTime := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return fixedTime }

	tok, err := m.Mint(MintInput{
		Mailbox:     "alice@example.com",
		JTI:         "01JJABCDEFG",
		Lifetime:    60 * time.Second,
		TenantID:    "tenant-1",
		ActorUserID: "01JADMIN001",
	})
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT segments; got %d (%q)", len(parts), tok)
	}

	// Header.
	hdr := decodeSegment(t, parts[0])
	if hdr["alg"] != "HS256" {
		t.Errorf("alg = %v; want HS256", hdr["alg"])
	}
	if hdr["typ"] != "JWT" {
		t.Errorf("typ = %v; want JWT", hdr["typ"])
	}

	// Claims.
	claims := decodeSegment(t, parts[1])
	if claims["mailbox"] != "alice@example.com" {
		t.Errorf("mailbox = %v", claims["mailbox"])
	}
	if claims["jti"] != "01JJABCDEFG" {
		t.Errorf("jti = %v", claims["jti"])
	}
	if claims["iss"] != "test-issuer" {
		t.Errorf("iss = %v", claims["iss"])
	}
	if iat, ok := claims["iat"].(float64); !ok || int64(iat) != fixedTime.Unix() {
		t.Errorf("iat = %v; want %d", claims["iat"], fixedTime.Unix())
	}
	if exp, ok := claims["exp"].(float64); !ok || int64(exp) != fixedTime.Add(60*time.Second).Unix() {
		t.Errorf("exp = %v; want %d", claims["exp"], fixedTime.Add(60*time.Second).Unix())
	}
	if claims["tenant_id"] != "tenant-1" {
		t.Errorf("tenant_id = %v", claims["tenant_id"])
	}
	if claims["actor_user_id"] != "01JADMIN001" {
		t.Errorf("actor_user_id = %v", claims["actor_user_id"])
	}

	// Signature verifies against same secret.
	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(strings.Repeat("k", MinSecretLength)))
	mac.Write([]byte(signingInput))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if parts[2] != expectedSig {
		t.Errorf("signature mismatch\n got: %s\nwant: %s", parts[2], expectedSig)
	}
}

func TestMint_OmitsEmptyOptionalClaims(t *testing.T) {
	m := testMinter(t)
	tok, err := m.Mint(MintInput{
		Mailbox: "bob@example.com",
		JTI:     "01JBJK",
	})
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(tok, ".")
	claims := decodeSegment(t, parts[1])
	if _, ok := claims["tenant_id"]; ok {
		t.Error("tenant_id should be omitted when empty")
	}
	if _, ok := claims["actor_user_id"]; ok {
		t.Error("actor_user_id should be omitted when empty")
	}
	if _, ok := claims["nbf"]; ok {
		t.Error("nbf should be omitted (never set today)")
	}
}

func TestMint_ClampsLifetimeToMax(t *testing.T) {
	m := testMinter(t)
	fixedTime := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return fixedTime }

	tok, err := m.Mint(MintInput{
		Mailbox:  "alice@example.com",
		JTI:     "01JABC",
		Lifetime: 1 * time.Hour, // way over MaxLifetime
	})
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(tok, ".")
	claims := decodeSegment(t, parts[1])
	expected := fixedTime.Add(MaxLifetime).Unix()
	if exp, ok := claims["exp"].(float64); !ok || int64(exp) != expected {
		t.Errorf("exp = %v; want %d (clamped to MaxLifetime)", claims["exp"], expected)
	}
}

func TestMint_ClampsZeroLifetimeToMax(t *testing.T) {
	m := testMinter(t)
	tok, err := m.Mint(MintInput{
		Mailbox:  "alice@example.com",
		JTI:     "01JABC",
		Lifetime: 0, // unset
	})
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(tok, ".")
	claims := decodeSegment(t, parts[1])
	iat := int64(claims["iat"].(float64))
	exp := int64(claims["exp"].(float64))
	if exp-iat != int64(MaxLifetime.Seconds()) {
		t.Errorf("exp-iat = %d; want %d (MaxLifetime default)", exp-iat, int64(MaxLifetime.Seconds()))
	}
}

// --- helpers ---

func testMinter(t *testing.T) *Minter {
	t.Helper()
	secret := []byte(strings.Repeat("k", MinSecretLength))
	m, err := New(secret, "test-issuer")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func decodeSegment(t *testing.T, seg string) map[string]any {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		t.Fatalf("decode segment %q: %v", seg, err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal segment: %v", err)
	}
	return out
}
