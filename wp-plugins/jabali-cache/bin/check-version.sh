#!/usr/bin/env bash
# GH #627 — release consistency guard for the jabali-cache plugin. Verifies the
# version + wp.org package metadata agree across every touch point, so drift is
# caught in CI (build-release.sh calls this) rather than only at SVN deploy time.
#
# Checks:
#   1. Version identical across: plugin header `Version`, `JABALI_CACHE_VERSION`,
#      readme.txt `Stable tag`.
#   2. readme.txt has a `= <version> =` changelog section AND CHANGELOG.md has a
#      `## [<version>]` heading for it.
#   3. wp.org metadata (`Requires at least`, `Requires PHP`) match between the
#      plugin header and readme.txt.
set -euo pipefail

SRC_DIR="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
die() { echo "check-version: $*" >&2; exit 1; }

hdr()    { grep -oiP "^\s*\*\s*$1:\s*\K.+" "${SRC_DIR}/jabali-cache.php" | head -1 | tr -d '[:space:]'; }
readme() { grep -oiP "^$1:\s*\K.+"        "${SRC_DIR}/readme.txt"       | head -1 | tr -d '[:space:]'; }

HDR_VER="$(hdr Version)"
STABLE="$(readme 'Stable tag')"
CONST_VER="$(grep -oiP "JABALI_CACHE_VERSION',\s*'\K[0-9]+\.[0-9]+\.[0-9]+" "${SRC_DIR}/jabali-cache.php" || true)"

[[ -n "$HDR_VER"   ]] || die "no Version in jabali-cache.php header"
[[ -n "$STABLE"    ]] || die "no Stable tag in readme.txt"
[[ -n "$CONST_VER" ]] || die "no JABALI_CACHE_VERSION in jabali-cache.php"
[[ "$HDR_VER" == "$STABLE"    ]] || die "version mismatch: header=$HDR_VER readme Stable tag=$STABLE"
[[ "$HDR_VER" == "$CONST_VER" ]] || die "version mismatch: header=$HDR_VER JABALI_CACHE_VERSION=$CONST_VER"

VER_RE="${HDR_VER//./\\.}"
grep -qE "^= ${VER_RE} =" "${SRC_DIR}/readme.txt" \
  || die "readme.txt changelog has no '= ${HDR_VER} =' section"
if [[ -f "${SRC_DIR}/CHANGELOG.md" ]]; then
  grep -qE "^##+ \[?${VER_RE}\]?" "${SRC_DIR}/CHANGELOG.md" \
    || die "CHANGELOG.md has no '## [${HDR_VER}]' heading"
fi

# GH #627: lib.php ships a JABALI_CACHE_VERSION fallback for direct-loaded
# drop-in/support code — it must match the header or a direct-load path reports
# a stale version.
if [[ -f "${SRC_DIR}/includes/lib.php" ]]; then
  LIB_VER="$(grep -oiP "JABALI_CACHE_VERSION',\s*'\K[0-9]+\.[0-9]+\.[0-9]+" "${SRC_DIR}/includes/lib.php" | head -1 || true)"
  if [[ -n "$LIB_VER" && "$LIB_VER" != "$HDR_VER" ]]; then
    die "version mismatch: includes/lib.php fallback JABALI_CACHE_VERSION=$LIB_VER header=$HDR_VER"
  fi
fi

# wp.org metadata cross-check (header vs readme) — a mismatch confuses wp.org's
# compatibility display. Only fields present in BOTH files are compared.
for field in "Requires at least" "Requires PHP"; do
  h="$(hdr "$field")" || true
  r="$(readme "$field")" || true
  if [[ -n "$h" && -n "$r" && "$h" != "$r" ]]; then
    die "metadata mismatch for '$field': header=$h readme=$r"
  fi
done

echo "check-version: OK — v${HDR_VER} consistent across header, JABALI_CACHE_VERSION, Stable tag, changelogs, and wp.org metadata"
