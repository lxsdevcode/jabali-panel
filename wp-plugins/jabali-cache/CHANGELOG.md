# Changelog

All notable changes to the **Jabali Cache** WordPress plugin are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/), and the
project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

Nothing yet.

## [1.0.1] — 2026-07-02

### Changed
- **Redesigned the admin dashboard** (Settings → Jabali Cache) to a modern,
  card-based layout (GH #609):
  - Large `Jabali Cache` page title and a drop-in status banner.
  - Four top metric cards: **Caching enabled**, **Redis connection**,
    **Server page cache**, and **Cache hits (this request)**.
  - Two-column lower layout: a **Cache Status** card (left) and, on the right, a
    **Technical Details** card plus an **Actions** card
    (Flush cache now / Refresh status / View documentation).
  - Inline SVG icons, soft borders, rounded corners, subtle shadows, and clear
    green success / neutral states; responsive down to a single column.
  - Connection settings and drop-in install/repair/remove moved below the
    dashboard under **Drop-ins & Settings**.

### Preserved
- All operational data still shown: caching enabled, Redis connection, driver,
  serializer, target, key prefix, key count, request hit/miss ratio, object TTL,
  object-cache and advanced-cache drop-in status, server page-cache status +
  last probe, Edge `Cache-Control`, and revalidation details.
- All existing actions still work: flush cache, install/repair/remove drop-ins,
  save connection settings, and the admin-bar **Flush Cache** shortcut.

### Notes
- Dashboard/UI change only — the caching engine, drop-ins, and wire behavior are
  unchanged. The `object-cache.php` / `advanced-cache.php` drop-in versions are
  intentionally **not** bumped, so no drop-in reinstall is triggered on upgrade.

## [1.0.0]

### Added
- Initial release.
- Persistent Redis object cache with strict per-site key isolation.
- Pure-PHP Redis client fallback when the phpredis extension is absent.
- Optional full-page cache (off by default; complements the nginx edge cache).
- WP-CLI commands: status, flush, enable, disable, update-dropins,
  remove-dropins, diagnose.
- Admin diagnostics screen with live connection status and fix-it guidance.
- Multisite-aware namespacing.
- Fail-safe: WordPress keeps running normally whenever Redis is unavailable.

[Unreleased]: https://git.linux-hosting.co.il/shukivaknin/jabali2/src/branch/main/wp-plugins/jabali-cache
[1.0.1]: https://git.linux-hosting.co.il/shukivaknin/jabali2/issues/609
[1.0.0]: https://git.linux-hosting.co.il/shukivaknin/jabali2/src/branch/main/wp-plugins/jabali-cache
