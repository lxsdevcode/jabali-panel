# ADR-0149: Agent-side SO_PEERCRED authorization on the root PTY broker

**Status:** Accepted (2026-06-27)
**Extends:** ADR-0096 (M45 root web terminal — PTY broker)
**Driven by:** Gitea #469 — the root-shell broker trusted socket group perms alone.

---

## Context

The M45 web terminal (ADR-0096) runs a PTY broker in the root agent. The browser
authorization, the off-by-default gate, and the one-shot terminal token are all
enforced in `panel-api`; `panel-api` then dials the agent's broker socket and
pumps opaque frames. The broker spawns `/bin/bash -l` as **root**.

The broker socket is `0660 root:<jabali-sockets>`, and the broker only required
that the first frame be init JSON with a non-empty `session_id`. It verified
nothing about the caller. The security property therefore rested entirely on the
assumption that *only* `panel-api` is ever in the `jabali-sockets` group.

That assumption is brittle: any local process that is (accidentally or
maliciously) placed in `jabali-sockets`, or any local-socket primitive obtained
inside `panel-api`, becomes a direct **root shell** endpoint — no token, no
admin identity, no session row required.

## Decision

Add an **independent agent-side authorization** check in the broker, as
defense-in-depth that does not move the existing panel-api gate:

`panel-api` runs under systemd with `Group=jabali-sockets`, so its process
**primary gid** is the `jabali-sockets` gid. A process that is merely *added* to
that group carries `jabali-sockets` as a **supplementary** gid, not its primary.
`SO_PEERCRED` reports the peer's **primary** gid. The broker therefore:

- reads the connecting peer's uid + primary gid via `SO_PEERCRED`;
- accepts the connection only if `peer_uid == 0` (root) **or**
  `peer_primary_gid == jabali-sockets gid`;
- **fails closed** if the credentials cannot be read (non-unix conn / error).

This rejects the "accidentally in the group" process (supplementary gid ≠
primary) while admitting exactly `panel-api` (and root, which already owns the
host). It needs no token plumbing into the agent and no change to the wire
protocol.

### Why not validate the one-shot token agent-side?

That would duplicate `panel-api`'s DB-backed token/session validation into the
agent (DB access, token store, client-IP/session-row checks) — a much larger
boundary move for marginal gain. `SO_PEERCRED` answers the actual threat ("any
group member can open a root shell") directly. The panel-api token gate remains
the primary authorization; this is a second, independent fence.

## Consequences

- A non-panel-api local process can no longer reach the broker even with
  `jabali-sockets` group membership.
- The check is a no-op for the legitimate path (panel-api's primary gid matches).
- If `panel-api`'s `Group=` is ever changed away from `jabali-sockets`, the
  broker will reject it — the unit config and this check are now coupled and
  must move together.
- Verified by `TestTerminalPeerCred`; box validation of an actual terminal
  session is recommended when the feature is next enabled.
