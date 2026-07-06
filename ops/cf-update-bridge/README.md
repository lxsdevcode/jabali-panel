# `jabali update` → Codeberg release bridge (Cloudflare Worker)

Transitional bridge so **already-deployed hosts** keep updating after the forge
moved from the (retired) self-hosted Gitea to Codeberg.

## The problem it solves

Every deployed host has `https://git.jabali-panel.com/api/v1/repos/shukivaknin/jabali2`
compiled into its binary (`update_release.go` `defaultReleaseAPIBase`). New
binaries point at `codeberg.org`, but a host only gets a new binary *by
updating* — from the old URL. Chicken-and-egg.

`worker.js` sits on `git.jabali-panel.com` and forwards the release API to
Codeberg, so existing hosts transparently pull Codeberg releases at their old
URL. The **first** such update hands them a codeberg-pointing binary, after
which they bypass the bridge. Self-liquidating.

## Deploy (route on the git.jabali-panel.com zone)

Route pattern: `git.jabali-panel.com/api/v1/repos/shukivaknin/jabali2/*` → this Worker.

### Option A — wrangler (recommended)
```bash
# wrangler.toml:
#   name = "jabali-update-bridge"
#   main = "worker.js"
#   compatibility_date = "2024-11-01"
#   route = { pattern = "git.jabali-panel.com/api/v1/repos/shukivaknin/jabali2/*", zone_name = "jabali-panel.com" }
wrangler deploy
```

### Option B — Cloudflare API
1. Upload the script:
   `PUT /accounts/{account_id}/workers/scripts/jabali-update-bridge` (body = worker.js, content-type application/javascript+module).
2. Add the route on the zone:
   `POST /zones/{zone_id}/workers/routes`
   `{ "pattern": "git.jabali-panel.com/api/v1/repos/shukivaknin/jabali2/*", "script": "jabali-update-bridge" }`

## Verify BEFORE relying on it

```bash
# Should return the SAME release JSON as Codeberg, with X-Jabali-Bridge: codeberg
curl -s https://git.jabali-panel.com/api/v1/repos/shukivaknin/jabali2/releases?limit=1 | head
curl -sI https://git.jabali-panel.com/api/v1/repos/shukivaknin/jabali2/branches/main | grep -i x-jabali-bridge
# The browser_download_url in the response MUST be a codeberg.org URL.
```

Then do a real `jabali update` (or its dry-run) on a canary host and confirm it
resolves + installs + passes sha256.

## Scope / caveats

- **Only the release API** is proxied. The old Gitea web UI, issues, and git
  clone are NOT — the default (tarball) update path doesn't need them.
- **`jabali update --from-source`** git-clones `git.jabali-panel.com/...` — that
  path still needs the old git server (or its own repoint). The default tarball
  path is what this bridge covers.
- **Retire when the fleet has rolled over** — once hosts report a
  codeberg-pointing version, remove the Worker + the DNS record.
- The Codeberg repo is public, so the bridge needs no token.
