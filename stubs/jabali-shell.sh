#!/bin/bash
# jabali-shell — nspawn container login shell for Jabali Panel users
# Replaces the old chroot jail with proper container isolation.
#
# Three modes:
#   1. Interactive SSH login: ssh user@host → bash inside container
#   2. Command execution: ssh user@host "cmd" → run inside container
#   3. SFTP/SCP/rsync: handled transparently through container
set -euo pipefail

USER="$(whoami)"
CONTAINER="${USER}-jail"

# Ensure container is running
if ! machinectl show "$CONTAINER" --property=State 2>/dev/null | grep -q "State=running"; then
    # Try to start it
    if command -v jabali-isolate &>/dev/null; then
        sudo /usr/local/bin/jabali-isolate start "$USER" 2>/dev/null || true
        sleep 1
    fi

    # Verify it started
    if ! machinectl show "$CONTAINER" --property=State 2>/dev/null | grep -q "State=running"; then
        echo "Error: Your container is not running. Contact your administrator." >&2
        exit 1
    fi
fi

# No command → interactive shell with proper PTY
if [[ -z "${SSH_ORIGINAL_COMMAND:-}" ]]; then
    exec sudo systemd-run --quiet --wait --pty \
        --machine="$CONTAINER" \
        --uid="$(id -u)" --gid="$(id -g)" \
        --setenv=HOME="/home/$USER" \
        --setenv=USER="$USER" \
        --setenv=SHELL=/bin/bash \
        --setenv=TERM="${TERM:-xterm-256color}" \
        /bin/bash --login
fi

# Command provided → execute inside container (covers SFTP, SCP, rsync, VS Code, etc.)
exec sudo systemd-run --quiet --wait --pipe \
    --machine="$CONTAINER" \
    --uid="$(id -u)" --gid="$(id -g)" \
    --setenv=HOME="/home/$USER" \
    --setenv=USER="$USER" \
    --setenv=SHELL=/bin/bash \
    /bin/sh -c "$SSH_ORIGINAL_COMMAND"
