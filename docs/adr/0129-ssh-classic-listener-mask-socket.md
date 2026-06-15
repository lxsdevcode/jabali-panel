# ADR-0129: Normalize SSH to a single classic ssh.service listener (mask ssh.socket)

**Date**: 2026-06-15
**Status**: Accepted
**Deciders**: shuki + Claude
**Related**: ADR-0028 (M12 SFTP, sshd drop-in + reload), ADR-0067 (SSH shell sandbox), ADR-0127 (SSH sandbox userns). Fixes GH #133.

## Context

Debian 13 (trixie) and Ubuntu 24.04 ship OpenSSH with **both** `ssh.service`
(classic long-lived daemon) and `ssh.socket` (socket activation) **enabled by
preset** (`enabled; preset: enabled` on both). The two units conflict on
`:22` — only one may own the port. systemd resolves the race at boot
("Socket service ssh.service already active, refusing"), so the host *appears*
fine, but it is left in a fragile hybrid: `ssh.service` is the live listener
while `ssh.socket` sits enabled-but-dead.

In that hybrid state a later `systemctl reload ssh` (SIGHUP) makes the
socket-activated sshd re-exec and try to rebind a port the enabled
`ssh.socket` still claims:

```
sshd[…]: Received SIGHUP; restarting.
sshd[…]: fatal: Cannot bind any address.
systemd[1]: ssh.service: Main process exited, code=exited, status=255/EXCEPTION
systemd[1]: ssh.service: Failed with result 'exit-code'.
```

→ `ssh.service` **failed**, `ssh.socket` inactive → **operator lockout**
(GH #133, reproduced byte-for-byte on a Debian 13 test host). jabali issues
that reload from three places: `install.sh`, `jabali update`
(`update.go`), and the panel's SSH-config change handler
(`system_set_ssh_config.go`).

Two further problems made socket activation the wrong model for jabali
specifically:

1. **The panel's port-change feature is a silent no-op under socket
   activation.** `renderGlobalDropin` writes `Port N` into a sshd drop-in,
   but the listening port is owned by `ssh.socket`'s `ListenStream`, not by
   sshd's `Port` directive. Changing the port in the panel does nothing.
2. The panel's whole SSH model (write drop-in → reload daemon) assumes a
   **long-lived, reloadable** daemon — i.e. classic `ssh.service`.

## Decision

**Converge every host onto a single classic `ssh.service` listener and
retire `ssh.socket`.** A shared, idempotent, lockout-safe script —
`install/ssh/normalize-ssh-classic.sh` — is run by **both** `install.sh`
(fresh installs) and `jabali update` (so the already-deployed fleet
self-heals without a reinstall):

1. Ensure `/run/sshd` exists, then `sshd -t` (validate config; a missing
   privsep dir is a *runtime* condition, not a config error — create it
   first so the gate tests config validity, not `/run` state).
2. `enable` (+ `unmask`) `ssh.service`.
3. `stop` + `disable` + **`mask`** `ssh.socket`. Mask (not merely disable)
   so an openssh upgrade's preset cannot silently re-enable it and
   reintroduce the conflict.
4. `restart ssh.service` (NOT reload — reload is the SIGHUP that fails to
   rebind; an in-flight SSH session survives a restart because its
   `sshd-session` child persists).
5. **Lockout guard:** confirm a `:22` listener exists (`ss -ltn`). If not,
   roll `ssh.socket` back (`unmask` + `start`) so the operator keeps a way
   in, then a second `ssh.service` attempt. The script never `_die`s
   leaving the host with no listener.

The three `systemctl reload ssh` call sites are made safe by this: after
normalization the socket is masked, so reload SIGHUPs a classic daemon that
rebinds cleanly. `update.go`'s unconditional reload is replaced by a call to
the normalizer; `install.sh`'s `ensure_sshd_running` delegates to it.

## Alternatives Considered

- **Keep socket activation, disable ssh.service** — leaves the panel's
  port-change feature a no-op and keeps a per-connection bind model the
  panel doesn't reload-manage. Rejected.
- **Only guard the reloads (skip reload when socket-active)** — stops the
  immediate crash but leaves every dual-enabled host one stray SIGHUP from
  lockout forever, and leaves the port-change no-op unfixed. Rejected as a
  half-fix.
- **`disable` ssh.socket without `mask`** — a future openssh upgrade preset
  re-enables it, reintroducing the hybrid. Rejected.

## Consequences

### Positive
- Fixes GH #133: no more reload-induced lockout; `jabali update` heals
  already-affected hosts.
- The panel SSH-port-change feature now actually moves the listener.
- One predictable, reloadable listener; no per-connection bind races.

### Negative
- Drops socket activation (slightly higher idle footprint: one resident
  sshd master vs on-demand). Negligible for a control-plane host.
- A masked `ssh.socket` is a deliberate, documented state an operator might
  find surprising; the mask is intentional and reversible
  (`systemctl unmask ssh.socket`).

## Implementation

`install/ssh/normalize-ssh-classic.sh` (new, idempotent, lockout-safe).
`install.sh` `ensure_sshd_running` delegates to it. `update.go` SSH step
replaces `systemctl reload ssh` with the normalizer. Reproduced the exact
`fatal: Cannot bind any address` lockout on a Debian 13 host, then verified
the normalizer converges to classic, restores the `:22` listener, is
idempotent, and renders a subsequent `systemctl reload ssh` harmless.
Supersedes the partial GH #133 status-display fix (`128d0f92`), which
corrected only the false "stopped" reading, not the lockout.
