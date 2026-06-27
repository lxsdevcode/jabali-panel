package migrate

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"testing"

	"golang.org/x/crypto/ssh"
)

func testHostKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return signer.PublicKey()
}

// TestPinningHostKeyCallback covers the in-memory host-key verification (Gitea
// #461): no fingerprint => trust on first use (accept); a matching fingerprint
// => accept; a non-matching fingerprint => reject before any credential is sent.
func TestPinningHostKeyCallback(t *testing.T) {
	addr := &net.TCPAddr{IP: net.ParseIP("203.0.113.5"), Port: 22}
	host := "203.0.113.5:22"
	key := testHostKey(t)
	fp := ssh.FingerprintSHA256(key)

	// No fingerprint -> TOFU accept.
	if err := PinningHostKeyCallback("")(host, addr, key); err != nil {
		t.Fatalf("TOFU should accept, got %v", err)
	}

	// Correct fingerprint -> accept (both with and without the SHA256: prefix).
	if err := PinningHostKeyCallback(fp)(host, addr, key); err != nil {
		t.Fatalf("matching fingerprint should accept, got %v", err)
	}
	bare := fp[len("SHA256:"):]
	if err := PinningHostKeyCallback(bare)(host, addr, key); err != nil {
		t.Fatalf("matching bare fingerprint should accept, got %v", err)
	}

	// Wrong fingerprint -> reject, even for a real key.
	if err := PinningHostKeyCallback("SHA256:AAAAdefinitelynottherightfingerprint")(host, addr, key); err == nil {
		t.Fatal("a non-matching fingerprint must be rejected")
	}

	// A different real key against a pinned fingerprint -> reject.
	if err := PinningHostKeyCallback(fp)(host, addr, testHostKey(t)); err == nil {
		t.Fatal("a different key must fail the pinned fingerprint")
	}
}
