#!/bin/bash
# jabali-container-idle-check — stop nspawn containers with no active SSH sessions
# Called by systemd timer every 5 minutes.
set -euo pipefail

MACHINES_DIR="/var/lib/machines"

for container_dir in "$MACHINES_DIR"/*-php; do
    [[ -d "$container_dir" ]] || continue

    container=$(basename "$container_dir")
    user="${container%-php}"

    # Check if container is running
    if ! machinectl show "$container" --property=State --value 2>/dev/null | grep -q "^running$"; then
        continue
    fi

    # Get container leader PID
    leader=$(machinectl show "$container" --property=Leader --value 2>/dev/null)
    [[ -z "$leader" || "$leader" == "0" ]] && continue

    # Check for active nsenter processes targeting this container's PID namespace
    if pgrep -f "nsenter.*--target $leader" >/dev/null 2>&1; then
        continue
    fi

    # No active sessions — stop the container
    systemctl stop "systemd-nspawn@${container}.service" 2>/dev/null || true
    logger -t jabali-isolator "Stopped idle container $container (no active sessions)"
done
