#!/bin/bash
# jabali-shell — SSH login shell for Jabali Panel users
#
# Two-tier isolation:
#   1. nspawn (preferred) — full container via jabali-isolator
#   2. bwrap (fallback)  — bubblewrap sandbox (works inside LXC)
#   If neither is available, access is denied.
#
# Three modes per tier:
#   1. Interactive SSH login: ssh user@host → bash inside sandbox
#   2. Command execution: ssh user@host "cmd" → run inside sandbox
#   3. SFTP/SCP/rsync: handled transparently
set -euo pipefail

JUSER="$(whoami)"
CONTAINER="${JUSER}-php"

# --- Tier 1: nspawn container ---
try_nspawn() {
    # Check if nspawn is available and containers can run
    command -v machinectl &>/dev/null || return 1

    # Ensure container is running
    if ! machinectl list --no-legend 2>/dev/null | grep -q "^${CONTAINER} "; then
        if command -v jabali-isolate &>/dev/null; then
            if [[ ! -d "/var/lib/machines/${CONTAINER}" ]]; then
                sudo /usr/local/bin/jabali-isolate create "$JUSER" >/dev/null 2>&1 || return 1
            fi
            sudo /usr/local/bin/jabali-isolate start "$JUSER" >/dev/null 2>&1 || return 1
            sleep 1
        else
            return 1
        fi

        # Verify it started
        if ! machinectl list --no-legend 2>/dev/null | grep -q "^${CONTAINER} "; then
            return 1
        fi
    fi

    # Get the leader PID
    local leader
    leader=$(machinectl show "$CONTAINER" --property=Leader --value 2>/dev/null)
    if [[ -z "$leader" || "$leader" == "0" ]]; then
        return 1
    fi

    local uid_num gid_num
    uid_num="$(id -u)"
    gid_num="$(id -g)"

    # No command → interactive shell
    if [[ -z "${SSH_ORIGINAL_COMMAND:-}" ]]; then
        exec sudo nsenter --target "$leader" --mount --pid --ipc --uts --no-fork \
            setpriv --reuid="$uid_num" --regid="$gid_num" --init-groups \
            env HOME="/home/$JUSER" USER="$JUSER" SHELL=/bin/bash TERM="${TERM:-xterm-256color}" \
            /bin/bash -il
    fi

    # Command provided → execute inside container
    exec sudo nsenter --target "$leader" --mount --pid --ipc --uts \
        setpriv --reuid="$uid_num" --regid="$gid_num" --init-groups \
        env HOME="/home/$JUSER" USER="$JUSER" SHELL=/bin/bash \
        /bin/sh -c "$SSH_ORIGINAL_COMMAND"
}

# --- Tier 2: bubblewrap sandbox ---
try_bwrap() {
    command -v bwrap &>/dev/null || return 1

    # Delegate to the bwrap wrapper script
    local bwrap_shell="/usr/local/bin/jabali-shell-bwrap"
    if [[ -x "$bwrap_shell" ]]; then
        exec "$bwrap_shell"
    fi

    return 1
}

# --- Main: try nspawn, then bwrap, then fail ---
if try_nspawn; then
    exit 0
fi

if try_bwrap; then
    exit 0
fi

echo "Error: No isolation backend available (nspawn and bwrap both failed). Contact your administrator." >&2
exit 1
