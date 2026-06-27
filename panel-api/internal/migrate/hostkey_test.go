package migrate

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"path/filepath"
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

// TestPinningHostKeyTOFU covers the trust-on-first-use path (Gitea #461): the
// first connect pins the key, a later connect with the SAME key is accepted,
// and a connect with a DIFFERENT key is rejected.
func TestPinningHostKeyTOFU(t *testing.T) {
	kh := filepath.Join(t.TempDir(), "job.known_hosts")
	addr := &net.TCPAddr{IP: net.ParseIP("203.0.113.5"), Port: 22}
	host := "203.0.113.5:22"
	key1 := testHostKey(t)

	// First connect: unknown host → captured.
	cb1, err := PinningHostKeyCallback(kh, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := cb1(host, addr, key1); err != nil {
		t.Fatalf("first connect should pin, got %v", err)
	}

	// Second connect, same key → accepted (fresh callback re-reads the file).
	cb2, err := PinningHostKeyCallback(kh, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := cb2(host, addr, key1); err != nil {
		t.Fatalf("matching key should be accepted, got %v", err)
	}

	// Third connect, different key → rejected (MITM / key flip).
	key2 := testHostKey(t)
	cb3, err := PinningHostKeyCallback(kh, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := cb3(host, addr, key2); err == nil {
		t.Fatal("a changed host key must be rejected")
	}
}

// TestPinningHostKeyFingerprint covers admin-supplied fingerprint verification.
func TestPinningHostKeyFingerprint(t *testing.T) {
	addr := &net.TCPAddr{IP: net.ParseIP("203.0.113.5"), Port: 22}
	host := "203.0.113.5:22"
	key := testHostKey(t)
	fp := ssh.FingerprintSHA256(key)

	// Correct fingerprint → accepted + pinned.
	kh := filepath.Join(t.TempDir(), "job.known_hosts")
	cb, err := PinningHostKeyCallback(kh, fp)
	if err != nil {
		t.Fatal(err)
	}
	if err := cb(host, addr, key); err != nil {
		t.Fatalf("matching fingerprint should be accepted, got %v", err)
	}

	// Wrong fingerprint → rejected, even for a different real key.
	kh2 := filepath.Join(t.TempDir(), "job.known_hosts")
	cbBad, err := PinningHostKeyCallback(kh2, "SHA256:AAAAdefinitelynottherightfingerprintvalue")
	if err != nil {
		t.Fatal(err)
	}
	if err := cbBad(host, addr, key); err == nil {
		t.Fatal("a non-matching fingerprint must be rejected")
	}
}

func TestKnownHostsPathAndJobID(t *testing.T) {
	if got := KnownHostsPath("01ABC"); got != filepath.Join(SecretsDir, "01ABC.known_hosts") {
		t.Errorf("KnownHostsPath = %q", got)
	}
	if got := JobIDFromSecretPath(filepath.Join(SecretsDir, "01ABC.env")); got != "01ABC" {
		t.Errorf("JobIDFromSecretPath = %q", got)
	}
}
