# Blueprint — WordPress Cache Suite (GH #612 #615 #617 #618 #620 #621)

Milestone that turns Jabali's single WordPress cache toggle into a full,
observable, guided cache-management system. Foundation (#606 phpredis+pconnect,
#625 page-cache constants, #629–#645 nginx-cache correctness, #632 warm-on-purge,
#630 purge-ownership) is already shipped — this plan builds ON it.

## Source-of-truth invariant (load-bearing — ADR-0059 / lib.php / GH #602)

The WordPress cache drop-ins load **before** WordPress boots, so they read
`JABALI_CACHE_*` **constants** from `wp-config.php`, never the DB option. The
**panel is the single source of truth**: `setWPConfigCacheConstants`
(panel-agent/internal/commands/wordpress_cache.go) stamps the constants; the WP
admin screen is read-only/advisory (#602). Every setting this suite adds MUST be
stamped as a constant by the panel, not written by the plugin. Constants already
mapped in `lib.php apply_constants()`: SOCKET/DB/PASSWORD/USER/PREFIX/MAXTTL/
SCHEME/DISABLED/PAGE_CACHE/PAGE_TTL.

## Dependency graph

```
Wave A  #612 split controls  ───────────────┬──────────────┐
                                            │              │
Wave B  #615 warmup jobs (uses warmup verb) │              │
                                            ▼              ▼
Wave C            #617 analytics      #618 profiles
                          │                  │
                          └────────┬─────────┘
                                   ▼
Wave D   #620 advisor  ,  #621 cache-aware clone/restore/migration
```

Wave A is the gate. B can start in parallel with A (independent). C needs A. D needs A+B+C.

---

## Wave A — Split object/page/TTL/memory controls (#612)  [GATE]

**Context brief.** Today `application_installs.cache_enabled` is one boolean that
the tooltip says drives both Redis object cache + nginx page cache. Reality:
object cache = jabali-cache plugin (constants), page cache = nginx FastCGI
(per-domain `cache_enabled`/`cache_ttl_seconds`). They must become independently
controllable, with TTL + a Redis per-tenant memory cap surfaced in the app UI.

**Tasks.**
1. Migration: add to the per-install cache model (extend `application_installs`
   or the cache-settings JSON used by `applications_cache_settings.go`):
   `object_cache_enabled bool`, `page_cache_enabled bool`, `page_ttl int`,
   `object_maxttl int`, `redis_maxmemory_mb int` (0 = unlimited/default).
2. Agent: extend `wordpress.cache_set` to stamp `JABALI_CACHE_PAGE_CACHE` +
   `JABALI_CACHE_PAGE_TTL` + `JABALI_CACHE_MAXTTL` from the panel values (the
   constants already exist + are read — GH #625). Apply the Redis per-tenant
   `maxmemory` via the ACL/`CONFIG SET` on the tenant's logical DB namespace if
   feasible, else document as server-wide (note in ADR).
3. API: extend `GET/PUT /applications/:id/cache-settings`
   (`applications_cache_settings.go`) with the new fields + validation.
4. Reconcile: `wordpress.cache_set` + a `nginx.cache` re-render on change.
5. UI: `CacheSettingsDrawer.tsx` — split "Object cache" + "Page cache" switches,
   page-TTL input, object-max-TTL input, Redis memory input, with the existing
   URL-exclusion list. Tooltip corrected (no longer "one switch does both").

**Verify.** `wp jabali-cache diagnose` reflects the stamped constants; toggling
object-only vs page-only produces the expected wp-config constants + nginx vhost
(golden test on both); `nginx -t` passes; live: object-only site has no page
cache HIT header, page-only site caches without the object drop-in.

**Exit.** Object + page cache independently controllable from the app UI, panel
stamps every value as a constant, golden + live tests pass.

---

## Wave B — Warmup / preload jobs (#615)  [parallel with A]

**Context brief.** `nginx.cache_warmup` (agent) crawls the sitemap; `#632`
already warms after a manual purge. Extend to a managed job that fires on cache
**enable**, **WordPress update/deploy**, and **clone/restore** (#621 reuses this),
and expose progress like an app install.

**Tasks.**
1. Agent: harden `nginx.cache_warmup` — bounded concurrency, max URLs from the
   sitemap, `--max-redirs 0` (already, #639), report count + duration.
2. Panel: a warmup dispatcher (fire-and-forget detached ctx, like #632) called
   from: cache-enable (Wave A), WP auto-update hook, deploy/clone (#621).
3. Optional queue: record a `warmup_jobs` row (status running→done) so the UI can
   show "warming N/M urls" — mirror the application_install row pattern.
4. UI: a "Warm cache" action on the app row + a progress indicator.

**Verify.** Enabling cache on a WP site auto-warms; a purge warms (#632 already);
`X-Jabali-Cache: HIT` on the homepage within seconds of enable without a manual
visit; job row transitions running→done.

**Exit.** Cache is warm after enable/purge/update/clone without a first-visitor
cold MISS; warmup visible as a job.

---

## Wave C — Analytics (#617) + Profiles (#618)  [after A]

### #617 Analytics / observability
**Context brief.** nginx emits `X-Jabali-Cache: HIT|MISS|BYPASS|EXPIRED|STALE`;
Redis exposes INFO (keyspace hits/misses, memory). Neither is aggregated in the
panel. Build a collector + a per-install cache card.

**Tasks.**
1. Agent: a `nginx.cache.stats` verb that tails/samples the vhost access log
   (add `$upstream_cache_status` to a dedicated log format) and returns
   HIT/MISS/BYPASS counts + ratio over a window; a `wordpress.cache_health`
   already exists — extend it to return Redis INFO (hit ratio, used_memory,
   evicted_keys, per-prefix key count).
2. Panel: aggregate + expose `GET /applications/:id/cache-stats`.
3. UI: a cache analytics card (hit ratio gauge, HIT/MISS/BYPASS bars, Redis
   memory + key count) on the app detail / Applications drawer.

**Verify.** Card shows a rising HIT ratio after warmup; BYPASS count rises when
hitting `/cart`; Redis memory reflects real usage.

### #618 Cache profiles + guided safety
**Context brief.** The nginx template already hardcodes ecommerce bypasses
(`/cart`, `/checkout`, `/my-account`, WooCommerce cookies). Turn these into
explicit selectable **profiles** with warnings.

**Tasks.**
1. Define profiles (backend constants + a small registry): `brochure`,
   `woocommerce`, `edd`, `membership_lms`, `custom`. Each maps to a bypass-path +
   bypass-cookie preset + a recommended TTL + a safety note.
2. API: profile field on the cache-settings model → the chosen profile expands to
   `CacheBypassPaths` + cookie bypasses the agent already renders.
3. UI: a profile selector in `CacheSettingsDrawer` with per-profile warnings
   ("WooCommerce carts/checkout are always bypassed; do not cache logged-in
   sessions"). Detect active plugins (from #620 probe) to suggest a profile.

**Verify.** Selecting `woocommerce` renders the WooCommerce bypass set in the
vhost (golden test); `brochure` renders the minimal set; switching profiles
re-renders + reloads nginx.

**Exit.** Analytics card live + accurate; profiles selectable with correct
bypass expansion + warnings.

---

## Wave D — Advisor (#620) + Cache-aware clone/restore/migration (#621)  [after A+B+C]

### #620 Cache advisor
**Context brief.** Instead of guessing, probe the install and recommend a
profile + settings.

**Tasks.**
1. Agent: a `wordpress.cache_probe` verb — WP version, active theme, active
   plugins (detect WooCommerce/EDD/membership/LMS/multilingual/currency),
   homepage + sample-post TTFB with vs without object cache, presence of
   `/cart`/`/checkout`.
2. Panel: `POST /applications/:id/cache-advise` → runs the probe, maps findings
   to a recommended profile (#618) + object/page/TTL settings (#612), returns a
   diff vs current + a one-click "apply recommendation".
3. UI: an "Advisor" panel showing findings + recommended settings + Apply.

**Verify.** On a WooCommerce site the advisor recommends the `woocommerce` profile
+ page cache with cart/checkout bypass; on a brochure site it recommends
aggressive page cache; Apply writes the settings (Wave A path) + warms (Wave B).

### #621 Cache-aware clone / restore / migration
**Context brief.** Clone/restore/migration currently don't set cache defaults or
warm; a migrated site starts cold and may carry stale source cache config.

**Tasks.**
1. On clone/restore/migration completion: strip any source `JABALI_CACHE_*`
   constants + re-stamp jabali's (the connection constants are per-tenant — never
   carry the source's socket/ACL); default to a safe profile (brochure/object-off
   until advised); trigger a warmup (#615).
2. Wire into `wordpress.clone`, `system.restore`/`backup.restore`, and the
   migration import spine (`migration.import_wp_run`).

**Verify.** A cloned/migrated WP site comes up with jabali's own cache constants
(not the source's), a safe default profile, and a warm cache; no stale
cross-tenant Redis prefix.

**Exit.** Clone/restore/migration produce a correctly-configured, warm cache with
no source-config carryover.

---

## Cross-cutting

- **ADR:** write ADR-0149 "WordPress cache suite — panel-stamped constants, split
  controls, profiles, advisor" recording the source-of-truth invariant + the
  profile registry.
- **Security:** every probe/warmup runs SSRF-pinned to local nginx (`--resolve`,
  `--max-redirs 0`, GH #639); Redis memory caps are per-tenant ACL-scoped; no
  plugin ever writes wp-config (GH #602).
- **Tests:** golden vhost + wp-config tests per wave; a live `.86` smoke per wave
  (enable→warm→HIT, profile→bypass, advisor→apply).
- **Runbook:** plans/wp-cache-suite-runbook.md at the end.
