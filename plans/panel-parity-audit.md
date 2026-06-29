# Admin/User panel parity audit (Gitea #524)

Goal: admin and user shells should look + behave the same for shared features.
**Reference layout = the admin Users page** (`admin/users/UserList.tsx`):
title + primary-action in a `Space` header → `<Card tabList=… activeTabKey…>`
(tabs INSIDE the card) → `SearchableTableStringQ` (card + debounced search +
sortable + `RowActions`). Role-specific columns (e.g. Owner) are the only
allowed delta. Migrate page-by-page to this composition (a single forced shared
shell is risky across pages with different needs).

Legend: **S** = standard `SearchableTableStringQ`, **B** = bare antd `<Table>`,
**—** = no list / different view.

## List-page pairs

| Feature | admin | user | status |
|---|---|---|---|
| Domains | S | S | ✅ consistent |
| Databases | S | S | ✅ consistent |
| Applications | S | S | ✅ consistent (badges aligned, GH #504) |
| Docker Apps | B | B | ✅ match each other (tabs + StatCards aligned this session) |
| Cron | B | B | ⚠️ match each other, but neither uses the standard component |
| Mail | B* | S | ⚠️ user Mailboxes now standard (this session); admin hand-rolls card+`Input.Search` (looks standard, different component); **columns differ a lot** |
| DNS | B (records) | S (zones) | ℹ️ different views (records editor vs zones overview), not a true pair |

\* admin mail = `<Card>` + `Input.Search` + `<Table>` — visually standard, but
not the `SearchableTableStringQ` component.

## Done
- **User mail Mailboxes** → `SearchableTableStringQ` (`dcb9202a`). Was a bare
  `<Table>` with no search; now matches the panel default. Verified live
  (seed15: search box renders + filters 16→2).
- **Admin Mail** → `Card.tabList` (tabs inside the card), matching the Users
  reference. Was `<Tabs>` outside a separate content `<Card>`. Verified live.
- **User Applications** → `Card.tabList` (Installed/Catalog inside the card,
  StatCards + table in the body). Was `<Tabs>` outside. Verified live.
- **User mail secondary tabs** → `SearchableTableStringQ`: SharedResources,
  SharedFolders, Groups, and the main Forwarders list. Each was a bare `<Table>`
  with no search; now card + debounced search like the panel default. Filter is
  client-side (these lists are already fully loaded in one query). The DA-migration
  redirect-alias table inside ForwardersTab stays a bare `<Table>` (read-only
  "rows shown for visibility" artifact, not a managed list).

## Assessed — no change needed
- **Cron** (admin + user): already `<Card>` + `Input.Search` + `<Table>` — i.e.
  visually card+search+table, and they match each other. Only nit is they use a
  hand-rolled `Input.Search`+`<Table>` rather than the `SearchableTableStringQ`
  component. Visually equivalent → not migrating (pure churn).
- **Table styling** (header bg, row height, status badges, action buttons):
  already consistent everywhere via the shared AntD theme + `RowActions` +
  shared status-tag helpers. #524's styling acceptance is met by the theme.

## Resolved decisions
- **Mail columns** (decided 2026-06-29: "user adopts admin's clean style"):
  removed the per-row LibravatarAvatar from the user mail Mailboxes table — no
  other panel table has row avatars, so it was the most divergent element.
  Email + aliases stay (folded under the address); the data-bearing columns
  (Groups/Auto-replies/Quota/Last-usage) stay (no data lost). Owner remains an
  admin-only column. Verified live (seed15: 0 row avatars, search retained).
- **Per-mail-tab search**: the LIST mail tabs (Forwarders/Groups/Shared Folders/
  Shared Resources) now use `SearchableTableStringQ` (card + debounced search),
  matching the panel default. Catch-All/Disclaimer are single-record config
  forms (not lists → no search). Logs is a streaming/paged view kept bare for
  now; revisit if mail logs grow large.

## Remaining parity work (prioritised)

1. **Mail columns** (biggest visual delta). User Mailboxes shows avatar +
   aliases + Groups + Auto-replies + Last-usage; admin shows Name + Owner. To
   "look the same" the column sets should converge — but this is a PRODUCT
   decision (admin gains the richer columns across domains, or user drops some).
   Owner stays admin-only. Needs a decision before implementing.
2. ~~Other user mail tabs use bare `<Table>`.~~ **DONE** — the list tabs
   (Forwarders/Groups/Shared Folders/Shared Resources) now use
   `SearchableTableStringQ`. Catch-All/Disclaimer are config forms (not lists);
   Logs left bare (streaming view).
3. **Cron** (admin + user) both `<Card>` + `Input.Search` + `<Table>` — visually
   the standard layout already, just not the shared component. Not converting
   (pure churn; see "Assessed — no change needed").
4. **Admin mail** → swap the hand-rolled `Input.Search`+`<Table>` for
   `SearchableTableStringQ` (component parity; low visual gain, layout churn —
   only if we want strict component uniformity).
5. **Mail tab structure**: admin has Mailboxes+Groups; user has 8 tabs. Per the
   2026-06-29 decision ("table chrome only; just look the same"), tab-count
   parity is deferred — the feature tabs (Forwarders/etc.) are tenant-facing.

## Not parity gaps (legit bare tables)
Server-status cards, docker-app drawers, config sections (domain email/IP-ACL/
directory-privacy), dashboards, disk-usage, file manager, SSH-keys/API-tokens
single-purpose tables — these are not list pages and don't need the search
chrome.
