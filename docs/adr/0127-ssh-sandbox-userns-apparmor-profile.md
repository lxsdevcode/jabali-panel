# ADR-0127: AppArmor userns profile for the M13 SSH sandbox on userns-restricted hosts

**Date**: 2026-06-15
**Status**: Accepted
**Deciders**: shuki + Claude
**Related**: ADR-0067 (M13 SSH shell sandbox / bwrap), ADR-0086/0092 (AppArmor profiles). Surfaced while e2e-testing GH #184.

## Context

Ubuntu 24.04 (noble) ships `kernel.apparmor_restrict_unprivileged_userns=1`,
which blocks unprivileged user-namespace creation unless the creating
process is covered by an AppArmor profile that permits `userns`. `bwrap`
is not setuid, so the M13 SSH sandbox (`jabali-ssh-shell` → bwrap) fails
for every tenant with `bwrap: setting up uid map: Permission denied` —
the SSH/SFTP shell is effectively dead on Ubuntu noble hosts. Debian does
not (yet) set this restriction, which is why it was never caught:
10.0.3.14 (Debian, the primary functional test box) can't reproduce it;
it only surfaced on mx (Ubuntu 24.04).

## Decision

Ship a **scoped** AppArmor profile that grants `userns` to
`/usr/local/bin/jabali-ssh-shell` only, loaded in **enforce** mode:

```
abi <abi/4.0>,
include <tunables/global>
profile jabali-ssh-shell /usr/local/bin/jabali-ssh-shell flags=(unconfined) {
  userns,
  include if exists <local/jabali-ssh-shell>
}
```

- `flags=(unconfined)` adds **no** confinement — the bwrap sandbox is the
  real boundary; the flag only makes the binary "profiled" so the kernel
  userns check passes, while still allowing all its file/exec access. The
  exec'd `bwrap` inherits the profile and may create the namespace.
- Loaded with `apparmor_parser -r` (which defaults to enforce). **Do NOT
  `aa-enforce` it** — `aa-enforce` on a `flags=(unconfined)` profile strips
  the allow-all and turns it restrictive, after which the wrapper is denied
  its own reads (e.g. `/etc/jabali/ssh-sandbox-mode`). Complain mode does
  NOT satisfy the kernel userns check, so enforce-via-parser is the only
  working state (both verified empirically per ADR-0092).
- Installed by `install_ssh_sandbox_prereqs`, **gated on
  `kernel.apparmor_restrict_unprivileged_userns == 1`** — a no-op on Debian
  and on hosts without the restriction. Runs after `install_apparmor`, so
  `apparmor_parser` is present.

## Alternatives Considered

- **Disable the restriction host-wide** (`sysctl
  kernel.apparmor_restrict_unprivileged_userns=0`) — simplest, but
  re-opens unprivileged-userns to *every* process on the host, not just our
  sandbox. Rejected (CONVENTIONS: security over functionality).
- **Ubuntu's `bwrap-userns-restrict` profile** — Ubuntu's canonical option
  (allows bwrap userns, blocks nested namespaces), but it is not present on
  the box (`/usr/share/apparmor/extra-profiles/` lacked it after Ubuntu's
  revert for breaking Flatpak), and it grants userns to *all* bwrap callers.
  Our scoped profile is narrower.
- **Profile on `/usr/bin/bwrap`** — the common community fix, but broader
  than necessary (every bwrap invocation). Scoping to `jabali-ssh-shell`
  grants the capability only to our entry point.

## Consequences

### Positive
- The M13 SSH sandbox works on Ubuntu 24.04+ without weakening host-wide
  userns hardening or granting userns to unrelated bwrap callers.
- No-op on Debian; self-healing on `jabali update` (idempotent re-parse).

### Negative
- `flags=(unconfined)` means `jabali-ssh-shell` itself is not AppArmor-
  confined (only the bwrap sandbox confines the tenant) — matches the
  ecosystem norm (Chrome/Flatpak ship the same shape) and ADR-0067's model
  where bwrap is the boundary.
- The profile is enforce-only by design; it is intentionally excluded from
  the cautious complain→enforce rollout used for the confinement profiles.

## Implementation

- `install.sh` → `install_ssh_sandbox_prereqs`: writes
  `/etc/apparmor.d/jabali-ssh-shell` + `apparmor_parser -r`, gated on the
  sysctl.
- **Live-verified on mx.jabali-panel.com** (Ubuntu 24.04, restriction=1):
  with the profile loaded, a tenant SSHes through `jabali-ssh-shell` into
  the bwrap sandbox successfully (`whoami`, sandboxed `/`, file reads);
  without it (or in complain mode, or with `aa-enforce`), the session
  fails. Restriction left at its default `1`.
