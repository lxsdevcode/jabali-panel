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
