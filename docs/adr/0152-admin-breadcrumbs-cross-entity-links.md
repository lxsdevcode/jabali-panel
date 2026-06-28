# ADR-0152: Admin breadcrumbs + cross-entity navigation

**Status:** Accepted (2026-06-28) — shipped (Waves A–E).
**Driven by:** Gitea #483.
**Plan:** `plans/m-admin-breadcrumbs-crosslinks.md`.

---

## Context

The admin panel reads as disconnected list pages. Admins investigate by
following relationships (`domain → owner → that owner's databases / mailboxes /
apps / backups / package`) but there are no first-class links and no
breadcrumbs, so each hop means returning to the sidebar and re-searching.

Two structural facts of the existing admin shell constrain any solution:

1. **It is list + Drawer, mostly flat.** Most entities are created/edited in a
   Drawer on their list page, not on a dedicated detail route. `App.tsx`
   redirects `users/create` and `users/edit/:id` back to `users` — there is no
   user detail route at all. Domains are the exception (`domains/edit/:id`,
   `domains/:id/dns`).
2. **No list endpoint accepts an owner filter.** `internal/api/*.go` has no
   `Query("user_id")`/`owner` filter; `DomainList` only batch-resolves usernames
   for display.

Breadcrumbs need a navigable hierarchy that the flat list+Drawer model does not
provide.

## Decision

1. **Owner-scoped views are query params on existing list routes**
   (`?user_id=<id>`), not new per-resource detail routes — this preserves the
   list+Drawer model and avoids duplicating every list into a detail variant.
2. **One new detail route — a User Overview hub** (`/jabali-admin/users/:id`,
   `AdminUserOverview`). It is the breadcrumb root for drilling into a user and
   the target of every `→ owner` link, filling the gap left by there being no
   user detail route. It does not replace the Drawer-based edit. Declared after
   the static `users/create` so it cannot shadow it.
3. **Breadcrumb sibling navigation uses AntD Breadcrumb dropdowns**, driven by a
   shared `entityLinks` helper that is the single source of truth for admin
   entity URLs. A presentational `AdminBreadcrumb` component wraps AntD
   Breadcrumb and maps hrefs to react-router `<Link>` (SPA-internal nav).
4. **Cross-entity links are read-only navigation** — no permission change
   (admin already sees all). The feature is admin-shell only; the user shell is
   out of scope.
5. **All six resources are user-scoped** (verified against `internal/models/`:
   database / application_install / python_app / backup_job / backup_schedule
   carry non-null `UserID`; `docker_app.UserID` is nullable = server-level rows
   excluded from user views; `mailbox` carries both `DomainID` and `UserID` so
   it is owner-filterable). Every hub card is therefore a flat 1-hop link; none
   degrade to "via domains". A user→package link is included (users carry
   `package_id`). Logs are server-wide, so there is no user→logs link.
6. **Hub counts come from the filtered lists** (`page_size=1`, read `total` from
   the `{data,total,page,page_size}` envelope) — no dedicated counts endpoint.

## Consequences

- Backend change is additive and read-only: an optional ULID-validated
  `user_id` filter on the admin list handlers (domains, databases, mail,
  docker/python apps, backups). No migration, no new tables.
- A route rename only touches `entityLinks.ts`.
- Owner-scoped lists must show a removable "Owner: <name>" filter chip so admins
  can widen back to all, and the added `?user_id` param must compose with each
  list's existing search/sort/filter state rather than replace it.
- Delivered in five inline waves (A backend filter, B shared UI module, C hub,
  D cross-links+breadcrumbs, E E2E). Executed inline, not via agent dispatch:
  Wave A↔C/D is a wire contract and parallel dispatch on wire contracts is a
  known failure mode in this repo.
