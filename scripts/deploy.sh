#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"


CONFIG_FILE="${CONFIG_FILE:-$ROOT_DIR/config.toml}"

# Capture env overrides before we assign defaults so config.toml can sit between
# defaults and environment: CLI > env > config > defaults.
ENV_DEPLOY_HOST="${DEPLOY_HOST-}"
ENV_DEPLOY_USER="${DEPLOY_USER-}"
ENV_DEPLOY_PATH="${DEPLOY_PATH-}"
ENV_WWW_USER="${WWW_USER-}"
ENV_NPM_CACHE_DIR="${NPM_CACHE_DIR-}"
ENV_GITEA_REMOTE="${GITEA_REMOTE-}"
ENV_GITEA_URL="${GITEA_URL-}"
ENV_GITHUB_REMOTE="${GITHUB_REMOTE-}"
ENV_GITHUB_URL="${GITHUB_URL-}"
ENV_PUSH_BRANCH="${PUSH_BRANCH-}"

DEPLOY_HOST=""
DEPLOY_USER="root"
DEPLOY_PATH="/var/www/jabali"
WWW_USER="www-data"
NPM_CACHE_DIR=""
GITEA_REMOTE="gitea"
GITEA_URL=""
GITHUB_REMOTE="origin"
GITHUB_URL=""
PUSH_BRANCH=""

trim_ws() {
    local s="${1:-}"
    s="$(echo "$s" | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')"
    printf '%s' "$s"
}

toml_unquote() {
    local v
    v="$(trim_ws "${1:-}")"

    # Only accept simple double-quoted strings, booleans, or integers.
    if [[ ${#v} -ge 2 && "${v:0:1}" == '"' && "${v: -1}" == '"' ]]; then
        printf '%s' "${v:1:${#v}-2}"
        return 0
    fi

    if [[ "$v" =~ ^(true|false)$ ]]; then
        printf '%s' "$v"
        return 0
    fi

    if [[ "$v" =~ ^-?[0-9]+$ ]]; then
        printf '%s' "$v"
        return 0
    fi

    return 1
}

load_config_toml() {
    local file section line key raw value
    file="$1"
    section=""

    [[ -f "$file" ]] || return 0

    while IFS= read -r line || [[ -n "$line" ]]; do
        # Strip comments and whitespace.
        line="${line%%#*}"
        line="$(trim_ws "$line")"
        [[ -z "$line" ]] && continue

        if [[ "$line" =~ ^\[([A-Za-z0-9_.-]+)\]$ ]]; then
            section="${BASH_REMATCH[1]}"
            continue
        fi

        [[ "$section" == "deploy" ]] || continue

        if [[ "$line" =~ ^([A-Za-z0-9_]+)[[:space:]]*=[[:space:]]*(.+)$ ]]; then
            key="${BASH_REMATCH[1]}"
            raw="${BASH_REMATCH[2]}"
            value=""

            if ! value="$(toml_unquote "$raw")"; then
                continue
            fi

            case "$key" in
                host) DEPLOY_HOST="$value" ;;
                user) DEPLOY_USER="$value" ;;
                path) DEPLOY_PATH="$value" ;;
                www_user) WWW_USER="$value" ;;
                npm_cache_dir) NPM_CACHE_DIR="$value" ;;
                gitea_remote) GITEA_REMOTE="$value" ;;
                gitea_url) GITEA_URL="$value" ;;
                github_remote) GITHUB_REMOTE="$value" ;;
                github_url) GITHUB_URL="$value" ;;
                push_branch) PUSH_BRANCH="$value" ;;
            esac
        fi
    done < "$file"
}

load_config_toml "$CONFIG_FILE"

# Apply environment overrides on top of config.
if [[ -n "${ENV_DEPLOY_HOST:-}" ]]; then DEPLOY_HOST="$ENV_DEPLOY_HOST"; fi
if [[ -n "${ENV_DEPLOY_USER:-}" ]]; then DEPLOY_USER="$ENV_DEPLOY_USER"; fi
if [[ -n "${ENV_DEPLOY_PATH:-}" ]]; then DEPLOY_PATH="$ENV_DEPLOY_PATH"; fi
if [[ -n "${ENV_WWW_USER:-}" ]]; then WWW_USER="$ENV_WWW_USER"; fi
if [[ -n "${ENV_NPM_CACHE_DIR:-}" ]]; then NPM_CACHE_DIR="$ENV_NPM_CACHE_DIR"; fi
if [[ -n "${ENV_GITEA_REMOTE:-}" ]]; then GITEA_REMOTE="$ENV_GITEA_REMOTE"; fi
if [[ -n "${ENV_GITEA_URL:-}" ]]; then GITEA_URL="$ENV_GITEA_URL"; fi
if [[ -n "${ENV_GITHUB_REMOTE:-}" ]]; then GITHUB_REMOTE="$ENV_GITHUB_REMOTE"; fi
if [[ -n "${ENV_GITHUB_URL:-}" ]]; then GITHUB_URL="$ENV_GITHUB_URL"; fi
if [[ -n "${ENV_PUSH_BRANCH:-}" ]]; then PUSH_BRANCH="$ENV_PUSH_BRANCH"; fi

SKIP_SYNC=0
SKIP_COMPOSER=0
SKIP_NPM=0
SKIP_MIGRATE=0
SKIP_CACHE=0
SKIP_AGENT_RESTART=0
DELETE_REMOTE=0
DRY_RUN=0
SKIP_PUSH=0
BUILD_DEB=0
DEB_UPLOAD=0
DEB_DISTS="stable bookworm trixie noble jammy"
PUSH_GITEA=1
PUSH_GITHUB=0
SET_VERSION=""
CONTAINER_BUILD=0
CONTAINER_PUSH=0
CONTAINER_DEPLOY=0
CONTAINER_HOST="rasp"

usage() {
    cat <<'EOF'
Usage: scripts/deploy.sh [options]

Options:
  --host HOST         Remote host (from config.toml or --host flag)
  --user USER         SSH user (default: root)
  --path PATH         Remote path (default: /var/www/jabali)
  --www-user USER     Remote runtime user (default: www-data)
  --skip-sync         Skip rsync sync step
  --skip-composer     Skip composer install
  --skip-npm          Skip npm install/build
  --skip-migrate      Skip php artisan migrate
  --skip-cache        Skip cache clear/rebuild
  --skip-agent-restart Skip restarting jabali-agent service
  --delete            Pass --delete to rsync (dangerous)
  --dry-run           Dry-run rsync only
  --skip-push         Skip all git push operations
  --push TARGET       Push to: gitea, github, or both (default: gitea)
  --push-gitea        Push current branch to Gitea from deploy server (default: on)
  --no-push-gitea     Disable Gitea push
  --gitea-remote NAME Gitea git remote name (default: gitea)
  --gitea-url URL     Push to this URL instead of a named remote
  --push-github       Push current branch to GitHub from deploy server (default: on)
  --no-push-github    Disable GitHub push
  --github-remote NAME GitHub git remote name (default: origin)
  --github-url URL    Push to this URL instead of a named remote
  --version VALUE     Set VERSION to a specific value before remote push
  --deb               Build .deb package after deploy
  --deb-upload        Build .deb and upload to deb.jabali-panel.com repo
  --deb-dists DISTS   Dists for reprepro (default: "stable bookworm trixie noble jammy")
  --container         Build container image locally with podman/docker
  --container-deploy  Build on rasp (arm64), deploy container, and push to Docker Hub
  --container-push    Build locally and push container image to Docker Hub
  --container-host H  Container build host (default: rasp)
  -h, --help          Show this help

Environment overrides:
  CONFIG_FILE points to a TOML file (default: ./config.toml). The script reads [deploy] keys.
  CONFIG_FILE, DEPLOY_HOST, DEPLOY_USER, DEPLOY_PATH, WWW_USER, NPM_CACHE_DIR, GITEA_REMOTE, GITEA_URL, GITHUB_REMOTE, GITHUB_URL, PUSH_BRANCH
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --host)
            DEPLOY_HOST="$2"
            shift 2
            ;;
        --user)
            DEPLOY_USER="$2"
            shift 2
            ;;
        --path)
            DEPLOY_PATH="$2"
            shift 2
            ;;
        --www-user)
            WWW_USER="$2"
            shift 2
            ;;
        --skip-sync)
            SKIP_SYNC=1
            shift
            ;;
        --skip-composer)
            SKIP_COMPOSER=1
            shift
            ;;
        --skip-npm)
            SKIP_NPM=1
            shift
            ;;
        --skip-migrate)
            SKIP_MIGRATE=1
            shift
            ;;
        --skip-cache)
            SKIP_CACHE=1
            shift
            ;;
        --skip-agent-restart)
            SKIP_AGENT_RESTART=1
            shift
            ;;
        --delete)
            DELETE_REMOTE=1
            shift
            ;;
        --dry-run)
            DRY_RUN=1
            shift
            ;;
        --skip-push)
            SKIP_PUSH=1
            PUSH_GITEA=0
            PUSH_GITHUB=0
            shift
            ;;
        --push)
            case "${2:-}" in
                gitea)
                    PUSH_GITEA=1
                    PUSH_GITHUB=0
                    ;;
                github)
                    PUSH_GITEA=0
                    PUSH_GITHUB=1
                    ;;
                both)
                    PUSH_GITEA=1
                    PUSH_GITHUB=1
                    ;;
                *)
                    echo "Invalid --push target: '${2:-}'. Use: gitea, github, or both"
                    exit 1
                    ;;
            esac
            shift 2
            ;;
        --push-gitea)
            PUSH_GITEA=1
            shift
            ;;
        --no-push-gitea)
            PUSH_GITEA=0
            shift
            ;;
        --gitea-remote)
            GITEA_REMOTE="$2"
            shift 2
            ;;
        --gitea-url)
            GITEA_URL="$2"
            shift 2
            ;;
        --push-github)
            PUSH_GITHUB=1
            shift
            ;;
        --no-push-github)
            PUSH_GITHUB=0
            shift
            ;;
        --github-remote)
            GITHUB_REMOTE="$2"
            shift 2
            ;;
        --github-url)
            GITHUB_URL="$2"
            shift 2
            ;;
        --version)
            SET_VERSION="$2"
            shift 2
            ;;
        --deb)
            BUILD_DEB=1
            shift
            ;;
        --deb-upload)
            BUILD_DEB=1
            DEB_UPLOAD=1
            shift
            ;;
        --deb-dists)
            DEB_DISTS="$2"
            shift 2
            ;;
        --container)
            CONTAINER_BUILD=1
            shift
            ;;
        --container-deploy)
            CONTAINER_BUILD=1
            CONTAINER_DEPLOY=1
            CONTAINER_PUSH=1
            shift
            ;;
        --container-push)
            CONTAINER_BUILD=1
            CONTAINER_PUSH=1
            shift
            ;;
        --container-host)
            CONTAINER_HOST="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

if [[ -z "$DEPLOY_HOST" ]] && [[ "$CONTAINER_BUILD" -eq 0 ]]; then
    echo "Error: No deploy host configured. Set it in config.toml [deploy] host or use --host flag."
    exit 1
fi

REMOTE=""
if [[ -n "$DEPLOY_HOST" ]]; then
    REMOTE="${DEPLOY_USER}@${DEPLOY_HOST}"
fi

ensure_remote_git_clean() {
    local status_output
    status_output="$(remote_run "if [[ ! -d \"$DEPLOY_PATH/.git\" ]]; then echo '__NO_GIT__'; exit 0; fi; cd \"$DEPLOY_PATH\" && git status --porcelain")"

    if [[ "$status_output" == "__NO_GIT__" ]]; then
        echo "Remote path is not a git repository: $DEPLOY_PATH"
        exit 1
    fi

    if [[ -n "$status_output" ]]; then
        echo "Remote git worktree is dirty at $DEPLOY_PATH. Commit or stash remote changes first."
        echo "$status_output"
        exit 1
    fi
}

remote_commit_and_push() {
    local local_head push_branch
    local_head="$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo unknown)"

    if [[ -n "$PUSH_BRANCH" ]]; then
        push_branch="$PUSH_BRANCH"
    else
        push_branch="$(remote_run "cd \"$DEPLOY_PATH\" && git rev-parse --abbrev-ref HEAD")"
        if [[ -z "$push_branch" || "$push_branch" == "HEAD" ]]; then
            push_branch="main"
        fi
    fi

    ssh -o StrictHostKeyChecking=accept-new "$REMOTE" \
        DEPLOY_PATH="$DEPLOY_PATH" \
        PUSH_BRANCH="$push_branch" \
        PUSH_GITEA="$PUSH_GITEA" \
        PUSH_GITHUB="$PUSH_GITHUB" \
        GITEA_REMOTE="$GITEA_REMOTE" \
        GITEA_URL="$GITEA_URL" \
        GITHUB_REMOTE="$GITHUB_REMOTE" \
        GITHUB_URL="$GITHUB_URL" \
        SET_VERSION="$SET_VERSION" \
        LOCAL_HEAD="$local_head" \
        bash -s <<'EOF'
set -euo pipefail

cd "$DEPLOY_PATH"

if [[ ! -d .git ]]; then
    echo "Remote path is not a git repository: $DEPLOY_PATH" >&2
    exit 1
fi

git config --global --add safe.directory "$DEPLOY_PATH" >/dev/null 2>&1 || true
if ! git config user.name >/dev/null; then
    git config user.name "Jabali Deploy"
fi
if ! git config user.email >/dev/null; then
    git config user.email "root@$(hostname -f 2>/dev/null || hostname)"
fi

current="$(sed -n 's/^VERSION=//p' VERSION || true)"
if [[ -z "$current" ]]; then
    echo "VERSION file missing or invalid on remote." >&2
    exit 1
fi

if [[ -n "${SET_VERSION:-}" ]]; then
    new="$SET_VERSION"
else
    if [[ "$current" =~ ^(.+-rc)([0-9]+)?$ ]]; then
        base="${BASH_REMATCH[1]}"
        num="${BASH_REMATCH[2]}"
        if [[ -z "$num" ]]; then
            num=1
        else
            num=$((num + 1))
        fi
        new="${base}${num}"
    elif [[ "$current" =~ ^(.+?)([0-9]+)$ ]]; then
        new="${BASH_REMATCH[1]}$((BASH_REMATCH[2] + 1))"
    else
        echo "Cannot auto-bump VERSION from '$current'. Use --version to set it explicitly." >&2
        exit 1
    fi
fi

if [[ "$new" == "$current" ]]; then
    echo "VERSION is already '$current'. Use --version to set a new value." >&2
    exit 1
fi

printf 'VERSION=%s\n' "$new" > VERSION
sed -i -E "s|JABALI_VERSION=\"\\$\\{JABALI_VERSION:-[^}]+\\}\"|JABALI_VERSION=\"\\\${JABALI_VERSION:-$new}\"|" install.sh

git add -A
if git diff --cached --quiet; then
    echo "No changes detected after sync/version bump; skipping commit."
else
    git commit -m "Deploy sync from ${LOCAL_HEAD} (v${new})"
fi

if [[ "$PUSH_GITEA" -eq 1 ]]; then
    if [[ -n "$GITEA_URL" ]]; then
        git push "$GITEA_URL" "$PUSH_BRANCH"
    else
        git push "$GITEA_REMOTE" "$PUSH_BRANCH"
    fi
fi

if [[ "$PUSH_GITHUB" -eq 1 ]]; then
    if [[ -n "$GITHUB_URL" ]]; then
        git push "$GITHUB_URL" "$PUSH_BRANCH"
    else
        git push "$GITHUB_REMOTE" "$PUSH_BRANCH"
    fi
fi
EOF
}

rsync_project() {
    local -a rsync_opts
    rsync_opts=(-az --info=progress2)
    if [[ "$DELETE_REMOTE" -eq 1 ]]; then
        rsync_opts+=(--delete)
    fi
    if [[ "$DRY_RUN" -eq 1 ]]; then
        rsync_opts+=(--dry-run)
    fi

    rsync "${rsync_opts[@]}" \
        --exclude ".git/" \
        --exclude "node_modules/" \
        --exclude "vendor/" \
        --exclude "storage/" \
        --exclude "bootstrap/cache/" \
        --exclude "public/build/" \
        --exclude ".env" \
        --exclude ".env.*" \
        --exclude "database/*.sqlite" \
        --exclude "database/*.sqlite-wal" \
        --exclude "database/*.sqlite-shm" \
        "$ROOT_DIR/" \
        "${REMOTE}:${DEPLOY_PATH}/"
}

remote_run() {
    ssh -o StrictHostKeyChecking=accept-new "$REMOTE" "bash -lc '$1'"
}

remote_run_www() {
    ssh -o StrictHostKeyChecking=accept-new "$REMOTE" "bash -lc 'cd \"$DEPLOY_PATH\" && sudo -u \"$WWW_USER\" -H bash -lc \"$1\"'"
}

ensure_remote_permissions() {
    local parent_dir
    parent_dir="$(dirname "$DEPLOY_PATH")"

    if [[ -z "$NPM_CACHE_DIR" ]]; then
        NPM_CACHE_DIR="${parent_dir}/.npm"
    fi

    remote_run "mkdir -p \"$DEPLOY_PATH/storage\" \"$DEPLOY_PATH/bootstrap/cache\" \"$DEPLOY_PATH/public/build\" \"$DEPLOY_PATH/node_modules\" \"$DEPLOY_PATH/database\" \"$NPM_CACHE_DIR\""
    remote_run "chown -R \"$WWW_USER\":\"$WWW_USER\" \"$DEPLOY_PATH\""
    remote_run "chown -R \"$WWW_USER\":\"$WWW_USER\" \"$NPM_CACHE_DIR\""
    remote_run "if [[ -f \"$DEPLOY_PATH/auth.json\" ]]; then chown \"$WWW_USER\":\"$WWW_USER\" \"$DEPLOY_PATH/auth.json\" && chmod 600 \"$DEPLOY_PATH/auth.json\"; fi"
}

# ── Remote deploy (skipped for container-only builds) ─────────────
if [[ -n "$DEPLOY_HOST" ]]; then

echo "Deploying to ${REMOTE}:${DEPLOY_PATH}"

if [[ "$DRY_RUN" -eq 0 && "$SKIP_PUSH" -eq 0 && ( "$PUSH_GITEA" -eq 1 || "$PUSH_GITHUB" -eq 1 ) ]]; then
    echo "Validating remote git worktree..."
    ensure_remote_git_clean
fi

if [[ "$DRY_RUN" -eq 1 ]]; then
    echo "Dry run complete. No remote commands executed."
    exit 0
fi

if [[ "$SKIP_SYNC" -eq 0 ]]; then
    # Push to gitea from local, then pull on remote — faster than rsync
    echo "Pushing to gitea..."
    if [[ -n "$GITEA_URL" ]]; then
        git -C "$ROOT_DIR" push "$GITEA_URL" HEAD:main 2>/dev/null || git -C "$ROOT_DIR" push "$GITEA_URL" main 2>/dev/null || true
    else
        git -C "$ROOT_DIR" push "$GITEA_REMOTE" main 2>/dev/null || true
    fi

    echo "Pulling latest on ${REMOTE}..."
    remote_run "cd \"$DEPLOY_PATH\" && git config --global --add safe.directory \"$DEPLOY_PATH\" 2>/dev/null; git fetch --all && git reset --hard origin/main 2>/dev/null || git reset --hard gitea/main 2>/dev/null || git pull --ff-only"
fi

echo "Ensuring remote permissions..."
ensure_remote_permissions

if [[ "$SKIP_COMPOSER" -eq 0 ]]; then
    echo "Installing composer dependencies..."
    remote_run_www "composer install --no-interaction --prefer-dist --optimize-autoloader --no-dev"
fi

if [[ "$SKIP_NPM" -eq 0 ]]; then
    echo "Building frontend assets..."
    remote_run_www "npm ci"
    remote_run_www "npm run build"
fi

echo "Entering maintenance mode..."
remote_run_www "php artisan down --retry=60 --refresh=15" || true

if [[ "$SKIP_MIGRATE" -eq 0 ]]; then
    echo "Running migrations..."
    remote_run_www "php artisan migrate --force"
fi

if [[ "$SKIP_CACHE" -eq 0 ]]; then
    echo "Refreshing caches..."
    remote_run_www "php artisan optimize:clear"
    remote_run_www "php artisan config:cache"
    remote_run_www "php artisan route:cache"
    remote_run_www "php artisan view:cache"
    remote_run_www "php artisan queue:restart"
fi

echo "Exiting maintenance mode..."
remote_run_www "php artisan up" || true

if [[ "$SKIP_AGENT_RESTART" -eq 0 ]]; then
    echo "Restarting services..."
    remote_run "if systemctl list-unit-files jabali-agent.service --no-legend 2>/dev/null | grep -q '^jabali-agent\\.service'; then systemctl restart jabali-agent; fi"
    remote_run "PHP_VER=\$(ls /run/php/php*-fpm-panel.sock 2>/dev/null | grep -oP 'php\K[0-9]+\.[0-9]+' | head -1); systemctl restart php\${PHP_VER:-8.4}-fpm-panel 2>/dev/null || systemctl restart php\${PHP_VER:-8.4}-fpm 2>/dev/null || true"
    remote_run "systemctl reload nginx 2>/dev/null || true"
fi


echo "Patching Filament notifications JS (Livewire compatibility)..."
remote_run "f=\"$DEPLOY_PATH/public/js/filament/notifications/notifications.js\"; if [[ -f \"\$f\" ]]; then perl -0777 -i -pe \"s/\\Qo=r();s(()=>{\\E/o=r();typeof s==\\x22function\\x22&&s(()=>{/g\" \"\$f\"; perl -0777 -i -pe \"s/\\Q}),n(({payload:a})=>{\\E/}),typeof n==\\x22function\\x22&&n(({payload:a})=>{/g\" \"\$f\"; perl -0777 -i -pe \"s/\\Qa?.snapshot?.data?.isFilamentNotificationsComponent&&e()\\E/a?.snapshot?.data?.isFilamentNotificationsComponent&&typeof e==\\x22function\\x22&&e()/g\" \"\$f\"; fi"

fi  # end remote deploy

# ── Deb package build ──
if [[ "$BUILD_DEB" -eq 1 ]]; then
    VERSION="$(cat "$ROOT_DIR/VERSION")"
    VERSION="${VERSION#VERSION=}"
    DEB_FILE="jabali-panel_${VERSION}_all.deb"

    echo ""
    echo "Building .deb package (v${VERSION})..."

    if [ ! -d "$ROOT_DIR/vendor" ]; then
        echo "  Installing composer dependencies for package..."
        composer install --no-interaction --prefer-dist --no-dev --quiet
    fi
    if [ ! -d "$ROOT_DIR/public/build" ]; then
        echo "  Building frontend assets for package..."
        npm ci --silent && npm run build
    fi

    bash "$ROOT_DIR/scripts/build-jabali-panel-deb.sh"

    if [[ "$DEB_UPLOAD" -eq 1 ]]; then
        DEB_REPO_HOST="hostsclick"
        DEB_REPO_PATH="/var/www/deb.jabali-panel.com"

        echo "Uploading ${DEB_FILE} to deb repo..."
        scp "$ROOT_DIR/${DEB_FILE}" "${DEB_REPO_HOST}:/tmp/${DEB_FILE}"

        for dist in $DEB_DISTS; do
            echo "  Adding to ${dist}..."
            ssh "$DEB_REPO_HOST" "cd ${DEB_REPO_PATH} && reprepro includedeb ${dist} /tmp/${DEB_FILE}"
        done

        ssh "$DEB_REPO_HOST" "rm -f /tmp/${DEB_FILE}"
        echo "Package published to deb.jabali-panel.com for: ${DEB_DISTS}"
    else
        echo "Package built: ${ROOT_DIR}/${DEB_FILE}"
        echo "To upload: scripts/deploy.sh --deb-upload"
    fi
fi

# ── Container Build ───────────────────────────────────────────────
if [[ "$CONTAINER_BUILD" -eq 1 ]]; then

    # Read version
    if [ -f "${ROOT_DIR}/VERSION" ]; then
        CONTAINER_VERSION=$(cat "${ROOT_DIR}/VERSION" | tr -d '[:space:]')
        CONTAINER_VERSION="${CONTAINER_VERSION#VERSION=}"
    else
        CONTAINER_VERSION="latest"
    fi

    CONTAINER_REGISTRY="docker.io/shukivaknin/jabali-panel"

    # Check for auth.json (required for Filament private repo)
    AUTH_JSON="${ROOT_DIR}/auth.json"
    if [ ! -f "$AUTH_JSON" ]; then
        echo "ERROR: auth.json not found at ${AUTH_JSON}"
        echo "Required for Filament composer packages."
        exit 1
    fi

    if [[ "$CONTAINER_DEPLOY" -eq 1 ]]; then
        # ── Remote build on container host (arm64) ────────────────
        CBUILD_DIR="/tmp/jabali-build"

        echo "==> Syncing project to ${CONTAINER_HOST}:${CBUILD_DIR}..."
        rsync -az --delete \
            --exclude='.git' \
            --exclude='node_modules' \
            --exclude='/vendor' \
            --exclude='public/build' \
            --exclude='storage/logs/*' \
            --exclude='storage/framework/cache/*' \
            --exclude='storage/framework/sessions/*' \
            --exclude='storage/framework/views/*' \
            --exclude='.env' \
            --exclude='.env.local' \
            --exclude='.env.production' \
            --exclude='tests' \
            --exclude='scripts/zap' \
            --exclude='.full-review' \
            --exclude='.ship-safe' \
            --exclude='*.deb' \
            "${ROOT_DIR}/" "${CONTAINER_HOST}:${CBUILD_DIR}/"

        scp "$AUTH_JSON" "${CONTAINER_HOST}:${CBUILD_DIR}/auth.json"

        echo "==> Building container on ${CONTAINER_HOST}..."
        ssh "$CONTAINER_HOST" "cd ${CBUILD_DIR} && DOCKER_BUILDKIT=1 sudo docker build \
            --secret id=composer_auth,src=auth.json \
            -t jabali-panel:${CONTAINER_VERSION} \
            -t jabali-panel:latest \
            -f Containerfile ."

        echo "==> Container image built on ${CONTAINER_HOST}: jabali-panel:${CONTAINER_VERSION}"

        # Stop old container and redeploy
        echo "==> Deploying container on ${CONTAINER_HOST}..."
        ssh "$CONTAINER_HOST" "sudo docker stop jabali 2>/dev/null || true"
        ssh "$CONTAINER_HOST" "sudo docker rm jabali 2>/dev/null || true"

        ssh "$CONTAINER_HOST" "sudo docker run -d --name jabali \
            --hostname jabali.local \
            --restart unless-stopped \
            -p 8880:80 -p 4443:443 \
            -p 2525:25 -p 5587:587 -p 9933:993 -p 1100:110 \
            -v jabali-mysql:/var/lib/mysql \
            -v jabali-storage:/var/www/jabali/storage \
            -v jabali-mail:/var/mail \
            -v jabali-home:/home \
            -v jabali-letsencrypt:/etc/letsencrypt \
            -e APP_URL=https://${CONTAINER_HOST}:4443 \
            -e SERVER_HOSTNAME=jabali.local \
            --cap-add NET_BIND_SERVICE \
            --cap-add NET_RAW \
            jabali-panel:latest"

        # Wait for startup and verify
        echo "==> Waiting for container to start..."
        sleep 30
        ssh "$CONTAINER_HOST" "sudo docker exec jabali supervisorctl status"

        # Push to Docker Hub
        echo "==> Tagging and pushing to Docker Hub..."
        ssh "$CONTAINER_HOST" "sudo docker tag jabali-panel:${CONTAINER_VERSION} ${CONTAINER_REGISTRY}:${CONTAINER_VERSION}"
        ssh "$CONTAINER_HOST" "sudo docker tag jabali-panel:latest ${CONTAINER_REGISTRY}:latest"
        ssh "$CONTAINER_HOST" "sudo docker push ${CONTAINER_REGISTRY}:${CONTAINER_VERSION}"
        ssh "$CONTAINER_HOST" "sudo docker push ${CONTAINER_REGISTRY}:latest"
        echo "==> Pushed ${CONTAINER_REGISTRY}:${CONTAINER_VERSION} and :latest"

        # Cleanup auth.json from remote
        ssh "$CONTAINER_HOST" "rm -f ${CBUILD_DIR}/auth.json"

    else
        # ── Local build ───────────────────────────────────────────
        echo "==> Building container image locally..."

        CONTAINER_CMD="podman"
        if ! command -v podman &>/dev/null; then
            CONTAINER_CMD="docker"
        fi

        $CONTAINER_CMD build \
            --secret id=composer_auth,src="$AUTH_JSON" \
            -t "jabali-panel:${CONTAINER_VERSION}" \
            -t "jabali-panel:latest" \
            -f "${ROOT_DIR}/Containerfile" \
            "${ROOT_DIR}"

        echo "==> Container image built: jabali-panel:${CONTAINER_VERSION}"

        if [[ "$CONTAINER_PUSH" -eq 1 ]]; then
            echo "==> Pushing to ${CONTAINER_REGISTRY}..."
            $CONTAINER_CMD tag "jabali-panel:${CONTAINER_VERSION}" "${CONTAINER_REGISTRY}:${CONTAINER_VERSION}"
            $CONTAINER_CMD tag "jabali-panel:latest" "${CONTAINER_REGISTRY}:latest"
            $CONTAINER_CMD push "${CONTAINER_REGISTRY}:${CONTAINER_VERSION}"
            $CONTAINER_CMD push "${CONTAINER_REGISTRY}:latest"
            echo "==> Pushed ${CONTAINER_REGISTRY}:${CONTAINER_VERSION} and :latest"
        fi
    fi
fi

echo "Deploy complete."
