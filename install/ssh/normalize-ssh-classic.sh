#!/usr/bin/env bash
# normalize-ssh-classic.sh — converge the host onto a single, classic,
# long-lived OpenSSH listener (ssh.service) and remove the socket-activation
# unit (ssh.socket). Idempotent + lockout-safe. Shared by install.sh and
# `jabali update` so the deployed fleet self-heals on update, not only on
# reinstall.
#
# WHY (GH #133): Debian 13 / Ubuntu 24.04 ship BOTH ssh.service and
# ssh.socket "enabled by preset". They conflict on :22 — only one may own
# the port. When both are enabled and a later `systemctl reload ssh` sends
# SIGHUP, the socket-activated sshd re-execs and cannot rebind the port that
# the dead-but-enabled ssh.socket still claims:
#     sshd[…]: fatal: Cannot bind any address
#     ssh.service: Failed with result 'exit-code'
# → ssh.service failed, ssh.socket inactive → the operator is locked out.
#
# Classic mode is also REQUIRED by the panel: the SSH-port-change feature
# writes `Port N` into a sshd drop-in, but under socket activation the listen
# port is owned by ssh.socket's ListenStream, so the drop-in is a silent
# no-op. A reloadable long-lived daemon is the model the panel assumes.
#
# This script is deliberately conservative: it NEVER leaves the host without
# a :22 listener. If the converged ssh.service fails to bind, it rolls the
# socket unit back so the operator keeps a way in.
set -u

log()  { printf '[ssh-normalize] %s\n' "$1"; }
warn() { printf '[ssh-normalize] WARN: %s\n' "$1" >&2; }
err()  { printf '[ssh-normalize] ERROR: %s\n' "$1" >&2; }

# Debian 13 / systemd 256 ships systemd-ssh-generator, which auto-exposes sshd
# over AF_VSOCK (+ AF_UNIX). On a KVM/QEMU guest with no assigned vsock CID it
# floods the console + journal on EVERY boot and EVERY daemon-reload with:
#   systemd-ssh-generator[...]: Failed to query local AF_VSOCK CID: Cannot assign requested address
# (GH #290). The console I/O storm can wedge an idle VM and lock SSH out. jabali
# serves SSH via classic ssh.service ONLY (never over vsock/unix), so disabling
# the generator has zero effect on the real TCP :22 listener — it just stops the
# failing auto-units. Mask it to /dev/null. Idempotent.
disable_ssh_vsock_generator() {
  local gen mask=/etc/systemd/system-generators/systemd-ssh-generator
  for gen in /usr/lib/systemd/system-generators/systemd-ssh-generator \
             /lib/systemd/system-generators/systemd-ssh-generator; do
    [[ -e "$gen" ]] || continue
    if [[ "$(readlink -f "$mask" 2>/dev/null)" != /dev/null ]]; then
      mkdir -p /etc/systemd/system-generators
      ln -sf /dev/null "$mask"
      systemctl daemon-reload 2>/dev/null || true
      log "disabled systemd-ssh-generator (AF_VSOCK CID spam on KVM guests — GH #290)"
    fi
    return 0
  done
}
disable_ssh_vsock_generator

# Is anything listening on TCP :22 right now? (IPv4 or IPv6.)
has_listener() {
  ss -ltnH 'sport = :22' 2>/dev/null | grep -q . && return 0
  return 1
}

# RHEL/Rocky ship sshd.service with no socket unit — nothing to normalize.
if ! systemctl list-unit-files ssh.service >/dev/null 2>&1; then
  if systemctl list-unit-files sshd.service >/dev/null 2>&1; then
    systemctl is-active --quiet sshd.service || {
      systemctl reset-failed sshd.service 2>/dev/null || true
      systemctl start sshd.service 2>/dev/null || warn "could not start sshd.service"
    }
    log "sshd.service host (no ssh.socket) — left as-is"
    exit 0
  fi
  warn "no ssh.service or sshd.service unit found — skipping SSH normalization"
  exit 0
fi

# Ensure the privilege-separation runtime dir exists BEFORE validating.
# `sshd -t` returns 255 on "Missing privilege separation directory:
# /run/sshd" — a RUNTIME condition (the tmpfs dir is gone after a failed
# ssh.service, exactly the state we're recovering), NOT a config error.
# ssh.service's own ExecStartPre creates it; do the same so the validate
# below tests config validity, not whether /run was repopulated.
mkdir -p -m 0755 /run/sshd 2>/dev/null || true
chown root:root /run/sshd 2>/dev/null || true

# Validate config BEFORE touching unit topology. A broken drop-in must not
# trigger a restart that fails to bind. If genuinely invalid, leave the
# running listener untouched and report.
if ! sshd -t 2>/dev/null; then
  err "sshd -t failed (config invalid) — refusing to restart SSH (live listener left untouched)"
  exit 1
fi

socket_was_enabled=0
systemctl is-enabled --quiet ssh.socket 2>/dev/null && socket_was_enabled=1
socket_was_active=0
systemctl is-active --quiet ssh.socket 2>/dev/null && socket_was_active=1
service_active=0
systemctl is-active --quiet ssh.service 2>/dev/null && service_active=1

# Already converged: classic service active, socket masked/absent → no-op.
socket_state="$(systemctl is-enabled ssh.socket 2>/dev/null || true)"
if [[ "$service_active" -eq 1 && "$socket_was_active" -eq 0 \
      && ( "$socket_state" == "masked" || "$socket_state" == "" ) ]]; then
  log "already classic (ssh.service active, ssh.socket $socket_state) — no change"
  exit 0
fi

log "converging onto classic ssh.service (socket enabled=$socket_was_enabled active=$socket_was_active)"

# 1) Make the classic service the canonical, boot-enabled listener.
systemctl unmask ssh.service 2>/dev/null || true
systemctl enable ssh.service  2>/dev/null || true

# 2) Retire the socket unit. mask (not just disable) so an openssh upgrade's
#    preset cannot silently re-enable it and reintroduce the conflict.
systemctl stop    ssh.socket 2>/dev/null || true
systemctl disable ssh.socket 2>/dev/null || true
systemctl mask    ssh.socket 2>/dev/null || true

# 3) Bring the classic listener up cleanly. restart (not reload): reload sends
#    SIGHUP which is exactly what fails to rebind in the hybrid state. An
#    in-flight SSH session survives a restart (its sshd-session proc persists).
systemctl reset-failed ssh.service 2>/dev/null || true
systemctl restart ssh.service 2>/dev/null || warn "ssh.service restart returned non-zero"

# 4) Lockout guard: confirm something is listening on :22. If not, roll the
#    socket unit back so the operator keeps a way in, then re-check.
if has_listener; then
  log "converged — ssh.service listening on :22, ssh.socket masked"
  exit 0
fi

err "no :22 listener after converging to ssh.service — rolling ssh.socket back"
systemctl unmask ssh.socket 2>/dev/null || true
systemctl start  ssh.socket 2>/dev/null || true
if has_listener; then
  warn "recovered via ssh.socket fallback — host stays socket-activated; investigate ssh.service bind failure"
  exit 1
fi
# Last resort: force the service again.
systemctl start ssh.service 2>/dev/null || true
if has_listener; then
  warn "recovered via ssh.service second attempt"
  exit 1
fi
err "FAILED to establish a :22 listener via ssh.service OR ssh.socket — manual intervention required (systemctl status ssh.service ssh.socket)"
exit 1
