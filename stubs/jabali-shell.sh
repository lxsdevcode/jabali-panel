#!/bin/bash
# jabali-shell — nspawn container login shell for Jabali Panel users
# Replaces the old chroot jail with proper container isolation.
#
# Three modes:
#   1. Interactive SSH login: ssh user@host → bash inside container
#   2. Command execution: ssh user@host "cmd" → run inside container
#   3. SFTP/SCP/rsync: handled transparently through container
set -euo pipefail

JUSER="$(whoami)"
CONTAINER="${JUSER}-php"

# Ensure container is running
if ! machinectl show "$CONTAINER" --property=State 2>/dev/null | grep -q "State=running"; then
    # Try to start it
    if command -v jabali-isolate &>/dev/null; then
        sudo /usr/local/bin/jabali-isolate start "$JUSER" 2>/dev/null || true
        sleep 1
    fi

    # Verify it started
    if ! machinectl show "$CONTAINER" --property=State 2>/dev/null | grep -q "State=running"; then
        echo "Error: Your container is not running. Contact your administrator." >&2
        exit 1
    fi
fi

# Get the leader PID of the container for nsenter
LEADER=$(machinectl show "$CONTAINER" --property=Leader --value 2>/dev/null)
if [[ -z "$LEADER" || "$LEADER" == "0" ]]; then
    echo "Error: Could not find container process. Contact your administrator." >&2
    exit 1
fi

UID_NUM="$(id -u)"
GID_NUM="$(id -g)"

# No command → interactive shell
if [[ -z "${SSH_ORIGINAL_COMMAND:-}" ]]; then
    exec sudo nsenter --target "$LEADER" --mount --pid --uts \
        setpriv --reuid="$UID_NUM" --regid="$GID_NUM" --init-groups \
        env HOME="/home/$JUSER" USER="$JUSER" SHELL=/bin/bash TERM="${TERM:-xterm-256color}" \
        /bin/bash --login
fi

# Command provided → execute inside container (covers SFTP, SCP, rsync, VS Code, etc.)
exec sudo nsenter --target "$LEADER" --mount --pid --uts \
    setpriv --reuid="$UID_NUM" --regid="$GID_NUM" --init-groups \
    env HOME="/home/$JUSER" USER="$JUSER" SHELL=/bin/bash \
    /bin/sh -c "$SSH_ORIGINAL_COMMAND"
