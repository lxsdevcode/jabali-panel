#!/usr/bin/env bash
# Drift-check the pinned external versions in install.sh against
# upstream. Hits GitHub Releases for the components hosted there;
# reports them as PINNED vs LATEST with a drift marker.
#
# Distro-tracked packages (nginx, MariaDB, certbot, etc.) are listed
# separately at the end -- we don't try to chase apt repos from here
# because the answer depends on the OS the operator just installed
# on, not on this script's host. Run `apt-cache policy <pkg>` on the
# target to see those.
#
# Usage:
#   ./scripts/check-external-versions.sh           # human table
#   ./scripts/check-external-versions.sh --json    # machine-readable
#
# Exit code is 0 when every pin matches latest, 1 when any drift is
# detected (CI gate). --json always exits 0 so a wrapper can decide.

set -euo pipefail

INSTALL_SH="$(dirname "$0")/../install.sh"
if [[ ! -f "$INSTALL_SH" ]]; then
  echo "install.sh not found at $INSTALL_SH" >&2
  exit 2
fi

# ---------- helpers ----------------------------------------------------------

# extract_pin <regex> -- pulls the first capture group from install.sh.
# Uses python re for clean capture-group extraction (sed escaping of
# install.sh paths is a footgun).
extract_pin() {
  python3 -c "
import re, sys
pat = sys.argv[1]
with open(sys.argv[2]) as f:
    for line in f:
        m = re.search(pat, line)
        if m:
            print(m.group(1))
            break
" "$1" "$INSTALL_SH"
}

# gh_latest <owner/repo> -- the latest non-prerelease tag from the
# Releases API. Strips a leading v. When the API rate-limits or fails
# we print "?" and let the caller mark unknown.
gh_latest() {
  local repo="$1"
  local tag
  tag="$(curl -fsSL \
    -H "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/${repo}/releases/latest" 2>/dev/null \
    | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('tag_name',''))" \
    2>/dev/null || true)"
  if [[ -z "$tag" ]]; then
    # Fall back to /tags if Releases is empty.
    tag="$(curl -fsSL \
      "https://api.github.com/repos/${repo}/tags" 2>/dev/null \
      | python3 -c "import sys,json; d=json.load(sys.stdin); print(d[0].get('name','') if d else '')" \
      2>/dev/null || true)"
  fi
  echo "${tag#v}"
}

# go_latest -- newest stable Go release.
go_latest() {
  curl -fsSL "https://go.dev/dl/?mode=json" \
    | python3 -c "import sys,json; d=json.load(sys.stdin); print(next(v['version'] for v in d if v.get('stable')).lstrip('go'))" \
    2>/dev/null || echo "?"
}

# normalise -- drop a leading v, trim whitespace.
n() { echo "${1#v}" | tr -d '[:space:]'; }

# ---------- pins from install.sh --------------------------------------------

GO_PIN="$(extract_pin 'JABALI_GO_VERSION:-([0-9]+\.[0-9]+\.[0-9]+)')"
ADMINER_PIN="$(extract_pin 'adminer/releases/download/v([0-9]+\.[0-9]+\.[0-9]+)')"
WPCLI_PIN="$(extract_pin 'wp_version="([0-9]+\.[0-9]+\.[0-9]+)"')"
PHPMYADMIN_PIN="$(extract_pin 'pma_version="([0-9]+\.[0-9]+\.[0-9]+)"')"
YARAX_PIN="$(extract_pin 'YARAX_VERSION="([0-9]+\.[0-9]+\.[0-9]+)"')"
LMD_PIN="$(extract_pin 'LMD_VERSION="([0-9.a-z-]+)"')"
STALWART_PIN="$(extract_pin 'stalwart_version="([0-9]+\.[0-9]+\.[0-9]+)"')"
STALWART_CLI_PIN="$(extract_pin 'cli_version="([0-9]+\.[0-9]+\.[0-9]+)"')"
BULWARK_PIN="$(extract_pin 'bulwark_version="([0-9]+\.[0-9]+\.[0-9]+)"')"
KRATOS_PIN="$(extract_pin 'kratos_version="([0-9]+\.[0-9]+\.[0-9]+)"')"
SNUF_PIN="$(extract_pin 'snuf_version="([0-9]+\.[0-9]+\.[0-9]+)"')"

# ---------- latest from upstream --------------------------------------------

declare -a ROWS
add_row() {
  local component="$1" pin="$2" latest="$3" repo="$4"
  local mark="OK"
  if [[ -z "$latest" || "$latest" == "?" ]]; then
    mark="?"
  elif [[ "$(n "$pin")" != "$(n "$latest")" ]]; then
    mark="DRIFT"
  fi
  ROWS+=("$component|$pin|$latest|$mark|$repo")
}

GO_LATEST="$(go_latest)"
add_row "Go toolchain"     "$GO_PIN"          "$GO_LATEST"                       "go.dev"
add_row "Adminer"          "$ADMINER_PIN"     "$(gh_latest vrana/adminer)"       "github.com/vrana/adminer"
add_row "wp-cli"           "$WPCLI_PIN"       "$(gh_latest wp-cli/wp-cli)"       "github.com/wp-cli/wp-cli"
phpmyadmin_latest() {
  local raw; raw="$(gh_latest phpmyadmin/phpmyadmin)"
  # Tags are RELEASE_X_Y_Z. Normalise to X.Y.Z for the diff.
  echo "${raw#RELEASE_}" | tr _ .
}
add_row "phpMyAdmin"       "$PHPMYADMIN_PIN"  "$(phpmyadmin_latest)" "github.com/phpmyadmin"
add_row "YARA-X"           "$YARAX_PIN"       "$(gh_latest VirusTotal/yara-x)"   "github.com/VirusTotal/yara-x"
add_row "Linux Malware Detect" "$LMD_PIN"     "?"                                "rfxn.com (no API)"
add_row "Stalwart server"  "$STALWART_PIN"    "$(gh_latest stalwartlabs/stalwart)" "github.com/stalwartlabs/stalwart"
add_row "Stalwart CLI"     "$STALWART_CLI_PIN" "$(gh_latest stalwartlabs/cli)"   "github.com/stalwartlabs/cli"
add_row "Bulwark webmail"  "$BULWARK_PIN"     "$(gh_latest bulwarkmail/webmail)" "github.com/bulwarkmail/webmail"
add_row "Ory Kratos"       "$KRATOS_PIN"      "$(gh_latest ory/kratos)"          "github.com/ory/kratos"
add_row "Snuffleupagus"    "$SNUF_PIN"        "$(gh_latest jvoisin/snuffleupagus)" "github.com/jvoisin/snuffleupagus"

# ---------- output -----------------------------------------------------------

if [[ "${1:-}" == "--json" ]]; then
  python3 -c "
import sys, json
rows = []
for line in sys.stdin:
    parts = line.rstrip('\\n').split('|')
    if len(parts) != 5: continue
    name, pin, latest, mark, repo = parts
    rows.append({'component': name, 'pinned': pin, 'latest': latest, 'status': mark, 'source': repo})
print(json.dumps(rows, indent=2))
" <<< "$(printf '%s\n' "${ROWS[@]}")"
  exit 0
fi

# Table.
drift_count=0
unknown_count=0
printf '%-26s %-14s %-14s %-8s %s\n' "COMPONENT" "PINNED" "LATEST" "STATUS" "SOURCE"
printf '%-26s %-14s %-14s %-8s %s\n' "---------" "------" "------" "------" "------"
for row in "${ROWS[@]}"; do
  IFS='|' read -r component pin latest mark repo <<< "$row"
  case "$mark" in
    OK)    color="\033[1;32m" ;;
    DRIFT) color="\033[1;33m"; drift_count=$((drift_count+1)) ;;
    ?)     color="\033[1;31m"; unknown_count=$((unknown_count+1)) ;;
    *)     color="" ;;
  esac
  printf "%-26s %-14s %-14s ${color}%-8s\033[0m %s\n" \
    "$component" "$pin" "$latest" "$mark" "$repo"
done

echo
if [[ $drift_count -gt 0 || $unknown_count -gt 0 ]]; then
  echo "Summary: $drift_count drift, $unknown_count unknown."
  echo "Distro-tracked packages (nginx, MariaDB, Redis, certbot, restic,"
  echo "ClamAV, AIDE, PowerDNS, Docker CE, PHP via Sury) are NOT checked"
  echo "here -- run 'apt-cache policy <pkg>' on the target host instead."
  [[ $drift_count -gt 0 ]] && exit 1
  exit 0
else
  echo "All checked pins match upstream latest."
  echo "Distro-tracked packages are not in scope; run 'apt-cache policy'"
  echo "on the target host."
  exit 0
fi
