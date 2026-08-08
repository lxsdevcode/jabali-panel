#!/usr/bin/env bash
# certbot --manual-cleanup-hook for the free jabalihosted.com wildcard cert
# (JAB-213 phase 3b). Removes every challenge TXT at the label's name.
# Idempotent: certbot calls it once per -d name; /v1/acme/cleanup clears all.
set -euo pipefail

ENV=/etc/jabali-panel/hostname.env
[[ -r "$ENV" ]] || exit 0
# shellcheck disable=SC1090
source "$ENV"
: "${TOKEN:?}" "${API:?}"

curl -sS --max-time 30 -o /dev/null \
  -H 'Content-Type: application/json' \
  -d "{\"token\":\"${TOKEN}\"}" \
  "${API}/v1/acme/cleanup" || true
