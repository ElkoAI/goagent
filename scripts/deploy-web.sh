#!/usr/bin/env bash
# Deploy files to a remote web host via rsync over SSH.
# Usage: ./scripts/deploy-web.sh [OPTIONS] <local-path> [remote-subpath]
#
# Options:
#   --dry-run    Show what would be transferred without doing it
#   --delete     Remove remote files not present locally
#
# Examples:
#   ./scripts/deploy-web.sh ~/.goagent/home/john/reports/daily/ reports/daily/
#   ./scripts/deploy-web.sh --dry-run ./public/ www/
#   ./scripts/deploy-web.sh index.html                          # single file to root
#
# Environment (set in .env or export):
#   DEPLOY_HOST  — remote hostname          (required)
#   DEPLOY_USER  — SSH username             (required)
#   DEPLOY_PATH  — remote base path         (default: /)
#   DEPLOY_PORT  — SSH port                 (default: 22)

set -euo pipefail
cd "$(dirname "$0")/.."

# Load environment.
set -a
[ -f .env ] && . .env
set +a

# --- Parse options ---
DRY_RUN=""
DELETE=""
while [[ "${1:-}" == --* ]]; do
    case "$1" in
        --dry-run) DRY_RUN="--dry-run"; shift ;;
        --delete)  DELETE="--delete"; shift ;;
        *) echo "Unknown option: $1" >&2; exit 1 ;;
    esac
done

LOCAL_PATH="${1:?Usage: deploy-web.sh [OPTIONS] <local-path> [remote-subpath]}"
REMOTE_SUB="${2:-}"

# --- Validate environment ---
: "${DEPLOY_HOST:?Set DEPLOY_HOST in .env (e.g. access-xxx.webspace-host.com)}"
: "${DEPLOY_USER:?Set DEPLOY_USER in .env (e.g. a1573256)}"
DEPLOY_PATH="${DEPLOY_PATH:-/}"
DEPLOY_PORT="${DEPLOY_PORT:-22}"

# Ensure remote path ends with /
DEPLOY_PATH="${DEPLOY_PATH%/}/"
REMOTE_DEST="${DEPLOY_PATH}${REMOTE_SUB}"

# --- Check tools ---
if ! command -v rsync &>/dev/null; then
    echo "rsync not found, falling back to scp" >&2
    if [ -d "$LOCAL_PATH" ]; then
        scp -P "$DEPLOY_PORT" -r "$LOCAL_PATH" "${DEPLOY_USER}@${DEPLOY_HOST}:${REMOTE_DEST}"
    else
        scp -P "$DEPLOY_PORT" "$LOCAL_PATH" "${DEPLOY_USER}@${DEPLOY_HOST}:${REMOTE_DEST}"
    fi
    exit $?
fi

# --- Build rsync command ---
RSYNC_OPTS=(
    -avz
    --chmod=D755,F644
    -e "ssh -p ${DEPLOY_PORT}"
    $DRY_RUN
    $DELETE
)

echo "Deploying: ${LOCAL_PATH} -> ${DEPLOY_USER}@${DEPLOY_HOST}:${REMOTE_DEST}"
rsync "${RSYNC_OPTS[@]}" "$LOCAL_PATH" "${DEPLOY_USER}@${DEPLOY_HOST}:${REMOTE_DEST}"
echo "Done."
