# Jabali CI runner image — self-hosted Forgejo Actions runner for Codeberg.
#
# Runs the runner in HOST-EXECUTION mode (jobs execute directly in this
# container's filesystem — no docker-in-docker). The whole toolchain the
# workflows need is baked in so a job never apt-installs or re-downloads a
# browser mid-run. Reproduces, byte-for-byte, the box that runs
# .forgejo/workflows.
#
# Why each dependency (learned the hard way bringing the runner up 2026-07-06):
#   - build-essential  : `go test -race` REQUIRES cgo → a C compiler. A bare
#                        container fails with "go: -race requires cgo; enable
#                        cgo by setting CGO_ENABLED=1". This bit us first.
#   - Go 1.25          : go.mod declares `go 1.25.0`. A 1.24 toolchain fails
#                        with "go.mod requires go >= 1.25.0" unless it can
#                        auto-download (GOTOOLCHAIN); pin the real version.
#   - procps, iproute2 : the E2E "Free port 4173" step uses pgrep/pkill/ss.
#   - Playwright + deps : Chromium + its ~30 system libs for the E2E job.
#
# Runner capacity is 1 (see config.yml): a 2-CPU box oversubscribes
# `go test -race` if it runs concurrently with the Chromium/vite E2E.

FROM debian:trixie-slim

ARG GO_VERSION=1.25.11
ARG NODE_MAJOR=20
ARG FORGEJO_RUNNER_VERSION=12.12.0
ARG PLAYWRIGHT_VERSION=1.55.0

ENV DEBIAN_FRONTEND=noninteractive \
    PATH=/usr/local/go/bin:/home/ci/go/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin

# 1. Base tools + build-essential (cgo/-race) + Playwright/browser prereqs.
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl wget git bash make zstd xz-utils \
      procps iproute2 build-essential passwd \
    && rm -rf /var/lib/apt/lists/*

# 2. Go (pinned to match go.mod).
RUN curl -fsSL -o /tmp/go.tgz "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" \
    && tar -C /usr/local -xzf /tmp/go.tgz && rm /tmp/go.tgz \
    && go version

# 3. Node 20 (NodeSource).
RUN curl -fsSL "https://deb.nodesource.com/setup_${NODE_MAJOR}.x" | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/* && node --version

# 4. Forgejo Actions runner.
RUN curl -fsSL -o /usr/local/bin/forgejo-runner \
      "https://code.forgejo.org/forgejo/runner/releases/download/v${FORGEJO_RUNNER_VERSION}/forgejo-runner-${FORGEJO_RUNNER_VERSION}-linux-amd64" \
    && chmod +x /usr/local/bin/forgejo-runner && forgejo-runner --version

# 5. Non-root ci user (jobs run as this user). NO sudo grant: the workflows
#    never sudo (deps are baked at build time, which is already root), so a
#    runtime root path would only widen the blast radius of PR-supplied code.
RUN useradd -m -u 1000 -s /bin/bash ci \
    && install -d -o ci -g ci \
         /home/ci/.cache /home/ci/.cache/actcache /home/ci/.cache/go-build \
         /home/ci/.cache/ms-playwright /home/ci/.npm \
         /home/ci/go /home/ci/go/pkg /home/ci/go/pkg/mod

# 6. Playwright OS deps (root) — the browser binary itself is fetched by the
#    job (cached in /home/ci/.cache/ms-playwright via the volume mount).
RUN npx --yes playwright@${PLAYWRIGHT_VERSION} install-deps chromium \
    && rm -rf /var/lib/apt/lists/*

USER ci
WORKDIR /home/ci/runner

# The runner is registered at first boot (see entrypoint.sh) using
# FORGEJO_RUNNER_REGISTRATION_TOKEN, then runs the daemon in the foreground
# so the container's lifecycle IS the runner (auto-restarts with the container,
# unlike the sshd-PID1 setup where the daemon didn't survive a restart).
COPY --chown=ci:ci entrypoint.sh /home/ci/entrypoint.sh
ENTRYPOINT ["/home/ci/entrypoint.sh"]
