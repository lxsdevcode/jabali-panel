# M52 — Mail Shared Resources & Groups redesign (Wave 3)

**Status:** DRAFT blueprint — reviewed once; **model NOT yet validated.** Two Wave A gates
(read-side `shareWith` effect + send-as) must pass through the real provisioning path before
the model is proven. Write-side (`shareWith` is settable) is confirmed; member-side effect
is not.
**Issues:** #236 (selective group resources), #241 (Shared Mailboxes vs Shared Folders),
#242 (Shared Calendars/Contacts/Files tab), #235 (Distribution Email Groups).
Adjacent (separable, mailbox-UX): #237 (alias/forwarding under mailbox), #238 (show
groups on mailbox), #240 (auto-responder → mailboxes tab). Target ADR-0133.

---

## 1. Problem

The M51 group model (ADR-0132, migration 000170) makes a group a single Stalwart
`@type:Group` principal that **auto-owns** one mailbox + calendar + address book + file
folder, and membership (`memberGroupIds`) grants every member access to **all** of them.

Consequences (all live-confirmed on mx 2026-06-20):
- **#236**: the panel's `has_mailbox`/`has_calendar`/`has_addressbook`/`has_files` toggles
  are **non-functional decoration**. The agent only sends `email/display_name/description`
  on `mailgroup.apply`; the flags never reach provisioning, and Stalwart auto-creates all
  four for any Group principal. Unchecking does nothing; edit can't remove a resource.
- The user's actual mental model (#241/#242): shared **mailboxes**, **calendars**,
  **contacts**, **files** should be **standalone resources**, each **assignable to a group
  or members**, and **one group can own many** of each — not "one group = one of each".

## 2. The Stalwart constraint (spike findings, mx / 0.16.6)

Two **independent** access paths exist:

| Path | Granularity | Use |
|---|---|---|
| `memberGroupIds` edge | **all-or-nothing** — grants every group collection | M51 today |
| per-collection `shareWith` | **per resource, per principal, per right** | the redesign |

Proven live (WRITE side, as admin):
- `Calendar/set { accountId:<group>, update:{<calId>:{ "shareWith/<principalId>": {rights} }}}`
  **succeeds** — the per-collection ACL is *settable* server-side from the agent's admin JMAP.
- `Mailbox`, `AddressBook`, `FileNode` expose the same `shareWith` property (default `{}`).
  (Mailbox rights keys are `mayReadItems`/`mayAddItems`/… — NOT `mayRead`; nail exact keys
  per resource in Wave A.)
- Adding a member did **not** populate `shareWith` (stayed `{}`) → membership and shareWith
  are separate. Per docs, membership grants **all** collections, so you **cannot mix**: any
  resource granted via membership pulls in all of them.

**NOT yet proven — two co-equal Wave A gates** (both are "admin/write side verified,
member/effect side NOT"; do NOT start Wave B reconciler work until both pass):

  **Gate 1 — read-side effect (MORE foundational than send-as).** That a `shareWith` grant
  actually makes the collection *readable to that member*, and removing it revokes access.
  Only the *set* call was confirmed; member-side read was **inferred**, not tested (the
  earlier "as shuki" query hit shuki's OWN account, where the group calendar correctly never
  appears; querying `accountId=<group>` was only ever done as *admin*, who always has
  access). The whole "retire `memberGroupIds`, compose via `shareWith`" model rests on this.
  - **Blocker found 2026-06-20:** jabali uses Stalwart's **external (SQL) directory**, so
    `x:Account/set` cannot create a throwaway member with a password
    ("Cannot set credentials for accounts in an external directory"). So this gate CANNOT be
    tested via raw JMAP — it requires a **real provisioned mailbox** (panel→agent→reconciler
    → MariaDB hash + registry projection) with a known password. Test: as that member,
    `Calendar/get accountId=<group> ids=[<cal>]` → returns the calendar with `myRights` when
    granted, FORBIDDEN/empty when not; and the member's `/jmap/session` `accounts` map should
    expose the shared account. If the sharee must *subscribe/accept* the share first, or it
    surfaces only via the session `accounts` map, the reconciler model changes shape — find
    that here, not deep in Wave A.

  **Gate 2 — send-as.** Membership also authorises *send-as the group address* — a
  **send-time check**, not a stored Identity/alias (confirmed: a member's `Identity`/`aliases`
  are unchanged after joining). Decoupling needs a spike. Candidates:
  1. `aliases/<groupaddr>` on the member — **rejected hypothesis**: Stalwart aliases are
     *delivery* aliases (mail to the alias lands in the member's own inbox), breaking the
     shared-inbox semantics. Verify it doesn't gate send-as cleanly anyway.
  2. An `Identity` in the member's account for the group address, authorised by a Mailbox
     `shareWith` submit right rather than membership — **preferred if it works**.
  3. A dedicated `enabledPermissions` send-as grant — fallback.

## 3. Target model

Shared resources become **first-class panel entities**, each projected to a Stalwart
collection and shared via explicit `shareWith` ACLs. The `memberGroupIds` all-or-nothing
edge is **retired for resource access** (kept only if Wave A proves it's the only viable
send-as path, and then ONLY for pure distribution/mail groups).

**Ownership — every Stalwart collection needs a host principal.** A "standalone" shared
resource (the #241/#242 framing) is not ownerless: creating one = **provision a backing
Stalwart principal and share its collection**. Decide the host in Wave A (design point, not
an implementation detail). Options: (a) a dedicated `@type:Group` principal per shared
resource (reuses today's projection; the group address can double as a shared-mailbox
address); (b) a single per-domain "shared-resources host" principal owning many collections.
(a) is the likely default — it keeps one collection per principal and a clean address for
shared mailboxes. `host_account_id` in the schema is that principal.

Entity types (panel DB-as-truth, reconciler projects to Stalwart):
- **Shared Mailbox** — a shared inbox (group address or standalone), hosted by its backing
  principal. Members get `shareWith` read/write; send-as resolved per Wave A Gate 2.
- **Shared Calendar / Shared Address Book / Shared File Folder** — a collection on a host
  principal, `shareWith` per assigned member/group.
- **Group** — now two flavours, made explicit (resolves #235 vs #236 confusion):
  - **Distribution group**: mail fan-out only (a recipient that expands to members). No
    shared collections. (#235)
  - **Resource group**: a named bundle of members used to grant a set of shared resources
    at once (expand → per-member `shareWith`). Replaces the M51 "group owns everything".

Assignment is many-to-many: a shared resource → many groups/members; a group → many shared
resources. ACL reconciliation diffs desired (DB) vs actual (`shareWith` on the collection).

## 4. Schema (migration 000174)

- New `shared_resources` table: `id, domain_id, kind ENUM('mailbox','calendar','addressbook','files'),
  local_part (mailbox only), display_name, host_account_id (the owning Stalwart principal),
  stalwart_collection_id (cached), created_at, updated_at`.
- New `shared_resource_grants` table: `resource_id, grantee_kind ENUM('mailbox','group'),
  grantee_id, rights ENUM('read','readwrite','admin'), created_at` (PK resource+grantee).
- `mail_groups`: add `group_kind ENUM('distribution','resource') NOT NULL DEFAULT 'resource'`;
  **deprecate** `has_mailbox/has_calendar/has_addressbook/has_files` (keep columns for one
  release, stop reading them — migration backfills existing groups into `shared_resources`
  rows so no data loss).
- Reuse `mail_group_members` for resource-group membership.

Migration MUST be schema + backfill only (no app-populated-table reads — see the
migration-ordering scars). Backfill of existing M51 groups → one `shared_resources` row per
enabled `has_*` flag + grants for current members.

## 5. Waves

**Wave A — prove the primitive end-to-end, then foundation (gating, sequential, NOT
parallel-dispatchable). Both gates below MUST pass before any reconciler work.**
- **Gate 1 (read-side effect)** and **Gate 2 (send-as)** from §2 — verified through the
  REAL provisioning path from a member's own session (raw JMAP can't create members under
  the external SQL directory). Provision a throwaway mailbox via the panel/agent with a
  known password, set/clear a `shareWith` grant on a host collection, and confirm the member
  can/can't read it + can send-as. Record the chosen send-as mechanism + the read-side
  semantics (immediate vs subscribe/accept) in ADR-0133. If Gate 1 fails, STOP — the model
  needs rework before Wave B.
- Decide the host-principal model (§3 ownership).
- Migration 000174 + repositories + backfill.
- Agent: new `sharedresource.apply` / `sharedresource.grant_set` / `sharedresource.destroy`
  commands using `Calendar/set`/`Mailbox/set`/`AddressBook/set`/`FileNode/set` shareWith.
  Nail the exact rights-key vocabulary per resource (Wave A spike artifact).
- Reconciler: desired-grants vs actual-`shareWith` diff + converge (idempotent, gate
  side-effects behind a no-change compare — per the per-tick-idempotent-loops scar).

**Wave B — REST + distribution groups (depends on A's contracts).**
- `/domains/:id/shared-resources` CRUD + `/.../grants`. Distribution-group path (#235).
- Wire-contract tests against the real envelope (`{data,total,page,page_size}`) — per the
  verify-wire-contract scar.

**Wave C — UI (depends on B).**
- **Shared Mailboxes** tab replaces **Shared Folders** (#241).
- **Shared Calendars / Contacts / Files** tab (#242) — list + create + assign-to-group.
- **Distribution Groups** tab (#235). Groups tab: drop the dead resource toggles (#236),
  show kind + assigned resources.
- Reuse SearchableTable + Drawer (create+edit) + kebab row actions per CONVENTIONS.

## 6. ADR-0133 decisions (to record)
1. Shared resources are first-class entities projected via per-collection `shareWith`;
   `memberGroupIds` retired for resource access (all-or-nothing — can't be selective).
2. Send-as mechanism = **<resolved in Wave A spike>**.
3. Two explicit group kinds: distribution (mail fan-out) vs resource (member bundle).
4. DB-as-truth; reconciler diffs grants vs live `shareWith`; idempotent converge.
5. M51 `has_*` columns deprecated + backfilled, not hard-dropped (one-release grace).

## 7. Risks
- **Gate 1 (read-side `shareWith` effect) is the model's foundation** — only the write side
  is proven. If a grant does NOT make the collection member-readable (e.g. Stalwart needs a
  subscribe/accept step, or surfaces shares only via the session `accounts` map), the
  reconciler + UX change shape. Close it FIRST in Wave A, via a real provisioned mailbox
  (external-directory blocks raw-JMAP member creation).
- **Send-as (Gate 2)** is the second unknown — close before Wave B. If none of the three
  mechanisms work cleanly, fall back: resource groups keep `memberGroupIds` for the
  mailbox/send-as ONLY and accept that such groups grant all (document the limitation); pure
  calendar/contacts/files sharing still goes selective via `shareWith`.
- `shareWith` reconciliation must be idempotent (no-change compare) — Stalwart `Foo/set`
  every tick would churn. Gate on diff.
- Backfill must not brick fresh/existing installs (migration = schema + static backfill only).
- Scope: this is a multi-wave milestone, NOT a quick fix. #236 is only honestly resolved by
  shipping at least Wave A + the toggle removal in Wave C.

## 8. Interim option (if full M52 is deferred)
Ship only the honest #236 fix now: remove the non-functional resource toggles from the
Groups UI + API (a group provides all shared resources), and revisit selectivity in M52.
Small, stops the lie, doesn't block the redesign.
