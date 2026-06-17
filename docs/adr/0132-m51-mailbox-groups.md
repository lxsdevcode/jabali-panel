# ADR-0132: M51 Mailbox User Groups — DB-as-truth groups, Stalwart registry projection, native resource sharing

**Date**: 2026-06-17
**Status**: Proposed
**Deciders**: shuki + Claude
**Related**: ADR-0042 (mailbox schema), ADR-0045 (Stalwart v0.16 SQL-directory, registry-as-projection), ADR-0002 (DB is source of truth), ADR-0004 (reconciler-driven convergence), #199 alias-identity finding

> Forward-looking. Blueprint: `plans/m51-mailbox-groups.md`. GitHub issue
> shukiv/jabali-panel#201. Decisions below reflect live probes against
> Stalwart 0.16.9 on the .14 test host (2026-06-17), not a guess.

## Context

Issue #201 asks for cPanel-style mail **groups**: create a group in a domain,
drop mailboxes into it, and have the group carry shared resources — a shared
mailbox (send + receive on the group address), a shared calendar, a shared
address book, and a shared file folder — with easy member add/remove and a
group selector on mailbox creation.

jabali's mail stack (ADR-0045): the panel DB is authoritative; Stalwart reads a
read-only SQL directory for auth/recipient resolution; the JMAP "registry"
(internal principal/groupware store) is a projection that `accountEnsureInRegistry`
mirrors mailboxes into so JMAP-level features work before first auth.

Two facts had to be established live before committing scope, because the issue's
value hinges on them:

1. Does Stalwart **create** the group's calendar/address book/file collections, or
   does membership merely grant access to collections that something else must
   provision?
2. Can a member **send as** the group address from a server-driven setup, or does
   it hit the #199 wall (sending identities only exist in the member's own
   real-auth session)?

## Probe results (Stalwart 0.16.9, live)

- `x:Account/set create {"@type":"Group", name, domainId, description}` creates a
  group principal, auto-derives `emailAddress = name@domain`.
- The group account **auto-owns** a default Calendar and AddressBook (verified via
  `Calendar/get` / `AddressBook/get` with `accountId=<group>`). FileNode root is
  lazy (empty until a `FileNode/set create`).
- Membership is a **user-side** edge: `update {"<memberId>":{"memberGroupIds/<gid>":true}}`.
- Destroy is refused while members are linked (`objectIsLinked`) — strip first.
**Member-session probe (authed AS a member, not admin):**

- Setting `memberGroupIds` makes the group account appear in the member's JMAP
  session `accounts` map (`isPersonal:false`, not read-only) automatically.
- In the member's own session: `Calendar/get` / `AddressBook/get` on the group
  return its collections with full rights; `Mailbox/get` shows the full group
  folder set with `maySubmit:true` everywhere; `Identity/get accountId=<group>`
  returns the group's `marketing@` sending Identity.
- **Send-as works** — not via the member's personal identity (the #199 wall holds
  there) but by operating in the *group account's* context, where the group's own
  Identity + `maySubmit` rights live.
- **Membership is durable**: after the member authenticated (principal re-derived
  per ADR-0045), `memberGroupIds` was still present — the registry edge is not
  clobbered by re-auth. This makes the registry the membership store and means
  SQL `queryMemberOf` is unnecessary.

## Decision

### 1. Groups are DB-authoritative; the Stalwart Group account is a projection

Groups never authenticate, so the panel DB owns them
(`mail_groups` + `mail_group_members`, migration 000170), and the reconciler
projects each group into the Stalwart registry via `x:Account/set @type:Group`
— the same DB-as-truth / registry-as-projection split ADR-0045 set for mailboxes.
No write path runs through the SQL directory.

### 2. Shared resources come from Stalwart, not from jabali ACL bookkeeping

Because Stalwart auto-provisions the group's calendar/address book and shares all
group-owned resources to members by membership alone, jabali does **not** write
per-resource ACLs. The agent's job is: ensure the Group account exists, set its
description (display name), provision the file root when `has_files`, and converge
membership. Stalwart does the sharing.

### 3. Shared mailbox = group-owned INBOX, read by membership (not a fan-out list)

The issue asks for a shared mailbox (send + receive on the group's behalf), not a
distribution list. v1: the group account owns its own INBOX; inbound SMTP to the
group address is delivered there; members access it by membership. The
`x:MailingList/set` fan-out model is explicitly **out of scope** for v1.

### 4. SQL directory gains group recipient resolution only (read-only)

`apply-plan.json.tmpl`: extend `queryRecipient` to accept a group address
(`mail_groups ⋈ domains`) so SMTP delivers to the group account's inbox. One
read-only SELECT; no new write path.

`queryMemberOf` is deliberately **left null**. The member-session probe showed
membership lives durably in the registry (`memberGroupIds` survives re-auth) and
that sharing + send-as flow from it. Wiring `queryMemberOf` would duplicate the
membership store and risk the two disagreeing. Membership is owned by jabali's DB
and projected into the registry by the agent; the directory only resolves the
group as a recipient.

### 5. No group password

Groups have no interactive login; the shared INBOX is reached by membership. The
issue's "Password" field under shared-mailbox is **dropped**. A dedicated
login/password for the shared address would be a v2 "service account" toggle.

### 6. Send-as is in scope (verified)

The member-session probe confirmed a member can send as the group via the group
account's own Identity (`maySubmit:true` on the shared folders). Send-as is a v1
deliverable, not a gated maybe. The only unproven leg is end-to-end inbound SMTP
to the group address (the `queryRecipient` change) — Wave E confirms with a real
test message. The GitHub reply can state send + receive + shared
calendar/addressbook/files, with the SMTP delivery confirmed in the live smoke.

## Consequences

- **Positive**: most of the feature is Stalwart-native; jabali adds a thin DB +
  projection + two read-only directory queries + UI. Calendar/address book/files
  need no bespoke sharing code.
- **Positive**: consistent with ADR-0045 — one more registry projection, reconciler
  convergence, idempotent per-tick (gated compare, `feedback_per_tick_idempotent_loops`).
- **Negative / risk**: end-to-end inbound SMTP to the group address unconfirmed
  until Wave E; delete ordering must strip memberships first; recipient-cache
  window to confirm in Wave E.
- **Negative**: introduces a second principal kind (Group) into the registry
  projection and reconciler — more surface in the mail reconcile tick.

## Alternatives considered

- **Imperative-only (Route B), groups live solely in the registry via JMAP** —
  rejected: fights the read-only-directory / DB-as-truth invariant held everywhere
  else; no DB record for the panel to list/own.
- **Distribution list (`x:MailingList/set`)** — rejected for v1: the issue wants a
  shared mailbox (one inbox, shared) not per-member fan-out.
- **jabali-managed per-resource ACLs** — rejected: redundant with Stalwart's
  membership-grants-access model; more code, more drift.
