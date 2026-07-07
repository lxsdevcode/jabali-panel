#!/usr/bin/env bash
#
# svn-deploy.sh — publish the Jabali Cache plugin to the WordPress.org SVN.
#
# WordPress.org hosts every plugin in a Subversion repo:
#   https://plugins.svn.wordpress.org/jabali-cache/
#     trunk/        the current development copy WP.org builds from
#     tags/<ver>/   an immutable snapshot per released version
#     assets/       listing images (banner/icon/screenshots) — NOT shipped to sites
#
# Releasing = copy the plugin files into trunk/, snapshot trunk into
# tags/<version>/, sync assets/, then `svn commit`. This script does all the
# local staging and, only with --publish, runs the commit. Password comes from
# the ## WordPress.org section of ~/cred.md when present, else svn prompts (your
# SVN password IS your WordPress.org account
# password). Dry-run by default.
#
# Prerequisites:
#   1. `svn` installed (apt install subversion).
#   2. A WordPress.org account that is a committer on the `jabali-cache` plugin
#      (the account that submitted it to the directory).
#   3. readme.txt "Stable tag" + the plugin-header "Version" already bumped to
#      the version you are releasing (this script verifies they match).
#
# Usage:
#   bin/svn-deploy.sh                      # dry-run: stage + show the diff, no commit
#   bin/svn-deploy.sh --publish -u <wporg-user>   # stage + svn commit (svn prompts for password)
#
set -euo pipefail

SLUG="jabali-cache"
SVN_URL="https://plugins.svn.wordpress.org/${SLUG}"

# Plugin source = the directory this script's parent lives in.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SRC_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

PUBLISH=0
WPORG_USER=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --publish) PUBLISH=1; shift ;;
    -u|--user) WPORG_USER="${2:-}"; shift 2 ;;
    -h|--help) sed -n '2,32p' "$0"; exit 0 ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

# WordPress.org SVN creds from ~/cred.md (## WordPress.org section) so --publish
# doesn't prompt. Format inside the section:
#   ## WordPress.org
#   username: <wporg-user>
#   password: <wporg-app-or-account-password>
# Absent section → falls back to interactive svn prompt (unchanged behavior).
CRED_FILE="${CRED_FILE:-$HOME/cred.md}"
WPORG_PASS=""
if [[ -f "$CRED_FILE" ]]; then
  wporg_block="$(awk '/^##[[:space:]]+WordPress\.org/{f=1;next} /^##[[:space:]]/{f=0} f' "$CRED_FILE")"
  if [[ -z "$WPORG_USER" ]]; then
    WPORG_USER="$(printf '%s\n' "$wporg_block" | grep -oiP '^\s*user(name)?:\s*\K\S.*' | head -1 | sed 's/[[:space:]]*$//')"
  fi
  WPORG_PASS="$(printf '%s\n' "$wporg_block" | grep -oiP '^\s*pass(word)?:\s*\K\S.*' | head -1 | sed 's/[[:space:]]*$//')"
fi

die() { echo "ERROR: $*" >&2; exit 1; }
command -v svn   >/dev/null || die "svn not installed (apt install subversion)"
command -v rsync >/dev/null || die "rsync not installed"

# --- 1. Resolve + cross-check the version ---------------------------------
HDR_VER="$(grep -oiP '^\s*\*\s*Version:\s*\K[0-9]+\.[0-9]+\.[0-9]+' "${SRC_DIR}/jabali-cache.php" || true)"
STABLE="$(grep -oiP '^Stable tag:\s*\K[0-9]+\.[0-9]+\.[0-9]+' "${SRC_DIR}/readme.txt" || true)"
[[ -n "$HDR_VER" ]] || die "could not read Version from jabali-cache.php"
[[ -n "$STABLE"  ]] || die "could not read Stable tag from readme.txt"
[[ "$HDR_VER" == "$STABLE" ]] || die "version mismatch: header=${HDR_VER} readme Stable tag=${STABLE} — bump both first"
VERSION="$HDR_VER"

# GH #627: cross-check EVERY version touch point, not just header + stable tag.
CONST_VER="$(grep -oiP "JABALI_CACHE_VERSION',\s*'\K[0-9]+\.[0-9]+\.[0-9]+" "${SRC_DIR}/jabali-cache.php" || true)"
[[ -n "$CONST_VER" ]] || die "could not read JABALI_CACHE_VERSION from jabali-cache.php"
[[ "$CONST_VER" == "$VERSION" ]] || die "version mismatch: JABALI_CACHE_VERSION=${CONST_VER} header=${VERSION} — bump the constant too"
VER_RE="${VERSION//./\\.}"
grep -qE "^##+ \[?${VER_RE}\]?" "${SRC_DIR}/CHANGELOG.md" 2>/dev/null \
    || die "CHANGELOG.md has no release heading for ${VERSION} (add '## [${VERSION}] — <date>')"
grep -qE "^= ${VER_RE} =" "${SRC_DIR}/readme.txt" 2>/dev/null \
    || die "readme.txt changelog has no '= ${VERSION} =' section"
echo "==> version consistent across header, JABALI_CACHE_VERSION, stable tag, CHANGELOG.md, readme.txt"

echo "==> Releasing ${SLUG} ${VERSION}"

# --- 2. Checkout the SVN repo (sparse: we only need trunk, tags, assets) ---
WORK="$(mktemp -d "${TMPDIR:-/tmp}/jc-svn-XXXXXX")"
trap 'rm -rf "$WORK"' EXIT
echo "==> Checking out ${SVN_URL} (sparse) into ${WORK}"
svn checkout --depth immediates "$SVN_URL" "$WORK" >/dev/null
svn update --set-depth infinity "$WORK/trunk"  >/dev/null
svn update --set-depth infinity "$WORK/assets" >/dev/null 2>&1 || mkdir -p "$WORK/assets"
svn update --set-depth immediates "$WORK/tags" >/dev/null

if [[ -d "$WORK/tags/${VERSION}" ]]; then
  die "tags/${VERSION} already exists on WordPress.org — bump the version, or remove the tag first"
fi

# --- 3. Sync plugin files into trunk/, honoring .distignore ---------------
# Build rsync excludes from .distignore (lines starting with / are repo-root
# anchored; blanks/comments skipped). Always drop VCS + the assets dir + bin.
RSYNC_EXCLUDES=( --exclude='.git' --exclude='.svn' --exclude='.wordpress-org' --exclude='bin' )
if [[ -f "${SRC_DIR}/.distignore" ]]; then
  while IFS= read -r line; do
    line="${line%%#*}"; line="${line#"${line%%[![:space:]]*}"}"; line="${line%"${line##*[![:space:]]}"}"
    [[ -z "$line" ]] && continue
    RSYNC_EXCLUDES+=( --exclude="${line#/}" )
  done < "${SRC_DIR}/.distignore"
fi
echo "==> Syncing plugin files into trunk/"
rsync -a --delete "${RSYNC_EXCLUDES[@]}" "${SRC_DIR}/" "$WORK/trunk/"

# --- 4. Sync listing assets (banner/icon) into assets/ --------------------
if [[ -d "${SRC_DIR}/.wordpress-org" ]]; then
  echo "==> Syncing listing assets into assets/"
  rsync -a --delete --exclude='.svn' "${SRC_DIR}/.wordpress-org/" "$WORK/assets/"
fi

# --- 5. Snapshot trunk -> tags/<version> ----------------------------------
echo "==> Creating tags/${VERSION} from trunk"
cp -a "$WORK/trunk" "$WORK/tags/${VERSION}"

# --- 6. Stage adds/removes for SVN ----------------------------------------
# `svn add` everything not yet versioned; `svn rm` anything gone from disk.
pushd "$WORK" >/dev/null
svn add --force . --auto-props --parents -q >/dev/null 2>&1 || true
# Delete files that vanished (status '!').
svn status | awk '/^!/{ $1=""; sub(/^ +/,""); print }' | while IFS= read -r missing; do
  [[ -n "$missing" ]] && svn rm --force "$missing" >/dev/null 2>&1 || true
done
echo
echo "==> Pending SVN changes:"
svn status
popd >/dev/null

# --- 7. Commit (only with --publish) --------------------------------------
MSG="Release ${VERSION}"
if [[ "$PUBLISH" -ne 1 ]]; then
  echo
  echo "DRY-RUN complete. Nothing committed."
  echo "To publish: bin/svn-deploy.sh --publish -u <your-wordpress.org-username>"
  echo "(svn will prompt for your password — it is your WordPress.org account password.)"
  exit 0
fi

[[ -n "$WPORG_USER" ]] || die "--publish needs a WordPress.org username: pass -u <user> or add a ## WordPress.org section (username:/password:) to ${CRED_FILE}"
echo
svn_auth=(--username "$WPORG_USER")
if [[ -n "$WPORG_PASS" ]]; then
  # --password is briefly visible in `ps`; acceptable on a personal deploy box.
  # --no-auth-cache keeps it out of ~/.subversion (cred.md stays the one store).
  svn_auth+=(--password "$WPORG_PASS" --non-interactive --no-auth-cache)
  echo "==> Committing to WordPress.org as '${WPORG_USER}' (credentials from ${CRED_FILE})"
else
  echo "==> Committing to WordPress.org as '${WPORG_USER}' (you will be prompted for your password)"
fi
svn commit "$WORK" -m "$MSG" "${svn_auth[@]}"
echo "==> Done. https://wordpress.org/plugins/${SLUG}/ will show ${VERSION} shortly."
