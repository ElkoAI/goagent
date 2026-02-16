#!/usr/bin/env bash
# Generate daily market report.
# Usage: ./scripts/daily-report.sh [--user USER] [--date YYYY-MM-DD] [--no-llm]
#
# Cron example (5:10pm CT weekdays):
#   10 17 * * 1-5 cd /path/to/goagent && ./scripts/daily-report.sh --user john

set -euo pipefail
cd "$(dirname "$0")/.."

# Load environment.
set -a
[ -f .env ] && . .env
set +a

# Build if binary is missing or stale.
BIN=./bin/goagent-report
if [ ! -f "$BIN" ] || [ "$(find cmd/report internal/reporting -newer "$BIN" 2>/dev/null | head -1)" ]; then
    echo "Building goagent-report..."
    GOTOOLCHAIN=local go build -o "$BIN" ./cmd/report
fi

exec "$BIN" daily "$@"
