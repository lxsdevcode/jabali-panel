#!/usr/bin/env bash
# Entrypoint for the Jabali CI runner container. Registers to Codeberg on
# first boot (idempotent — skips if .runner already exists) then execs the
# daemon in the foreground so the container's lifecycle IS the runner.
set -euo pipefail

cd /home/ci/runner

INSTANCE="${FORGEJO_INSTANCE_URL:-https://codeberg.org}"
NAME="${FORGEJO_RUNNER_NAME:-jabali-ci-runner}"
LABELS="${FORGEJO_RUNNER_LABELS:-jabali-ci:host}"

if [[ ! -f .runner ]]; then
  if [[ -z "${FORGEJO_RUNNER_REGISTRATION_TOKEN:-}" ]]; then
    echo "ERROR: no .runner and FORGEJO_RUNNER_REGISTRATION_TOKEN unset — cannot register." >&2
    echo "Get a repo-level token: Codeberg repo → Settings → Actions → Runners → Create." >&2
    exit 1
  fi
  echo "Registering runner '$NAME' to $INSTANCE with labels '$LABELS'..."
  forgejo-runner register --no-interactive \
    --instance "$INSTANCE" \
    --token "$FORGEJO_RUNNER_REGISTRATION_TOKEN" \
    --name "$NAME" \
    --labels "$LABELS"
fi

# capacity 1: a 2-CPU box oversubscribes `go test -race` alongside the
# Chromium/vite E2E. Generated once; kept if present.
if [[ ! -f config.yml ]]; then
  forgejo-runner generate-config > config.yml
  sed -i -E 's/^([[:space:]]*capacity:).*/\1 1/' config.yml
fi

exec forgejo-runner daemon --config config.yml
