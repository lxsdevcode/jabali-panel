#!/usr/bin/env bash
# certbot --manual-auth-hook for a free jabalihosted.com wildcard cert
# (JAB-213 phase 3b). certbot calls this once per -d name; both the apex and
# the wildcard produce a challenge at the SAME _acme-challenge.<label> name
# with different values, so the service's /v1/acme/present APPENDS (it does not
# replace) — see hostedsvc SetChallenge. Env from certbot: CERTBOT_DOMAIN,
# CERTBOT_VALIDATION.
set -euo pipefail

ENV=/etc/jabali-panel/hostname.env
[[ -r "$ENV" ]] || { echo "certbot-auth-hook: $ENV missing — box has no free hostname" >&2; exit 1; }
# shellcheck disable=SC1090
source "$ENV"
: "${TOKEN:?}" "${API:?}"

code=$(curl -sS --max-time 30 -o /dev/null -w '%{http_code}' \
  -H 'Content-Type: application/json' \
  -d "{\"token\":\"${TOKEN}\",\"txt\":\"${CERTBOT_VALIDATION}\"}" \
  "${API}/v1/acme/present")
[[ "$code" == "200" ]] || { echo "certbot-auth-hook: present failed ($code)" >&2; exit 1; }

# Give the record time to be live on Cloudflare's anycast edge before certbot
# asks Let's Encrypt to validate. CF propagation is seconds; LE also retries.
sleep 20
