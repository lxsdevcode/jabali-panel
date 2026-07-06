# Jabali CI runner (self-hosted Forgejo Actions for Codeberg)

The project's CI runs on a **self-hosted Forgejo Actions runner** registered to
`codeberg.org/shukivaknin/jabali2`. Codeberg's *hosted* runners can't run our
suite (10-min cap, no cache, alpha), so we bring our own — jobs run on our
hardware, results show in Codeberg.

This dir builds that runner as a container so it's reproducible and
restart-persistent.

## Bring it up

```bash
# On the runner host (Docker + ≥8 GiB RAM, NOT the forge host):
# 1. Codeberg repo → Settings → Actions → Runners → Create new Runner → copy token
echo 'FORGEJO_RUNNER_REGISTRATION_TOKEN=<token>' > .env && chmod 600 .env
# 2. build + run
docker compose up -d --build
# 3. confirm online: Codeberg repo → Actions → Runners  (label: jabali-ci)
```

The runner registers once (persisted on the `runner-state` volume) and runs the
daemon as the container's main process, so it survives restarts.

## Design decisions (each one cost a debugging cycle on 2026-07-06)

| Choice | Why |
|---|---|
| **host-execution** (`jabali-ci:host`), not docker-in-docker | Codeberg doesn't support a Docker daemon in the runner; jobs run in-container against the baked toolchain. Also gives each run its own netns → kills the vite port-4173 races. |
| **build-essential in the image** | `go test -race` requires cgo → a C compiler. Without it: `go: -race requires cgo`. |
| **Go pinned to 1.25** | `go.mod` declares `go 1.25.0`; a 1.24 toolchain fails `go.mod requires go >= 1.25.0` on a strict (`GOTOOLCHAIN=local`) runner. Keep the workflow `GO_VERSION` and the image `GO_VERSION` in lockstep with go.mod. |
| **capacity 1** | 2-CPU boxes oversubscribe `go test -race` when it runs alongside the Chromium/vite E2E → timeouts. Raise only with more CPU. |
| **Playwright deps baked, browser on a volume** | No `actions/cache` needed; the browser caches in `~/.cache/ms-playwright` across runs. |

## Workflows

`.forgejo/workflows/{ci,nightly,release}.yml` — ported from the old
`.gitea/workflows`, `runs-on: jabali-ci`. Keep them and go.mod's Go version in
sync with this image's `GO_VERSION`.

## Security notes

A CI runner executes code from PRs, so treat it as semi-trusted:

- **No sudo in the image** — the `ci` user has no root path; build-time deps are
  baked (Dockerfile RUN is already root). A PR cannot escalate in-container.
- **Persistent caches are a poisoning vector** — the shared `go-mod` / `npm` /
  `playwright` volumes speed repeat runs, but a malicious PR could write a
  tainted artifact that a later run consumes. **Mitigation (operator, on
  Codeberg):** repo → Settings → Actions → require approval to run workflows
  from **non-collaborators / first-time contributors**, so untrusted PRs don't
  auto-run on the runner. This is why Codeberg's *hosted* runners disable
  caching entirely; we accept the tradeoff behind the approval gate.
- **Registration token** is passed via a root-only `.env` (never committed) and
  is only needed at *first* registration — once `.runner` exists on the
  `runner-state` volume, you can drop `FORGEJO_RUNNER_REGISTRATION_TOKEN`.
- **Pinned downloads** — Go, Node, forgejo-runner, Playwright are all
  version-pinned via ARGs. Hardening TODO: verify the forgejo-runner `.asc`
  signature (published alongside the binary) and the Go tarball sha256 at build.
- **Isolation** — run this on a dedicated box, not a prod/forge host, so a
  runner compromise can't reach production or the git server.
