# Admin/User panel parity audit

Goal: admin and user shells should look + behave the same for shared features —
same table chrome (the panel-wide `SearchableTableStringQ`: card + debounced
search + sortable + `RowActions`), same column conventions (role-specific
columns like Owner are the only allowed delta), same tab structure.

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

## Done this session
- **User mail Mailboxes** → `SearchableTableStringQ` (`dcb9202a`). Was a bare
  `<Table>` with no search; now matches the panel default. Verified live
  (seed15: search box renders + filters 16→2).

## Remaining parity work (prioritised)

1. **Mail columns** (biggest visual delta). User Mailboxes shows avatar +
   aliases + Groups + Auto-replies + Last-usage; admin shows Name + Owner. To
   "look the same" the column sets should converge — but this is a PRODUCT
   decision (admin gains the richer columns across domains, or user drops some).
   Owner stays admin-only. Needs a decision before implementing.
2. **Other user mail tabs** (Forwarders / Groups / Catch-All / Disclaimer /
   Shared Folders / Shared Resources / Logs) use bare `<Table>`. Mechanical
   conversion to `SearchableTableStringQ`; low urgency (most hold few rows).
3. **Cron** (admin + user) both bare. Convert both to the standard for
   consistency with Domains/Databases.
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
