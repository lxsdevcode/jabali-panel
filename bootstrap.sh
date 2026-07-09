#!/usr/bin/env bash
# Jabali Panel TUI bootstrap (M353 / GH #353).
#
#   curl -fsSL https://get.jabali-panel.com | sudo bash
#
# Downloads the latest sha256-verified release tarball, extracts the prebuilt
# jabali-installer binary + install.sh, and launches the Bubble Tea installer.
# The installer collects a deploy profile + modules + config, then runs
# install.sh (JABALI_MODULES=…) with a live progress pane.
#
# Non-interactive parity: pipe with no TTY, pass --unattended, or preset
# JABALI_MODULES and the installer skips the TUI and runs install.sh directly.
# Any args after `bash -s --` are forwarded to the installer (e.g. --dry-run).
#
# Overrides: JABALI_RELEASE_API_BASE (default codeberg), JABALI_REF (unused here;
# the tarball is always the latest published release).
set -euo pipefail

API_BASE="${JABALI_RELEASE_API_BASE:-https://codeberg.org/api/v1/repos/shukivaknin/jabali2}"

die() { printf '\033[1;31m[bootstrap] %s\033[0m\n' "$*" >&2; exit 1; }
log() { printf '\033[1;34m[bootstrap]\033[0m %s\n' "$*"; }

[[ $EUID -eq 0 ]] || die "must run as root (curl … | sudo bash)"
for bin in curl tar sha256sum; do
  command -v "$bin" >/dev/null 2>&1 || die "missing required tool: $bin"
done

log "resolving latest release with a published tarball from ${API_BASE}"
# Scan the releases list (newest first) rather than /releases/latest: the very
# newest tag may be published seconds before its build finishes, so it can be
# asset-less. grep/sed only — no jq dependency on a fresh box. The first
# jabali-release-*.tar.gz URL in the list belongs to the newest release that has
# one; the .sha256 sidecar lives at the same download path.
rel="$(curl -fsSL "${API_BASE}/releases?limit=30")" || die "could not reach the release API"
tar_url="$(printf '%s' "$rel" \
  | grep -oE '"browser_download_url": *"[^"]+"' \
  | sed 's/.*"\(https[^"]*\)"/\1/' \
  | grep -E 'jabali-release-[0-9a-f]+\.tar\.gz$' | head -1 || true)"
[[ -n "$tar_url" ]] || die "no published release tarball found (the release build may not have attached assets yet)"
sum_url="$(printf '%s' "$rel" \
  | grep -oE '"browser_download_url": *"[^"]+"' \
  | sed 's/.*"\(https[^"]*\)"/\1/' \
  | grep -E 'jabali-release-[0-9a-f]+\.tar\.gz\.sha256$' | head -1 || true)"
[[ -n "$sum_url" ]] || die "release has no .sha256 checksum asset — refusing to run an unverified tarball"

tmp="$(mktemp -d /tmp/jabali-bootstrap.XXXXXX)"
trap 'rm -rf "$tmp"' EXIT

log "downloading release tarball"
curl -fsSL "$tar_url" -o "$tmp/release.tar.gz" || die "tarball download failed"
curl -fsSL "$sum_url" -o "$tmp/release.sha256" || die "checksum download failed"

# Verify: the .sha256 asset is "<hex>  <filename>"; compare the hex only so a
# differing filename in the asset doesn't matter.
expected="$(awk '{print $1}' "$tmp/release.sha256")"
actual="$(sha256sum "$tmp/release.tar.gz" | awk '{print $1}')"
[[ -n "$expected" && "$expected" == "$actual" ]] \
  || die "checksum mismatch — refusing to run (expected ${expected:-none}, got ${actual})"
log "checksum verified"

# Extract only what the bootstrap needs.
tar -C "$tmp" -xzf "$tmp/release.tar.gz" bin/jabali-installer install.sh \
  || die "release tarball is missing bin/jabali-installer or install.sh (rebuild the release)"
chmod +x "$tmp/bin/jabali-installer"

log "launching installer"
exec env JABALI_INSTALL_SH="$tmp/install.sh" "$tmp/bin/jabali-installer" "$@"
