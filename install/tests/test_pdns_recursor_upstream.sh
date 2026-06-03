#!/usr/bin/env bash
# install/tests/test_pdns_recursor_upstream.sh — regression coverage
# for GH-126 (2026-06-03): the pdns-recursor recurse upstream must
# NEVER be 127.0.0.53 in install.sh's default branch (no DNS_FORWARDER).
#
# Why this matters: install.sh's zz-jabali-recursor.conf drop-in resets
# systemd-resolved's DNS to 127.0.0.1 — which is the recursor itself.
# If recursor forwards to 127.0.0.53 (= resolved stub), the chain
# closes a fatal loop:
#
#   resolved (127.0.0.53)
#     -> upstream 127.0.0.1 (recursor)
#       -> forward-zones-recurse=.=127.0.0.53 (resolved)
#         -> ... timeout, Probe 1 dies, installer aborts.
#
# This test inspects install.sh's source to assert the default-branch
# upstream is a routable public DNS, not the loopback stub. Static
# string match is enough — the install.sh write path renders this
# value verbatim into /etc/powerdns/recursor.conf.
#
# Run from repo root:
#     bash install/tests/test_pdns_recursor_upstream.sh
#
# Exit 0 = pass. Non-zero = the loop bug regressed.

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" >/dev/null && pwd)"
INSTALL_SH="$SCRIPT_DIR/../../install.sh"

if [[ ! -f "$INSTALL_SH" ]]; then
  printf '[FAIL] cannot find %s — run from repo root\n' "$INSTALL_SH" >&2
  exit 1
fi

PASS_COUNT=0
FAIL_COUNT=0
FAILED_TESTS=()

pass() {
  PASS_COUNT=$((PASS_COUNT + 1))
  printf '\033[1;32m[PASS]\033[0m %s\n' "$1"
}

fail() {
  FAIL_COUNT=$((FAIL_COUNT + 1))
  FAILED_TESTS+=("$1")
  printf '\033[1;31m[FAIL]\033[0m %s — %s\n' "$1" "$2" >&2
}

# Extract the default-branch upstream assignment from install.sh.
# Two assignments exist inside install_pdns_recursor() — one inside
# the `if [[ -n "$DNS_FORWARDER" ]]` branch (value=$DNS_FORWARDER)
# and one in the else-branch (the default, what we want). The
# else-branch one is the unique `_rec_recurse_upstream="..."` line
# whose value is NOT literally `$DNS_FORWARDER`.
default_upstream="$(grep -E '_rec_recurse_upstream="[^"]+"' "$INSTALL_SH" \
  | grep -v '_rec_recurse_upstream="\$DNS_FORWARDER"' \
  | sed -nE 's/.*_rec_recurse_upstream="([^"]+)".*/\1/p' \
  | head -1)"

# ----- test 1: default upstream is NOT the resolved stub --------------
if [[ "$default_upstream" == "127.0.0.53" ]]; then
  fail "default upstream is not the resolved stub" \
       "found _rec_recurse_upstream=\"127.0.0.53\" — recreates the loop fixed in GH-126"
else
  pass "default upstream is not the resolved stub"
fi

# ----- test 2: default upstream is non-empty --------------------------
if [[ -z "$default_upstream" ]]; then
  fail "default upstream is non-empty" \
       "couldn't locate _rec_recurse_upstream assignment in the else-branch — install.sh structure changed?"
else
  pass "default upstream is non-empty (value: $default_upstream)"
fi

# ----- test 3: default upstream is a routable address ----------------
# Accept either a single IP or semicolon-separated list. Each entry
# must NOT be in the loopback range (127.0.0.0/8) — loopback there
# can only mean "another local daemon", and the only local daemons
# on :53 in the jabali stack are the recursor (would self-recurse)
# or the resolved stub (would loop, the GH-126 case).
loopback_seen=0
IFS=';' read -ra hosts <<< "$default_upstream"
for h in "${hosts[@]}"; do
  h_trim="${h//[[:space:]]/}"
  case "$h_trim" in
    127.*) loopback_seen=1; bad_host="$h_trim"; break ;;
    "")    : ;;  # trailing semicolon, ignore
  esac
done
if (( loopback_seen )); then
  fail "default upstream avoids loopback" \
       "entry '$bad_host' is in 127.0.0.0/8 — would chain back into the local stack"
else
  pass "default upstream avoids loopback (entries: $default_upstream)"
fi

# ---------------------------------------------------------------------

printf '\n'
if (( FAIL_COUNT == 0 )); then
  printf '\033[1;32mAll %d tests passed.\033[0m\n' "$PASS_COUNT"
  exit 0
else
  printf '\033[1;31m%d passed, %d failed.\033[0m\n' "$PASS_COUNT" "$FAIL_COUNT"
  printf 'Failed:\n'
  for t in "${FAILED_TESTS[@]}"; do printf '  - %s\n' "$t"; done
  exit 1
fi
