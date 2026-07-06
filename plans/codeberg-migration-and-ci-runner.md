# Handoff: Migrate jabali2 forge → Codeberg + containerized CI runner

**Audience:** a coding agent with NO prior context on this project. Read this
whole file before touching anything. Everything you need is here or pointed to.

**Status:** DRAFT plan, not started. Author handed off 2026-07-06.

---

## 0. TL;DR of the mission

The project's self-hosted Gitea (`git.linux-hosting.co.il` /
`git.jabali-panel.com`, behind Cloudflare) is a **single point of failure**: it
hosts git + issues + CI + the **release tarballs that `jabali update` pulls**.
It went fully down (web+API+SSH all timing out) and took customer updates with
it. We are moving OFF it.

**Target end state:**
- **Git + issues + releases → Codeberg** (`codeberg.org/shukivaknin/jabali2`).
  Codeberg runs **Forgejo** (Gitea fork, same `/api/v1` API), so existing
  tooling ports with URL swaps.
- **CI → a dedicated VPS running a containerized Forgejo Actions runner** (the
  operator is provisioning this VPS; get its access details from the operator).
  Codeberg's *hosted* runners CANNOT run our suite (10-min cap, no cache,
  alpha) — we self-host the runner. Our workflows are Gitea-Actions-compatible,
  so `.gitea/workflows/` → `.forgejo/workflows/` is nearly a copy.
- **`jabali update` → Codeberg releases** (careful step — field hosts have the
  old URL baked into their binaries).
- **We are deliberately NOT using GitHub** (proprietary/Microsoft; the project
  is AGPL-3.0 and the operator wants FOSS independence). GitHub stays only as a
  passive mirror if at all.

---

## 1. Why these choices (so you don't relitigate them)

- **License is AGPL-3.0, repo is already public** (it's mirrored public on
  GitHub today). So Codeberg's FOSS-only + public-repo Terms of Use are
  satisfied. No proprietary-exposure problem.
- **Codeberg hosted CI is out.** Per Codeberg docs (verify at
  <https://codeberg.org/actions/meta> and <https://docs.codeberg.org/ci/>):
  hosted Forgejo Actions runners cap at `codeberg-medium` = 4 CPU / 8 GB /
  **10 min**, **no `actions/cache`**, no Docker daemon, open-alpha ("expect
  downtime"). Our Playwright E2E alone (Chromium + vite build + 35 specs)
  exceeds that with no cache. Hosted Woodpecker needs a manual approval form
  and a different pipeline syntax. → **Self-hosted runner is the only fit.**
- **Containerized runner, host-execution mode, in a purpose-built image** —
  NOT docker-in-docker. Rationale: our jobs need a browser + system libs +
  host-port management; baking the toolchain into one image and running jobs
  in-container is simpler and faster than spawning a fresh job container per
  step (and Codeberg notes Docker-daemon-in-runner is unsupported anyway). A
  container also gives each run its own netns → **kills the port-4173 races**
  that plague the current host-mode runner, and makes `--capacity 2`
  (concurrent jobs) trivial.

---

## 2. Prerequisites & credentials

- **Working copy:** `/home/shuki/projects/jabali2` (this repo). Default branch
  `main`.
- **Codeberg account:** `shukivaknin` (id 1023186, shukivaknin@gmail.com).
- **Codeberg API token:** in `~/cred.md`, line `codeberg API Key:`. Read it at
  runtime; NEVER hardcode it into a file, commit, or log. Extract with:
  ```bash
  CB=$(grep -i 'codeberg API Key' ~/cred.md | grep -oE '[0-9a-f]{40}')
  # verify: curl -s -H "Authorization: token $CB" https://codeberg.org/api/v1/user
  ```
- **CI VPS:** operator is provisioning a fresh VPS (needs Docker + ≥4 GB RAM
  free for E2E). Get SSH access from the operator. It must NOT be the Gitea VM
  (defeats the purpose) and should not carry prod load.
- **Old Gitea API token** (for reading the 6 issues to migrate, IF it comes
  back up): `~/cred.md` line `gitea API key:`, or `~/.git-credentials` for
  `git.linux-hosting.co.il`.

---

## 3. HARD RULES (violating these breaks prod or the team)

1. **This is a live product's release pipeline. Do steps in order. Verify each
   before the next.** Especially step 6 (`jabali update` repoint) — a wrong
   move there bricks every customer's updater.
2. **Never commit directly to `main`.** Work on feature branches; the operator
   merges. Never force-push `main`. Never `git push --force` a shared branch
   without `--force-with-lease` and a reason.
3. **Secrets:** the Codeberg token and any runner registration token are
   secrets. Keep them in env / the runner's config file (root-only perms),
   never in a committed file or a log line.
4. **Don't rewrite git history** on any pushed branch.
5. **AGPL-3.0 + Codeberg TOS fair-use:** the runner is ours (no Codeberg
   resource concern), but keep the repo public and AGPL-licensed.
6. **Pin from current docs, not memory.** Forgejo runner image tags and
   registration flags change. Before writing the Dockerfile/compose/config,
   read the current Forgejo runner docs (links in step 3). Do not trust the
   example snippets in this file blindly — they are a STARTING POINT to verify.
7. **The workflow files to port are the OPTIMIZED ones on branch
   `chore/ci-optimize`, NOT the older versions on `main`.** See step 4.

---

## 4. Current in-flight state you must know

- **Remotes today:** `origin` = dead Gitea (`gitea:shukivaknin/jabali2.git`
  push alias, port 2222), `github` = `git@github.com:shukiv/jabali-panel.git`
  (public mirror). You will ADD a `codeberg` remote.
- **Unmerged PR branches** exist locally and on the remotes (e.g.
  `chore/ci-optimize`, `fix/apparmor-dbus-service-state`,
  `ux/crowdsec-merge-cards-740`, `fix/hestia-import-tar-path-glob-327`,
  `fix/stalwart-alias-recipient-query-346`, `fix/service-down-false-alarm-empty-state`,
  plus many `feat-*` / `feat-m49-*`). **Mirror-push must preserve ALL branches
  and tags** so no work is lost. These map to Gitea PRs #744–#750 that were
  mid-landing when Gitea died; the operator will recreate PRs on Codeberg.
- **CI optimization lives in `chore/ci-optimize`** (Gitea PR #750), NOT on
  `main`. It contains the tuned `.gitea/workflows/ci.yml` (path-filter gate job
  + Playwright `workers:2` + `playwright install chromium` without
  `--with-deps`) and a new `.gitea/workflows/nightly.yml`, plus
  `panel-ui/playwright.config.ts` `workers: CI?2`. **Port THESE, not main's.**
- **Release source constant:** `panel-api/cmd/server/update_release.go:51`
  `defaultReleaseAPIBase = "https://git.jabali-panel.com/api/v1/repos/shukivaknin/jabali2"`,
  overridable via env `JABALI_RELEASE_API_BASE` (read at line ~88).
- **6 open Gitea issues** to migrate (fetch list once Gitea or a backup is
  reachable: `GET /api/v1/repos/shukivaknin/jabali2/issues?type=issues&state=open`).

---

## Step 1 — Create the Codeberg repo + mirror-push everything (SAFE, do first)

**Goal:** a complete, working copy of the repo on Codeberg — all branches, all
tags. Reversible, no pipeline impact.

**Tasks:**
1. Create the repo via API (private=false; AGPL is fine on Codeberg):
   ```bash
   CB=$(grep -i 'codeberg API Key' ~/cred.md | grep -oE '[0-9a-f]{40}')
   curl -s -X POST -H "Authorization: token $CB" -H "Content-Type: application/json" \
     https://codeberg.org/api/v1/user/repos \
     -d '{"name":"jabali2","private":false,"default_branch":"main","description":"Jabali hosting control panel (AGPL-3.0)"}'
   ```
2. Add the remote and push a full mirror (all refs + tags). Use a token-in-URL
   push or an SSH key the operator adds to Codeberg. Token-URL example (do NOT
   commit the URL):
   ```bash
   cd /home/shuki/projects/jabali2
   git remote add codeberg "https://shukivaknin:${CB}@codeberg.org/shukivaknin/jabali2.git"
   git push codeberg --all
   git push codeberg --tags
   ```
   (Prefer configuring an SSH deploy key on Codeberg over a token-in-URL for a
   persistent remote; see <https://docs.codeberg.org/security/ssh-key/>.)

**Verify:**
```bash
curl -s -H "Authorization: token $CB" https://codeberg.org/api/v1/repos/shukivaknin/jabali2/branches | python3 -c "import sys,json;print('branches on codeberg:',len(json.load(sys.stdin)))"
```
Expect the branch count to match `git branch -r | grep -c origin/` (roughly).

**Rollback:** delete the Codeberg repo (API `DELETE /repos/shukivaknin/jabali2`).
Nothing local changes.

---

## Step 2 — Build the CI runner image (toolchain baked in)

**Goal:** a Docker image that contains everything our jobs need, so runs never
apt-install or re-download a browser. Lives in-repo at `ci/runner/`.

**Tasks:** create `ci/runner/Dockerfile`. It must provide:
- Base: Debian 13 (bookworm/trixie to match prod) or `catthehacker/ubuntu`-style
  base.
- **Go 1.24** (matches `GO_VERSION` in the workflows).
- **Node 20** (matches `NODE_VERSION`).
- **Playwright Chromium + its system deps** (`npx playwright install --with-deps
  chromium` once at build time — so runtime installs are no-ops).
- `procps`, `iproute2` (the workflows use `pgrep`/`pkill`/`ss` for the
  port-4173 cleanup), `git`, `bash`, `make`, `curl`, `ca-certificates`, `zstd`.
- The **Forgejo Actions runner** binary (`forgejo-runner`). **VERIFY the current
  image/binary source and version** at
  <https://code.forgejo.org/forgejo/runner> and the Codeberg self-hosted guide
  <https://docs.codeberg.org/ci/actions/>. Do not guess the tag.

**Verify:** `docker build -t jabali-ci-runner ci/runner/` succeeds; a shell in
the image has `go version` = 1.24.x, `node -v` = v20.x, `npx playwright
--version` works, `pgrep`/`ss` present.

---

## Step 3 — Run + register the runner on the VPS (host-exec mode)

**Goal:** the containerized runner registers to the Codeberg repo and picks up
jobs, executing them **in-container** (host label), with persistent caches.

**Tasks:** create `ci/runner/docker-compose.yml` (and a `config.yml`). It must:
1. Get a **repo-level registration token** from Codeberg: repo Settings →
   Actions → Runners → Create new Runner (or the API). Actions must be **enabled**
   on the repo first (repo Settings → Units → enable Actions).
2. Register the runner with a **host-execution label** so jobs run in the image
   directly (not a nested container). In Forgejo runner config, labels map like
   `LABEL:host` for host exec vs `LABEL:docker://IMAGE` for docker exec — **VERIFY
   the exact syntax in the current runner docs.** Pick a label, e.g.
   `jabali-ci` (host). Registration sketch (verify flags):
   ```bash
   forgejo-runner register --no-interactive \
     --instance https://codeberg.org \
     --token "$RUNNER_TOKEN" \
     --name jabali-ci-vps \
     --labels jabali-ci:host
   ```
3. Mount **persistent named volumes** into the job workspace for
   `GOMODCACHE` (`/root/go/pkg/mod` or the image's GOPATH), `~/.npm`, and
   `~/.cache/ms-playwright` so repeat runs are fast.
4. Give the container ≥2 CPU / ≥4 GB and set runner **capacity** (e.g. 2) so the
   4 jobs can run concurrently. Restart policy `unless-stopped`.
5. Keep the registration token in the compose env / a root-only `.env` file on
   the VPS, **never committed**.

**Verify:** the runner shows **online** in the Codeberg repo Actions → Runners
page (or `GET /repos/.../actions/runners`). Trigger a trivial test workflow and
confirm it executes on `jabali-ci`.

---

## Step 4 — Port the workflows to `.forgejo/workflows/`

**Goal:** CI runs on Codeberg using our OPTIMIZED workflows.

**Tasks:**
1. Base off branch **`chore/ci-optimize`** (has the tuned ci.yml + nightly.yml +
   playwright workers:2), not `main`.
2. Copy `.gitea/workflows/{ci,nightly,release}.yml` →
   `.forgejo/workflows/`. Codeberg/Forgejo reads `.forgejo/workflows/` (it also
   reads `.gitea/workflows/`, but standardize on `.forgejo/`).
3. Change every `runs-on: ubuntu-latest` → `runs-on: jabali-ci` (the label from
   step 3). Keep the `changes` path-filter gate job, the `workers:2` E2E, and
   `playwright install chromium` (no `--with-deps`, since the image has deps).
4. Because the runner image is persistent + caches are volume-mounted, the
   "host-mode setup-go cache: false" reasoning still applies — keep `cache:
   false` on setup-go and rely on the mounted `GOMODCACHE`.
5. Decide whether to delete `.gitea/workflows/` (avoid double-runs if the old
   Gitea ever returns) — recommended once Codeberg is authoritative.

**Verify:** open a test PR on Codeberg touching a Go file only → confirm E2E is
SKIPPED (green) and Go job runs; touch a `panel-ui/**` file → confirm E2E runs.
Confirm required status contexts post green. Confirm a full run finishes in a
sane time (E2E parallelized, browser cached).

---

## Step 5 — Release publishing → Codeberg

**Goal:** `release.yml` publishes the tarball to Codeberg Releases (what
`jabali update` will consume).

**Tasks:**
1. Port `release.yml` to `.forgejo/workflows/`. It currently POSTs to the Gitea
   Releases API using the auto-injected `GITHUB_TOKEN`/`secrets` — on Forgejo
   the auto-injected token + `/api/v1/repos/${REPO}/releases` API are the same
   shape, so it should port with the `runs-on` label change. **Verify the
   token/permissions** — Forgejo Actions injects a token with `contents:write`;
   confirm it can create releases + upload assets on Codeberg.
2. Keep the tag format `release-<short_sha>` and asset names
   `jabali-release-<short_sha>.tar.gz[.sha256]` — `jabali update` looks these up
   by name (see `panel-api/cmd/server/update_release.go`). DO NOT rename them.
3. `tools/build-release.sh` is the build; it runs identically in the new image.

**Verify:** a push to `main` on Codeberg cuts `release-<sha>` with both assets;
`curl .../api/v1/repos/shukivaknin/jabali2/releases/tags/release-<sha>` returns
them.

---

## Step 6 — Repoint `jabali update` to Codeberg (DANGER: field hosts)

**Goal:** future updates pull from Codeberg. **This is the one step that can
brick customer updaters — read the chicken-and-egg first.**

**The chicken-and-egg:** every deployed host has the OLD default
(`git.jabali-panel.com`) compiled into its `jabali-panel` binary. A host only
picks up the new (Codeberg) default AFTER it updates once to a binary that has
it — and that update must come from the OLD source. So:

**Tasks (in this order):**
1. Change `panel-api/cmd/server/update_release.go:51`
   `defaultReleaseAPIBase` → `https://codeberg.org/api/v1/repos/shukivaknin/jabali2`.
   Keep the `JABALI_RELEASE_API_BASE` env override (line ~88) intact.
2. Also update any comments/ldflags referencing the old host where relevant
   (the Go module path `git.jabali-panel.com/...` is a SEPARATE concern — that's
   the import path, changing it is a large mechanical rename; do NOT do it as
   part of this step unless the operator asks).
3. **Transition for existing hosts — pick ONE with the operator:**
   - **(a) DNS repoint** `git.jabali-panel.com` → Codeberg: does NOT work
     cleanly (Codeberg won't serve that Host/SNI + TLS). Reject unless a proxy
     is stood up.
   - **(b) Bridge release:** while the OLD Gitea is briefly restored, publish
     ONE release there built from the repointed binary, so hosts update once,
     get the Codeberg-aware binary, and thereafter pull from Codeberg. **This is
     the recommended path** — it needs the old Gitea up for one release cycle.
   - **(c) Operator-set env:** push `JABALI_RELEASE_API_BASE=https://codeberg.org/...`
     to hosts via config management as a stopgap.
4. Ship the change through CI (feature branch → operator merges).

**Verify:** on a test host, `jabali update` (or its dry-run) resolves the latest
`release-<sha>` from Codeberg and installs. Confirm sha256 verification passes.

---

## Step 7 — Migrate the 6 open issues

**Goal:** preserve the open issues on Codeberg.

**Tasks:**
1. Fetch open issues from the old Gitea (when reachable) or a backup:
   `GET https://git.linux-hosting.co.il/api/v1/repos/shukivaknin/jabali2/issues?type=issues&state=open&limit=50`.
2. Recreate each on Codeberg via `POST /api/v1/repos/shukivaknin/jabali2/issues`
   (title + body; append a footnote linking the original). Preserve labels
   (create labels first if needed).
3. **Tracker-labeling note:** this project historically had TWO trackers
   (self-hosted Gitea + a GitHub mirror) with independent numbering; commit
   messages use `(GH #N)` for github issues and `(Gitea #N)` for Gitea issues.
   After migration, the canonical tracker is Codeberg — agree a convention with
   the operator (e.g. `(#N)` = Codeberg) and note it. Do NOT assume a number is
   GitHub's.

**Verify:** issue count on Codeberg matches the 6 open ones; titles/bodies intact.

---

## Step 8 — Cutover & decommission

**Tasks:**
1. Point `origin` at Codeberg; keep GitHub as an optional passive mirror push
   (the project pushes to multiple remotes today).
2. Update repo docs / CONTRIBUTING / any `git.jabali-panel.com` references to
   Codeberg (grep the repo; the Go module import path is out of scope unless
   asked).
3. Recreate branch protection on Codeberg (require CI green; the operator sets
   access controls — you do NOT change access controls yourself, flag them).
4. Recreate the mid-landing PRs (#744–#750 equivalents) on Codeberg from their
   branches, or let the operator do it.
5. Once verified, the operator decides whether to keep the old Gitea as a cold
   mirror or retire it.

---

## Appendix — Gotchas learned the hard way (save yourself the cycles)

- **Codeberg hosted runners can't run this suite** (10-min cap, no cache). Use
  our own runner. (This is WHY the container exists.)
- **Port 4173 races:** the old host-mode runner had EADDRINUSE races on the
  vite-preview port across concurrent runs; the container's own netns fixes it —
  but keep the "Free port 4173" step anyway as a guard.
- **setup-go/setup-node `cache:` layers conflict with a persistent
  `$GOMODCACHE`/`~/.npm`** ("File exists" tar error) — that's why the workflows
  use `cache: false` on setup-go and rely on the persistent cache. Preserve this
  with volume mounts, don't re-enable `actions/cache`.
- **Playwright:** config sets `retries: 3`, `workers: CI?2`,
  `serviceWorkers:"block"`; specs mock the API per-test via `page.route` (no
  shared backend), so `workers:2` is safe.
- **Release asset names + tag format are load-bearing** for `jabali update` —
  never rename `jabali-release-<short_sha>.tar.gz` / `release-<short_sha>`.
- **Dual/triple remotes:** the project pushes to multiple remotes; when you add
  Codeberg, decide the authoritative one and keep pushes consistent.
- **Don't touch `main` directly; don't change access controls; don't hardcode
  secrets.** (Restated because it matters.)
- **Forgejo runner exact image/flags:** VERIFY at
  <https://code.forgejo.org/forgejo/runner> + <https://docs.codeberg.org/ci/actions/>.
  This file's snippets are starting points, not gospel.

---

## Open decisions for the operator (surface these, don't guess)

1. **VPS specs/access** for the runner (Docker + ≥4 GB RAM; SSH details).
2. **Step 6 transition** (a/b/c) — recommended (b) bridge-release needs the old
   Gitea up for one cycle. Confirm.
3. **Keep GitHub mirror or drop it entirely.**
4. **Retire the old self-hosted Gitea, or keep it as a cold mirror.**
5. **Go module import path** (`git.jabali-panel.com/...`) — rename to Codeberg
   now, or leave (it's a big mechanical change, separate from this migration).
6. **Branch protection / required checks** on Codeberg (operator sets these).
