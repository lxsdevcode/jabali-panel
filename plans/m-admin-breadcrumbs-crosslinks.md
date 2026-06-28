# Blueprint: Admin Breadcrumbs + Cross-Entity Links (Gitea #483)

**Status:** REVIEWED (advisor pass folded in)
**ADR target:** ADR-0152
**Migration:** none (pure UI + read-side query filters)
**Tracking:** Gitea #483
**Execution:** waves are SELF sequencing, executed **inline** — NOT agent
dispatch. Wave A↔C/D is a wire contract (filter param ↔ consumer); parallel
sub-agent dispatch on wire contracts is a known failure mode here
(feedback_subagent_contract_drift, feedback_never_agents). Do the waves in order,
in this session.

## 1. Problem

The admin panel reads as a set of disconnected list pages. Admins investigate by
following relationships — `domain → owner → that owner's databases / mailboxes /
apps / backups` — but there are no first-class links, so every hop means going
back to the sidebar, opening another list, and re-searching. There are also no
breadcrumbs, so deep pages (DomainEdit, DNS records, migration detail) give no
sense of "where am I / how do I climb back".

## 2. Constraints discovered (ground truth, not assumptions)

These shape the whole design and are the reason a naïve "add `<Breadcrumb>` to
each page" does not work:

1. **The admin shell is list + Drawer, mostly flat.** Most entities are
   created/edited in a Drawer on their list page, NOT on a dedicated detail
   route. Concretely, `App.tsx` redirects `users/create` and `users/edit/:id`
   straight back to `users` — **there is no User detail route at all.** Domains
   are the exception: `domains/edit/:id` (DomainEdit) and `domains/:id/dns` are
   real routes.
2. **No list endpoint accepts an owner filter.** Grep of `internal/api/*.go`
   for `Query("user_id"|"owner"|"username")` returns nothing. `DomainList`
   shows a `username` column via a batch owner lookup, but there is no
   `GET /admin/domains?user_id=…` to drive an owner-scoped view.
3. **Ownership data exists AND all six resources are genuinely user-scoped**
   (verified against `internal/models/` before locking the ADR):
   - `database.UserID` not null — user-scoped.
   - `application_install.UserID` + `python_app.UserID` not null (also carry
     `DomainID`) — user-scoped.
   - `backup_job.UserID` / `backup_schedule.UserID` — user-scoped.
   - `docker_app.UserID` is **nullable** (NULL = admin/server-level, M48):
     server-level installs simply don't appear under any user — correct.
   - `mailbox` carries **both** `DomainID` (not null) and `UserID` — so
     `mail?user_id=` is a real filter; mailboxes are domain-grouped in the UI
     but still owner-filterable. No 2-hop needed.
   Domains carry `user_id` + `username`. So every hub card is a flat 1-hop link;
   none degrade to "via domains". Relationships present in data; only the
   navigation + filtered read paths are missing.
4. **Existing breadcrumb code is local-only.** `FileManagerPage` and
   `DomainPathBrowserModal` use AntD `Breadcrumb` for *filesystem* paths — not
   reusable for entity breadcrumbs. There is no shared entity-breadcrumb
   component.

**Design consequence:** breadcrumbs need a navigable hierarchy. We get one
without rebuilding every page into detail routes by combining (a) a new
**User Overview** hub route, and (b) **owner-scoped list URLs** via query params
(`?user_id=<id>`). Breadcrumb state is then derivable from the route + query.

## 3. Design

### 3.1 Routing additions (minimal, additive)

- **New:** `GET /jabali-admin/users/:id` → `<AdminUserOverview>` — a per-user hub
  page. This is the keystone: it is the breadcrumb root for "drill into a user"
  and the link target for every `→ owner` link. It does NOT replace the
  Drawer-based edit (Edit stays a Drawer launched from the hub + the list).
- **Reuse existing list routes with a query param** for owner-scoped views:
  - `/jabali-admin/domains?user_id=<id>`
  - `/jabali-admin/databases?user_id=<id>`
  - `/jabali-admin/mail?user_id=<id>` (admin mailboxes)
  - `/jabali-admin/docker-apps?user_id=<id>`, `/jabali-admin/applications?user_id=<id>`
  - `/jabali-admin/backups?user_id=<id>`
  Each list, when `user_id` is present, filters to that owner AND renders the
  owner-scoped breadcrumb (`Users › <name> › Domains`).

No migration. No new top-level sidebar items.

### 3.2 Shared breadcrumb module (the deep module)

`panel-ui/src/components/admin/AdminBreadcrumb.tsx`

```
type Crumb = {
  title: ReactNode;          // label (string or icon+label)
  href?: string;             // react-router link target; omit for the leaf
  menu?: { key: string; label: ReactNode; href: string }[]; // sibling dropdown
};
function AdminBreadcrumb({ items }: { items: Crumb[] }): JSX.Element
```

- Wraps AntD `<Breadcrumb items=…>`; maps `href`→react-router `<Link>` so
  navigation is SPA-internal (no full reload).
- `menu` populates AntD Breadcrumb's built-in dropdown (the issue explicitly
  asks for "breadcrumb items that support dropdowns for sibling/related pages").
  Used so e.g. the `Domains` crumb under a user can drop down to
  `Databases / Mailboxes / Apps / Backups` for the SAME user — the bidirectional
  pivot the issue wants.
- Pure presentational; the route→crumbs mapping lives in a small per-area
  helper (3.3), keeping the component deep (one tiny interface, all the AntD +
  router wiring behind it).

### 3.3 Crumb derivation + link helpers

`panel-ui/src/components/admin/entityLinks.ts`

- `ownerCrumbs(user)` → `[Users, <username> (with sibling dropdown)]`
- `ownerResourceCrumbs(user, resource)` → `[Users, <username>, <Resource>]`
- `resourceListCrumbs(resource)` → `[<Resource>]` (no owner scope)
- `link.user(id)`, `link.userDomains(id)`, `link.userDatabases(id)`, … — single
  source of truth for the URL strings (so a route rename is one edit).

This is the seam: pages call `ownerResourceCrumbs(...)`, never hand-build URLs.

### 3.4 Cross-links placed

- **Domain → owner:** DomainList row gets the username cell linked to
  `link.user(user_id)`; DomainEdit header shows `Owner: <username>` linked +
  an `AdminBreadcrumb` (`Users › <name> › Domains › <domain>` when arrived via
  the owner path, else `Domains › <domain>`).
- **User → resources:** `AdminUserOverview` shows resource cards/links
  (Domains, Databases, Mailboxes, Docker/Python Apps, Backups, **Package**)
  each linking to the owner-scoped list URL, with live counts. The user→package
  link is a cheap natural add (users carry `package_id`; the issue names
  `package` in the drill path). Logs are server-wide (not per-user) so there is
  no user→logs link — note it rather than fake one.
- **Owner-scoped lists:** when `?user_id=` is set, the list filters + shows the
  owner breadcrumb with the sibling dropdown to pivot across that user's other
  resource types.

### 3.5 Breadcrumb-context preservation

Owner-scoped pages keep `?user_id=` in their internal links so the breadcrumb
trail and the back-path survive a hop (Users › shukivaknin › Domains →
DomainEdit keeps the `Users › shukivaknin › Domains` prefix). DomainEdit reads
`?user_id` (or falls back to the domain's own `user_id`) to build the prefix.

## 4. Backend (read-side filter wave)

Add an optional `user_id` query filter to the admin list handlers, returning
the same envelope, admin-only, validated as a ULID:

- `internal/api/domains.go` (admin list)
- `internal/api/databases.go`
- `internal/api/mail*` (admin mailboxes)
- `internal/api/docker-apps` + `applications.go`
- `internal/api/backups.go`

Pattern (per handler): `if uid := c.Query("user_id"); uid != "" { validate ULID;
repo filter by user_id }`. Repositories get a `…ByUserID` or a filter arg on the
existing list method. Pagination/sort unchanged. No new tables.

`AdminUserOverview` counts come from the **filtered lists themselves**: call each
with `page_size=1` and read `total` from the `{data,total,page,page_size}`
envelope. 6 parallel calls on a once-loaded hub is fine — **no new
`/resource-counts` endpoint** (that surface is only worth it if the hub later
shows recent rows per card, not just counts). DECIDED, not deferred.

## 5. Waves

- **Wave A (backend, additive):** `user_id` filter on the 5–6 admin list
  endpoints + repo methods + handler tests. Independently shippable; no UI
  dependency. *(no migration)*
- **Wave B (shared UI module):** `AdminBreadcrumb.tsx` + `entityLinks.ts` +
  unit tests (crumb derivation, link strings, dropdown menu). No page wiring
  yet. Depends on nothing.
- **Wave C (User Overview hub):** new route `users/:id` + `AdminUserOverview`
  with resource cards/counts (page_size=1 totals from Wave A). Depends on A + B.
  **Route order:** declare `users/:id` AFTER the static `users/create` (and the
  existing `users/edit/:id` redirect) — React-Router v6 ranks static over
  dynamic so it's safe, but make the order explicit so a future edit can't
  shadow `create`.
- **Wave D (cross-links + breadcrumbs on existing pages):** DomainList owner
  link, DomainEdit owner + breadcrumb, owner-scoped filtering + breadcrumbs on
  the resource lists (read `?user_id`). Depends on A + B + C. **`?user_id` is an
  ADDED `useSearchParams` param** — it must compose with each list's existing
  search/sort/filter state (e.g. DomainList already manages `_filters` + search
  + sort); read+merge, never replace the param set.
- **Wave E (E2E + ADR):** Playwright — `Users → shukivaknin → Domains →
  example.com → Owner: shukivaknin` round trip + the sibling-dropdown pivot;
  write ADR-0152. Depends on all.

A → (B ‖ start) → C → D → E. B is parallelizable with A.

## 6. ADR-0152 decisions to record

1. Owner-scoped views use **query params on existing list routes**, not new
   detail routes (keeps the list+Drawer model; avoids duplicating every list).
2. A **User Overview hub** route is the one new detail route — the breadcrumb
   root + `→ owner` target — because there was no user detail route at all.
3. Breadcrumb sibling navigation uses **AntD Breadcrumb dropdowns**, driven by a
   shared `entityLinks` helper (single source of truth for URLs).
4. Cross-entity links are **read-only navigation**; no permission change (admin
   already sees all). User-shell is out of scope (admin-only feature).
5. Hub counts via **filtered-list `total` with `page_size=1`** — no dedicated
   counts endpoint (DECIDED).
6. All six resources are user-scoped (verified §2.3); every hub card is a flat
   1-hop link. `docker_app` server-level (NULL user) rows are excluded from
   user views by design. Logs are server-wide → no user→logs link.

## 7. Risks / open questions (non-blocking — resolve during the wave)

- Owner-scoped lists should show a **visible, removable filter chip** ("Owner:
  <name> ✕") so admins can widen back to all — recommend yes (Wave D).
- Verify in Wave A that an added `user_id` filter composes cleanly with each
  list's existing pagination + sort (no handler hard-codes the WHERE).
- `AdminMailPage` lists mailboxes grouped by domain; when `?user_id=` is set it
  filters to that owner's mailboxes across domains — confirm the grouping UI
  degrades sensibly (flat list or domain-grouped subset).
