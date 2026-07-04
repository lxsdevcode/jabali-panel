# ADR-0154: Panel daemon AppArmor confinement via aa-exec

## Status
Accepted (2026-07-04)

## Context
The `jabali-panel` AppArmor profile is deliberately **name-only** (not attached
by binary path) so it does not confine direct operator CLI invocations
(`jabali update` / `repair` / `apparmor flip-mature`), which share the same
binary via the `/usr/local/bin/jabali` symlink. The profile's header comment
stated the daemon would pick the profile up via a systemd
`AppArmorProfile=jabali-panel` directive in the unit.

That directive was **never wired into the unit**, and on Debian systemd's
`AppArmorProfile=` silently no-ops regardless. Result: the profile loaded (and
showed `enforce` in the Security UI) but attached to no process — the panel
daemon ran **unconfined**. Verified on the test VM: the daemon's
`/proc/<pid>/attr/current` was `unconfined`.

## Decision
Confine the daemon by wrapping its `ExecStart` in `aa-exec -p jabali-panel`
(the mechanism already used for the Kratos migrate step; GH #705). The wrapper
probes that the profile is applicable and falls back to a plain exec otherwise,
so a kernel that skips the profile, a container, or a missing `aa-exec` can
never block daemon startup.

CLI invocations are unaffected — they run outside this unit, so they stay
unconfined by design (the name-only profile is never path-attached).

## Consequences
- The panel daemon is now genuinely MAC-confined (files/caps/network scoped to
  the profile) instead of relying on a loaded-but-unattached profile.
- Verified on the VM both ways: profile loaded -> daemon label
  `jabali-panel`, panel serves; profile absent -> plain fallback, panel serves.
- **Follow-up (soak-gated):** the profile still carries broad CLI-era powers
  (chown / exec / broad fs) the daemon no longer needs. Tightening it must ship
  in complain, soak 7 days on a live host to surface would-denies, then flip
  enforce per the standard jabali AppArmor lifecycle — not shipped blind.
