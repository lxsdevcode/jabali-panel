# M51 — Mailbox User Groups with Shared Resources

GitHub issue: shukiv/jabali-panel#201
ADR: docs/adr/0132-m51-mailbox-groups.md
Status: BLUEPRINT (awaiting review → wave dispatch)

## Goal

Let an operator/user create **mail groups** within a domain and place mailboxes
into them. A group bundles shared resources that every member gets automatically
by virtue of membership:

- **Shared mailbox** — group address `name@domain`; members read the group inbox
  and can send as the group.
- **Shared calendar** — owned by the group, visible to all members.
- **Shared address book** — owned by the group, visible to all members.
- **Shared file folder** — owned by the group, visible to all members.

Plus: add/remove members easily, and a **group selector when creating a mailbox**
(when groups exist for that domain).

## Why this is tractable (live probe evidence, Stalwart 0.16.9 on .14)

Probed the live management JMAP API (`x:Account/set`) on 2026-06-17. Findings:

1. **Group principal create works**:
   `x:Account/set {"create":{"#g":{"@type":"Group","name":"marketing","domainId":"<id>","description":"…"}}}`
   → returns an account id, auto-derives `emailAddress = marketing@domain`.

2. **Shared resources auto-provision** when the group is created (NOT empty):
   - `Calendar/get accountId=<group>` → one default calendar `Personal (marketing@…)`,
     `isDefault:true`, full `myRights`.
   - `AddressBook/get accountId=<group>` → one default address book, full rights.
   - `FileNode/get accountId=<group>` → empty list, but the filenode capability is
     live for the account — a shared folder is a `FileNode/set create` under the
     group account.

3. **Membership is user-side**: patch the *member's* account, not the group:
   `x:Account/set {"update":{"<memberId>":{"memberGroupIds/<groupId>":true}}}`
   → `updated`. Remove with `…:null`.

4. **Resource sharing is automatic by membership** (Stalwart docs,
   `/docs/collaboration/sharing`): *"Group resources … are automatically visible
   to every member once they are added to the appropriate group. Group membership
   alone grants access."* No per-resource ACL writes needed.

5. **Delete ordering**: destroying a group with live members is refused
   (`objectIsLinked`, lists the linked member accounts). Must strip memberships
   first, then `x:Account/set destroy`.

6. **Member-session probe (authed AS the member, not admin) — all green.**
   Set a known password on a test mailbox, authed Basic `member:pw` against the
   JMAP endpoint, and in the *member's own* session confirmed:
   - The session `accounts` map includes the group account (`isPersonal:false`,
     `isReadOnly:false`) automatically once `memberGroupIds` is set.
   - `Calendar/get` / `AddressBook/get` on the group account return the group's
     collections with full `myRights` → **shared resources work from the member
     session**, not just per docs.
   - `Mailbox/get accountId=<group>` shows the full folder set (Inbox/Sent/…),
     every folder `maySubmit:true` + full rights → **member reads the shared inbox
     and can send**.
   - `Identity/get accountId=<group>` returns the group's own sending Identity
     (`marketing@domain`) → **send-as-group works** via the shared-account context
     (NOT via the member's personal account — the #199 wall stands for the
     personal identity, but is sidestepped by sending in the group account).
   - **Membership is durable**: after the member authenticated (principal
     re-derived per ADR-0045), admin re-read still showed `memberGroupIds:{group}`.
     The registry edge is NOT clobbered by re-auth.

## Architecture — DB-as-truth, registry as projection (ADR-0045 preserved)

Groups don't authenticate, so jabali's DB is authoritative and the Stalwart
registry is a projection — exactly the `accountEnsureInRegistry` pattern used for
mailboxes today.

### Data model (migration 000170)

```
mail_groups
  id            CHAR(26) PK (ULID)
  domain_id     CHAR(26) FK domains(id) ON DELETE CASCADE
  user_id       CHAR(26) FK users(id)            -- owner (the domain's user)
  name          VARCHAR(64)   -- local part; group email = name@domain
  description   VARCHAR(255)
  display_name  VARCHAR(255)  -- defaults to name
  has_mailbox   TINYINT(1) default 1   -- shared INBOX provisioned
  has_calendar  TINYINT(1) default 1
  has_addressbook TINYINT(1) default 1
  has_files     TINYINT(1) default 1
  created_at / updated_at
  UNIQUE (domain_id, name)

mail_group_members
  group_id    CHAR(26) FK mail_groups(id) ON DELETE CASCADE
  mailbox_id  CHAR(26) FK mailboxes(id)  ON DELETE CASCADE
  created_at
  PRIMARY KEY (group_id, mailbox_id)
```

The group's shared-mailbox password: groups have no interactive login. The
registry Group account carries `aliases:{}` and no credentials; the shared INBOX
is read by members via membership, so **no group password is stored or required**.
(The issue lists a "Password" field under shared mailbox — we drop it; flag in the
reply. If johnnyq wants a dedicated login for the shared address, that's a v2
"service account" toggle, not core.)

### Stalwart SQL directory wiring (the ONE real change)

Inbound SMTP to `marketing@domain` must resolve. Today `queryRecipient` only
knows `mailboxes` + `email_forwarders` aliases. The group registry account has an
`emailAddress`, but our SqlDirectory is the auth/recipient source — add group
resolution so mail to a group address is accepted and delivered to the group
account's inbox:

- Extend `queryRecipient` (install/stalwart/apply-plan.json.tmpl) to also match a
  row in `mail_groups` joined to `domains` (email = `name@domain`), returning the
  group's email so Stalwart accepts + delivers to the group account's mailbox.

`queryMemberOf` is **NOT wired** — the member-session probe proved membership
lives durably in the registry (`memberGroupIds`, survives re-auth), and sharing +
send-as flow from that. Wiring `queryMemberOf` would be redundant (and risks a
second, conflicting membership store). Membership = DB (`mail_group_members`)
projected into the registry by the agent; the directory only resolves the group
*recipient*. One read-only SELECT, no write path through the directory.

### Agent commands (panel-agent, registry projection)

New `mailgroup_*.go` handlers mirroring the mailbox ones:

- `mailgroup.apply` — idempotent converge of one group:
  ensure domain in registry → `x:Account/set` create-or-noop the Group
  (`@type:Group`, name, domainId, description) → set display name (description) →
  provision FileNode root if `has_files` and absent. Returns `{ok, group_id}`.
- `mailgroup.members_set` — converge membership: read current members of the
  group account, diff against desired mailbox emails, patch each
  added/removed member's `memberGroupIds/<gid>`. Idempotent (compare before write,
  per `feedback_per_tick_idempotent_loops`).
- `mailgroup.delete` — strip all memberships first (handles `objectIsLinked`),
  then `x:Account/set destroy`.

Reuse: `accountIDByEmail`, `domainIDByName`, `createDomain`, `jmapCall`,
`setAccountDescription`. Add `groupIDByEmail` (same `{name,domainId}` filter).

### panel-api

Route family `mailgroups` (follows the M6.5 routes registry pattern):

- `GET /domains/:id/mailgroups` — list groups for a domain (+ member count).
- `POST /domains/:id/mailgroups` — create (name, description, resource toggles);
  defaults: display_name=name. Writes DB row → dispatches `mailgroup.apply`.
- `GET /mailgroups/:id` — detail incl. members.
- `PATCH /mailgroups/:id` — description / display_name / resource toggles.
- `DELETE /mailgroups/:id` — DB delete (cascade members) → `mailgroup.delete`.
- `PUT /mailgroups/:id/members` — set member mailbox-id list → `mailgroup.members_set`.
- Admin server-wide: `GET /admin/mailgroups` (ListAllWithDomain, mirrors
  AdminMailPage).
- Mailbox create: accept optional `group_ids[]`; after mailbox create, add to
  each group via members_set. Surfaced by the create wizard.

Repository: `MailGroupRepository` (Create/Update/Delete/Get/ListByDomain/
ListAll/SetMembers/ListMembers). Dedicated `UpdateXxx` methods, no Select
allowlist (per `feedback_domain_update_allowlist_silent_drop`).

Reconciler: a `mailgroup.apply` + `members_set` pass per group on the existing
mail reconcile tick, gated on a no-change compare so it doesn't churn the registry
every tick (per `feedback_per_tick_idempotent_loops`).

### panel-ui

- User Mail tab: new **Groups** sub-tab — SearchableTable (name, email, members,
  resources), Drawer create+edit (name, description, resource checkboxes:
  mailbox/calendar/addressbook/files), member transfer (Select of domain mailboxes).
- Admin Mail page: server-wide Groups table (mirror AdminMailPage).
- Create-mailbox wizard: when groups exist for the chosen domain, a multi-select
  "Add to groups" step.
- Hooks: `useMailGroups` (TanStack Query, envelope `{data,total,page,page_size}`
  per `feedback_verify_wire_contract` — verify against the handler, not this doc).

## Waves

- **Wave A (foundation, dispatchable):** migration 000170, MailGroup model +
  repository, directory query template changes (apply-plan.json.tmpl
  queryRecipient + queryMemberOf), unit tests. No UI. Additive — safe on main.
- **Wave B (agent):** `mailgroup.apply` / `members_set` / `delete` + JMAP helpers
  + tests. Depends on A's wire types only.
- **Wave C (panel-api):** routes_mailgroups + handlers + reconciler pass +
  mailbox-create group attach. Depends on A+B.
- **Wave D (UI):** user Groups sub-tab + admin table + create-wizard step.
  Depends on C's wire contract (read the handler).
- **Wave E (verify + smoke):** Playwright; **live e2e on .14** — the JMAP-level
  behaviour (resources, send-as, membership) is already proven via the
  member-session probe, so Wave E focuses on the two things the probe did NOT
  cover: (1) end-to-end inbound SMTP to a group address actually lands in the
  group inbox (the `queryRecipient` change), and (2) Bulwark webmail surfaces the
  shared account + group From identity cleanly. Runbook + ADR accept.

Per the never-agents rule: all waves executed inline by the dispatcher,
sequential, deploy to .14 after each.

## Risks / gated items

1. **Inbound SMTP delivery (the remaining unknown).** Send-as, receive-read, and
   all shared resources are PROVEN at the JMAP layer (member-session probe). The
   one thing the probe did NOT exercise is real SMTP: does mail to `marketing@`
   actually get accepted + delivered to the group inbox once `queryRecipient`
   knows the group? Conservative assumption: yes, once the SELECT matches. Wave E
   sends a real test message to confirm (and confirms Stalwart rejects a group
   address absent from `queryRecipient`, i.e. the change is load-bearing).
2. **Shared-mailbox model.** v1 = group account owns its own INBOX; mail to the
   group lands there; members read via membership. (Distribution-list fan-out to
   each member's personal inbox via `x:MailingList/set` is an alternative — NOT in
   v1; the issue asks for a shared mailbox, not a list.)
3. **Delete ordering.** Must strip memberships before destroy (`objectIsLinked`).
   `mailgroup.delete` handles it; DB cascade removes member rows first.
4. **Directory cache.** After group create/delete, recipient resolution may be
   cached; mirror the pdns lesson — verify Stalwart honours the new
   `mail_groups` row promptly (SqlDirectory re-reads per ADR-0045, but confirm no
   stale-recipient window in Wave E).
5. **Migration numbering.** 000170 — verify free at merge time
   (per `feedback_merge_audit_migrations`; this session already abandoned a 000170
   once for #196 — confirm it's not lingering).
6. **Password field dropped.** Issue lists a shared-mailbox password; groups don't
   log in, so omitted. Note in reply.

## Out of scope (v1)

- Distribution-list fan-out (`x:MailingList/set`).
- A dedicated login/password for the shared mailbox address.
- Cross-domain groups (groups are domain-scoped).
- Nested groups.
