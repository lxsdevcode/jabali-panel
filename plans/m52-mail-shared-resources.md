# M52 — Mail Shared Resources & Groups redesign (Wave 3)

**Status:** DRAFT blueprint — reviewed; **Gate 1 (read-side) VALIDATED on mx 2026-06-20**;
**Gate 2 (send-as) RESOLVED by architecture** — use **one single-collection principal per
shared resource** so membership/`shareWith` grants only that resource (selectivity is
inherent; no all-or-nothing, no `queryMemberOf` experiment). jabali **already ships** the
mailbox-share primitive (`mailbox.share_set`, "Shared Folders") — extends proven infra.

**Key architectural decision (resolves the whole cluster):** Context7
(`/docs/collaboration/sharing`) is explicit — "group membership alone grants access" to ALL
of a group's calendars/address books/files; there is no mail-only membership. Rather than
fight that, **stop bundling resources on one group principal.** Each shared resource is its
OWN principal hosting exactly ONE collection. Membership/`shareWith` on a single-collection
principal therefore grants only that one resource. Send-as a shared mailbox = membership in a
mailbox-only principal (its empty calendar/files don't matter). This matches the user's
#241/#242 model directly and makes the M51 "one group owns one of each, all-or-nothing"
obsolete.
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

**Existing infra to reuse (found 2026-06-20):** jabali already has
`mailbox.share_set` (`panel-agent/.../mailbox_share.go`) — the **Shared Folders** feature.
It writes `Mailbox/set shareWith` with a battle-tested Rights→Stalwart ACL mapping
(`mayRead`→`mayReadItems`, `mayAdmin`→`mayShare`, plus `maySubmit`), idempotent whole-map
replace, target-principal resolution by email. M52 extends this exact pattern to
Calendar/AddressBook/FileNode share_set — NOT greenfield.

**Gate 1 — read-side effect: VALIDATED end-to-end (mx, 2026-06-20).** Provisioned a real
test mailbox (`m52proof@jabali.site`, bcrypt hash inserted into `mailboxes`, projected via
`mailbox.create`; the external SQL directory means this is the only way — raw `x:Account/set`
can't set credentials). Shared a group calendar with it via `Calendar/set shareWith/<id>`,
then **from the member's own authenticated session**:
- `/jmap/session` `accounts` map automatically gained the group account (`prooftest@…`) —
  **no subscribe/accept step**.
- `Calendar/get accountId=<group> ids=[<cal>]` → returned the calendar with
  `myRights.mayReadItems=true`. **The grant grants real, immediate access.**
Mailbox `shareWith` is independently proven by the shipped Shared Folders feature. Calendar
proven here; AddressBook/FileNode use the same JMAP Sharing mechanism (low risk; confirm in
Wave A). **The "retire `memberGroupIds`, compose via `shareWith`" model holds for
calendars/contacts/files.**

**Gate 2 — send-as: still OPEN; the easy hypothesis is DISPROVEN (mx, 2026-06-20).**
As `m52proof` (with the group inbox shared incl. `maySubmit`):
- `Identity/set create {email: prooftest@jabali.site}` in the member's own account →
  **rejected**: "E-mail address not configured for this account."
- `Identity/get accountId=<group>` → **forbidden**.
So `maySubmit` on a shared mailbox does **not** confer send-as.

**Stalwart directory model (Context7 — `/docs/auth/backend/sql`):** an account may send-as an
address only if that address is in its `emails` (primary/alias) OR it is a member of that
group/list via `queryMemberOf`/`members`. There is **no decoupled "send-as only"** mechanism;
an `emails` alias is also a *delivery* alias (`queryRecipient` resolves it → delivery to the
account). Candidates, narrowed:
  1. **Registry membership** (`memberGroupIds`) — works, but grants ALL collections
     (ADR-0132). All-or-nothing.
  2. **SQL `queryMemberOf`** — jabali currently leaves it NULL (membership lives in the
     registry; mig 000170 comment). **KEY WAVE A EXPERIMENT:** wire `queryMemberOf` from
     `mail_group_members` and test whether SQL-directory membership grants **mail-only**
     (recipient-expansion + send-as) WITHOUT calendar/file collection access (collections may
     require the *registry* `memberGroupIds` or `shareWith`). If mail-only → the clean win:
     `queryMemberOf` drives send-as + distribution, `shareWith` drives selective
     calendars/contacts/files, registry `memberGroupIds` is retired.
  3. `emails` alias for the group addr on the member — rejected (delivery side-effect).
**Likely outcome (pending the Gate-2 experiment):** if SQL `queryMemberOf` is mail-only, a
send-as shared mailbox + selective collections is achievable; otherwise a send-as shared
mailbox keeps registry membership (all-grant) and only receive-only shared mailboxes +
calendars/contacts/files are selective. Close in Wave A before the mailbox path.

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

## 3b. Per-resource `shareWith` rights vocabulary (nailed on mx 2026-06-20)

Each collection type rejects keys from other types (`invalidProperties`). The
`share_set` agent commands must map jabali's internal Rights to these exact keys:

| Resource | Valid `shareWith` rights |
|---|---|
| Mailbox | `mayReadItems, mayAddItems, mayRemoveItems, mayCreateChild, mayRename, mayDelete, mayShare, maySubmit` |
| Calendar | `mayReadFreeBusy, mayReadItems, mayWriteAll, mayWriteOwn, mayUpdatePrivate, mayRSVP, mayShare, mayDelete` |
| AddressBook | `mayRead, mayWrite, mayDelete, mayShare` |
| FileNode | `mayRead, mayAddChildren, mayModifyContent, mayRename, mayDelete, mayShare` (host has NO node until created — `file.share_set` find-or-creates a top-level folder, then shares it) |

(Mailbox keys already encoded in `mailbox_share.go` `toStalwartACL`; existing
`mayRead`→`mayReadItems`, `mayAdmin`→`mayShare` mapping is Mailbox-specific.)

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

**Wave A — foundation (gates resolved; sequential).**
- Gates 1 + 2 are resolved (§2 / status). Model = one single-collection principal per shared
  resource + `shareWith` grants. No live `queryMemberOf` experiment needed.
- Confirm AddressBook + FileNode `shareWith` behave like Calendar (quick, same mechanism) —
  via the real-mailbox recipe (Gate 1 method).
- Migration 000174: `shared_resources` + `shared_resource_grants` (+ `mail_groups.group_kind`,
  deprecate `has_*`) + backfill (§4).
- Agent: extend the existing `mailbox.share_set` pattern (reuse its Rights→Stalwart ACL
  mapping) into `calendar.share_set` / `addressbook.share_set` / `file.share_set` — same
  idempotent whole-`shareWith`-map replace, target-by-email resolution. Plus a
  `sharedresource.apply`/`destroy` that provisions/removes the one-collection host principal.
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
- **Gate 1 (read-side `shareWith` effect): RESOLVED** — validated end-to-end on mx; grants
  give immediate member access, no accept step. Calendar proven; confirm AddressBook/FileNode
  in Wave A (low risk, same mechanism). No longer a blocker.
- **Send-as (Gate 2): the remaining unknown** — `maySubmit`-on-shared-mailbox is DISPROVEN.
  Close before the mailbox path. Fallback: shared mailboxes whose members send-as keep
  `memberGroupIds` (accept all-grant for that case); calendar/contacts/files stay selective.
  This means a "fully selective shared mailbox with send-as" may not be achievable — the
  Shared Mailboxes UX (#241) must reflect that (e.g. send-as ⇒ member also gets the group's
  other resources, OR offer receive-only shared mailboxes as the selective option).
- `shareWith` reconciliation must be idempotent (no-change compare) — Stalwart `Foo/set`
  every tick would churn. Gate on diff.
- Backfill must not brick fresh/existing installs (migration = schema + static backfill only).
- Scope: this is a multi-wave milestone, NOT a quick fix. #236 is only honestly resolved by
  shipping at least Wave A + the toggle removal in Wave C.

## 8. Interim option (if full M52 is deferred)
Ship only the honest #236 fix now: remove the non-functional resource toggles from the
Groups UI + API (a group provides all shared resources), and revisit selectivity in M52.
Small, stops the lie, doesn't block the redesign.
