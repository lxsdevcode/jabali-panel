#!/usr/bin/env bash
# install/tests/test_mariadb_tuning.sh — regression coverage for
# 2026-06-05 puzzle OOM incident: kernel killed mariadbd twice in one
# morning on a 3.8 GB-RAM VPS with no swap because nothing tuned
# innodb_buffer_pool_size against the host's actual RAM.
#
# Asserts install.sh ships:
#   1. A tune_mariadb_for_ram function with the documented bracket map.
#   2. An ensure_swap call early in the installer ordering (not just
#      from build_frontend), so swap exists BEFORE daemons start.
#   3. ensure_swap's threshold is widened to 4 GB (was 2 GB pre-fix).
#   4. tune_mariadb_for_ram wired into the main install ordering.
#
# Run from repo root:
#     bash install/tests/test_mariadb_tuning.sh
#
# Exit 0 = pass.
set -euo pipefail

cd "$(dirname "$0")/../.."

fail=0

# --- 1. Function exists + has the bracket map. ---
if ! grep -q '^tune_mariadb_for_ram() {' install.sh; then
  echo "FAIL: tune_mariadb_for_ram function not defined in install.sh"
  fail=1
fi
for bracket in '\[\[ \$mem_mb -le 2048' '\[\[ \$mem_mb -le 4096' '\[\[ \$mem_mb -le 8192' '\[\[ \$mem_mb -le 16384'; do
  if ! grep -Eq "$bracket" install.sh; then
    echo "FAIL: missing bracket: $bracket"
    fail=1
  fi
done

# --- 2. ensure_swap threshold widened to 4 GB. ---
if ! grep -Fq 'if [[ $mem_mb -gt 4096 ]]; then' install.sh; then
  echo "FAIL: ensure_swap threshold not widened to 4 GB"
  fail=1
fi
if grep -Fq 'if [[ $mem_mb -gt 2048 ]]; then' install.sh; then
  echo "FAIL: ensure_swap still uses the old 2 GB threshold"
  fail=1
fi

# --- 3. ensure_swap called early in the installer (NOT only from build_frontend). ---
# build_frontend already calls it. We want at least one OTHER call site
# in the main installer ordering, before mariadb provisioning.
count=$(grep -cE '^\s+ensure_swap\s*$' install.sh || true)
if [[ "$count" -lt 2 ]]; then
  echo "FAIL: ensure_swap called only $count time(s); expected >= 2 (one early + one from build_frontend)"
  fail=1
fi

# --- 4. tune_mariadb_for_ram wired into the main installer. ---
if ! grep -qE '^\s+tune_mariadb_for_ram\s*$' install.sh; then
  echo "FAIL: tune_mariadb_for_ram is defined but never called"
  fail=1
fi

# --- 5. Ordering: tune_mariadb_for_ram comes after install_mariadb_skip_networking. ---
nw=$(grep -n '^\s*install_mariadb_skip_networking\s*$' install.sh | tail -n1 | cut -d: -f1 || true)
tn=$(grep -n '^\s*tune_mariadb_for_ram\s*$' install.sh | tail -n1 | cut -d: -f1 || true)
if [[ -z "$nw" || -z "$tn" || "$tn" -le "$nw" ]]; then
  echo "FAIL: tune_mariadb_for_ram call site (line $tn) must come after install_mariadb_skip_networking (line $nw)"
  fail=1
fi

# --- 6. OOMScoreAdjust drop-in written. ---
if ! grep -q '15-jabali-oom.conf' install.sh; then
  echo "FAIL: OOMScoreAdjust drop-in path not present"
  fail=1
fi
if ! grep -q 'OOMScoreAdjust=-500' install.sh; then
  echo "FAIL: OOMScoreAdjust=-500 not present in install.sh"
  fail=1
fi

if [[ "$fail" -ne 0 ]]; then
  echo
  echo "FAIL: install.sh MariaDB right-sizing regressed"
  exit 1
fi

echo "PASS: MariaDB right-sizing + early swap call all in place"
