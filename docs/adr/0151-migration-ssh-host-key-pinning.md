# ADR-0151: SSH host-key pinning for migration connectors and rsync restore

**Status:** Accepted (2026-06-27) — backend core shipped.
**Driven by:** Gitea #461.
**Touches:** ADR-0094 (migration secrets dir).

---

## Context

The cPanel / DirectAdmin / HestiaCP migration connectors dialed the source over
SSH with `ssh.InsecureIgnoreHostKey()`, and the DirectAdmin rsync restore stage
used `StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null`. A network
attacker able to intercept routing/DNS to the source could impersonate it,
harvest the supplied SSH password / private-key auth attempt, and feed a
malicious restore into the destination account (the migration user is typically
root/admin on the source).

Migrations are often effectively one-shot, so naive trust-on-first-use (TOFU)
gives weak protection against an attacker present from the very first packet:
the first connect would simply pin the attacker's key. The mitigation must
therefore do more than a blind `HostKeyCallback` swap.

## Decision

Pin the source host key per job and verify it on every connection, with an
optional strong-verification path:

1. **Per-job pinned `known_hosts`** at `<SecretsDir>/<jobID>.known_hosts`
   (sibling of the secret env-file; same `root:jabali 0750` dir + lifecycle,
   removed by `WipeJobSecret`).
2. **Connectors** (`panel-api`, all three) use a shared
   `PinningHostKeyCallback`:
   - **Admin fingerprint supplied** (`SecretRef.ExpectedHostKey`, a SHA256
     fingerprint pasted out-of-band): the presented key MUST match it —
     fail-hard on mismatch. Strongest; closes the first-connect window.
   - **Otherwise TOFU**: the first connect captures + pins the key; every later
     connect must present the same key (a mid-migration key flip is rejected).
3. **Agent rsync restore stage** reads the same pinned file with
   `StrictHostKeyChecking=accept-new -o UserKnownHostsFile=<jobID>.known_hosts`,
   so the key pinned at discover is verified for the long transfer (the most
   exposed window), and an unpinned host is captured rather than blindly
   trusted.

This means the realistic threat — a MITM that appears between the short discover
connect and the long rsync transfer, or that flips the key mid-run — is now
rejected, and an admin who verifies the fingerprint out-of-band gets full
protection including the first connect.

## Consequences

- `ssh.InsecureIgnoreHostKey()` is gone from the migration path; the
  `/dev/null` known_hosts is gone from rsync.
- Default behavior is TOFU (no operator input required); the connector still
  works on first use but the first connect is unverified — documented, and the
  fingerprint field is the opt-in for operators who want certainty.
- **Follow-up (UI):** expose `ExpectedHostKey` in the migration wizard's
  Connection step (a fingerprint input + an "unverified first connect" warning)
  and thread it through the job model → `SecretRef`. The backend (verification +
  pinning + tests) is already in place; only the operator-facing field is
  pending.
- `WipeJobSecret` now also removes the pinned `known_hosts`.
