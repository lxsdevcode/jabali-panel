#!/usr/bin/env bash
# Entrypoint for the Jabali CI runner container.
#
# Runs as ROOT first to fix ownership of the runtime-mounted volumes (Docker
# creates named-volume mount points root-owned, which breaks the unprivileged
# `ci` user for Go/npm/Playwright caches AND the actions tool-cache), then
# re-execs itself as `ci` via gosu. This is idempotent and holds for BOTH
# fresh and pre-existing volumes, so no dir-by-dir chowning ever again.
# Job/PR code still runs as `ci` (never root) — same sandboxing as before.
set -euo pipefail

if [[ "$(id -u)" -eq 0 ]]; then
  for d in /home/ci /home/ci/runner /home/ci/toolcache /home/ci/.npm \
           /home/ci/go /home/ci/go/pkg /home/ci/go/pkg/mod \
           /home/ci/.cache /home/ci/.cache/actcache \
           /home/ci/.cache/go-build /home/ci/.cache/ms-playwright; do
    [[ -d "$d" ]] && chown ci:ci "$d"
  done
  exec gosu ci "$0" "$@"
fi

# ---- from here we are the unprivileged `ci` user ----
cd /home/ci/runner

INSTANCE="${FORGEJO_INSTANCE_URL:-https://codeberg.org}"
NAME="${FORGEJO_RUNNER_NAME:-jabali-ci-runner}"
LABELS="${FORGEJO_RUNNER_LABELS:-jabali-ci:host}"

if [[ ! -f .runner ]]; then
  if [[ -z "${FORGEJO_RUNNER_REGISTRATION_TOKEN:-}" ]]; then
    echo "ERROR: no .runner and FORGEJO_RUNNER_REGISTRATION_TOKEN unset — cannot register." >&2
    echo "Get a repo-level token: Codeberg repo -> Settings -> Actions -> Runners -> Create." >&2
    exit 1
  fi
  echo "Registering runner '$NAME' to $INSTANCE with labels '$LABELS'..."
  forgejo-runner register --no-interactive \
    --instance "$INSTANCE" \
    --token "$FORGEJO_RUNNER_REGISTRATION_TOKEN" \
    --name "$NAME" \
    --labels "$LABELS"
fi

# capacity 1: a 2-CPU box oversubscribes `go test -race` alongside the E2E.
if [[ ! -f config.yml ]]; then
  forgejo-runner generate-config > config.yml
  sed -i -E 's/^([[:space:]]*capacity:).*/\1 1/' config.yml
fi

exec forgejo-runner daemon --config config.yml
