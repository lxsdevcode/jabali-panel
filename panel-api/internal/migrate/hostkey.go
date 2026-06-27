package migrate

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SSH host-key pinning for migration connectors + the rsync restore stage
// (Gitea #461, ADR-0151).
//
// The connectors previously used ssh.InsecureIgnoreHostKey(), so a network
// MITM able to intercept routing to the source host could impersonate it and
// harvest the supplied SSH password / key-auth attempt, or feed a malicious
// restore. We replace that with a pinning callback:
//
//   - If an expected fingerprint is supplied (admin pasted it out-of-band),
//     the presented key MUST match it — strong verification, fail-hard on
//     mismatch.
//   - Otherwise trust-on-first-use: the first connect captures + pins the key
//     to a per-job known_hosts file; every later connect (and the agent's rsync
//     restore stage, which reads the same file) must present the SAME key, so a
//     mid-migration key flip is rejected. First-connect MITM is unverified —
//     callers warn the operator and offer the fingerprint path.

// KnownHostsPath is the per-job pinned known_hosts file, a sibling of the job's
// secret env-file so it shares the same root:jabali 0750 dir and lifecycle.
func KnownHostsPath(jobID string) string {
	return filepath.Join(SecretsDir, jobID+".known_hosts")
}

// JobIDFromSecretPath recovers the job id from a SecretRef.Path
// (".../<jobID>.env").
func JobIDFromSecretPath(secretPath string) string {
	return strings.TrimSuffix(filepath.Base(secretPath), ".env")
}

// PinningHostKeyCallback returns an ssh.HostKeyCallback that verifies the
// source host key and pins it to knownHostsPath. expectedFP (optional) is an
// admin-supplied SHA256 fingerprint ("SHA256:…" or the bare base64 body); when
// set the presented key must match it. Otherwise TOFU applies.
func PinningHostKeyCallback(knownHostsPath, expectedFP string) (ssh.HostKeyCallback, error) {
	// Ensure the file exists so knownhosts.New can parse it (empty = no pins).
	f, err := os.OpenFile(knownHostsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, fmt.Errorf("known_hosts %s: %w", knownHostsPath, err)
	}
	_ = f.Close()

	verify, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("load known_hosts %s: %w", knownHostsPath, err)
	}

	expected := normalizeFingerprint(expectedFP)

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if expected != "" {
			got := normalizeFingerprint(ssh.FingerprintSHA256(key))
			if got != expected {
				return fmt.Errorf("host key fingerprint mismatch for %s: got %s, expected %s", hostname, ssh.FingerprintSHA256(key), expectedFP)
			}
			// Verified against the admin fingerprint — pin so the rsync stage
			// (which has no fingerprint, only the file) reuses the same key.
			return appendKnownHost(knownHostsPath, hostname, key)
		}

		// TOFU. knownhosts verifies against the pinned file; a mismatch is a
		// hard reject, an unknown host is captured.
		verr := verify(hostname, remote, key)
		if verr == nil {
			return nil
		}
		var ke *knownhosts.KeyError
		if errors.As(verr, &ke) && len(ke.Want) == 0 {
			// Host not yet pinned → trust on first use.
			return appendKnownHost(knownHostsPath, hostname, key)
		}
		// Known host with a different key, or a revoked key → reject.
		return verr
	}, nil
}

// appendKnownHost writes a pinned line for hostname/key in OpenSSH known_hosts
// format so the agent's `ssh -o UserKnownHostsFile` consumes it directly.
func appendKnownHost(path, hostname string, key ssh.PublicKey) error {
	line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("pin host key: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		return fmt.Errorf("pin host key: %w", err)
	}
	return nil
}

// normalizeFingerprint canonicalizes a SHA256 fingerprint for comparison:
// trims, strips an optional "SHA256:" prefix, and drops trailing base64 "=".
func normalizeFingerprint(fp string) string {
	s := strings.TrimSpace(fp)
	s = strings.TrimPrefix(s, "SHA256:")
	s = strings.TrimRight(s, "=")
	return s
}
