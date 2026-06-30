#!/usr/bin/env bash
# =============================================================================
# init.sh — TikTok Clone ClickHouse initialisation script
#
# Usage:
#   ./init.sh [OPTIONS]
#
# Options:
#   --host         ClickHouse host              (default: localhost)
#   --port         Native TCP port              (default: 9000)
#   --http-port    HTTP port (used for health)  (default: 8123)
#   --user         ClickHouse user              (default: default)
#   --password     ClickHouse password          (default: "")
#   --database     Target database name         (default: tiktok)
#   --skip-wait    Skip waiting for server      (default: false)
#   --drop-db      Drop and recreate database   (default: false)
#   --dry-run      Print SQL without executing  (default: false)
#
# Environment variables (override defaults):
#   CH_HOST, CH_PORT, CH_HTTP_PORT, CH_USER, CH_PASSWORD, CH_DATABASE
# =============================================================================

set -euo pipefail

# ---------------------------------------------------------------------------
# Defaults (overrideable by env or flags)
# ---------------------------------------------------------------------------
CH_HOST="${CH_HOST:-localhost}"
CH_PORT="${CH_PORT:-9000}"
CH_HTTP_PORT="${CH_HTTP_PORT:-8123}"
CH_USER="${CH_USER:-default}"
CH_PASSWORD="${CH_PASSWORD:-}"
CH_DATABASE="${CH_DATABASE:-tiktok}"
SKIP_WAIT=false
DROP_DB=false
DRY_RUN=false

# Script directory — schemas live in ../schemas relative to this file
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCHEMAS_DIR="${SCRIPT_DIR}/../schemas"

# Colour codes for pretty output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
RESET='\033[0m'

log_info()    { echo -e "${CYAN}[INFO]${RESET}  $*"; }
log_ok()      { echo -e "${GREEN}[OK]${RESET}    $*"; }
log_warn()    { echo -e "${YELLOW}[WARN]${RESET}  $*"; }
log_error()   { echo -e "${RED}[ERROR]${RESET} $*" >&2; }
log_step()    { echo -e "\n${YELLOW}==> $*${RESET}"; }

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------
while [[ $# -gt 0 ]]; do
    case "$1" in
        --host)        CH_HOST="$2";      shift 2 ;;
        --port)        CH_PORT="$2";      shift 2 ;;
        --http-port)   CH_HTTP_PORT="$2"; shift 2 ;;
        --user)        CH_USER="$2";      shift 2 ;;
        --password)    CH_PASSWORD="$2";  shift 2 ;;
        --database)    CH_DATABASE="$2";  shift 2 ;;
        --skip-wait)   SKIP_WAIT=true;    shift   ;;
        --drop-db)     DROP_DB=true;      shift   ;;
        --dry-run)     DRY_RUN=true;      shift   ;;
        -h|--help)
            grep '^#' "$0" | sed 's/^# \{0,2\}//'
            exit 0
            ;;
        *)
            log_error "Unknown flag: $1"
            exit 1
            ;;
    esac
done

# ---------------------------------------------------------------------------
# Dependency checks
# ---------------------------------------------------------------------------
log_step "Checking dependencies"

if ! command -v clickhouse-client &>/dev/null; then
    log_error "clickhouse-client not found in PATH."
    log_error "Install: https://clickhouse.com/docs/en/install"
    exit 1
fi

CH_CLIENT_VERSION=$(clickhouse-client --version 2>&1 | grep -oP '\d+\.\d+' | head -1)
log_ok "clickhouse-client found (version prefix: ${CH_CLIENT_VERSION})"

if ! command -v curl &>/dev/null; then
    log_warn "curl not found — skipping HTTP health check, using TCP only"
fi

# ---------------------------------------------------------------------------
# Build the clickhouse-client command prefix
# ---------------------------------------------------------------------------
CH_ARGS=(
    --host     "${CH_HOST}"
    --port     "${CH_PORT}"
    --user     "${CH_USER}"
    --multiline
    --format   TabSeparated
)
if [[ -n "${CH_PASSWORD}" ]]; then
    CH_ARGS+=(--password "${CH_PASSWORD}")
fi

ch_exec() {
    # ch_exec <sql>  — execute SQL, honour DRY_RUN
    local sql="$1"
    if [[ "${DRY_RUN}" == true ]]; then
        echo "--- DRY RUN ---"
        echo "${sql}"
        echo "---------------"
        return 0
    fi
    clickhouse-client "${CH_ARGS[@]}" --query "${sql}"
}

ch_exec_file() {
    # ch_exec_file <path>  — pipe a .sql file to clickhouse-client
    local file="$1"
    if [[ "${DRY_RUN}" == true ]]; then
        echo "--- DRY RUN: ${file} ---"
        cat "${file}"
        echo "--- END ---"
        return 0
    fi
    clickhouse-client "${CH_ARGS[@]}" < "${file}"
}

# ---------------------------------------------------------------------------
# Wait for ClickHouse to be ready
# ---------------------------------------------------------------------------
wait_for_clickhouse() {
    local max_attempts=30
    local attempt=0
    log_step "Waiting for ClickHouse at ${CH_HOST}:${CH_HTTP_PORT}"

    while (( attempt < max_attempts )); do
        (( attempt++ )) || true

        if command -v curl &>/dev/null; then
            if curl -sf "http://${CH_HOST}:${CH_HTTP_PORT}/ping" &>/dev/null; then
                log_ok "ClickHouse is ready (HTTP ping succeeded)"
                return 0
            fi
        else
            # Fallback: try a trivial SELECT via native port
            if clickhouse-client "${CH_ARGS[@]}" --query "SELECT 1" &>/dev/null; then
                log_ok "ClickHouse is ready (TCP query succeeded)"
                return 0
            fi
        fi

        log_info "Attempt ${attempt}/${max_attempts} — waiting 2 s..."
        sleep 2
    done

    log_error "ClickHouse did not become ready after ${max_attempts} attempts."
    exit 1
}

if [[ "${SKIP_WAIT}" == false ]]; then
    wait_for_clickhouse
fi

# ---------------------------------------------------------------------------
# Database setup
# ---------------------------------------------------------------------------
log_step "Setting up database '${CH_DATABASE}'"

if [[ "${DROP_DB}" == true ]]; then
    log_warn "Dropping existing database '${CH_DATABASE}' (--drop-db is set)"
    ch_exec "DROP DATABASE IF EXISTS ${CH_DATABASE}"
    log_ok "Database dropped"
fi

ch_exec "CREATE DATABASE IF NOT EXISTS ${CH_DATABASE}"
log_ok "Database '${CH_DATABASE}' exists"

# ---------------------------------------------------------------------------
# Schema installation order
# Ordering matters: watch_time.sql references video_views; engagement_events
# is standalone; live_metrics, revenue_metrics, ad_metrics are standalone.
# ---------------------------------------------------------------------------
SCHEMA_FILES=(
    "video_views.sql"
    "watch_time.sql"
    "engagement_events.sql"
    "live_metrics.sql"
    "revenue_metrics.sql"
    "ad_metrics.sql"
)

log_step "Installing schemas from ${SCHEMAS_DIR}"

INSTALLED=0
FAILED=0

for schema_file in "${SCHEMA_FILES[@]}"; do
    full_path="${SCHEMAS_DIR}/${schema_file}"

    if [[ ! -f "${full_path}" ]]; then
        log_error "Schema file not found: ${full_path}"
        (( FAILED++ )) || true
        continue
    fi

    log_info "Applying ${schema_file}..."

    if ch_exec_file "${full_path}"; then
        log_ok "${schema_file} applied"
        (( INSTALLED++ )) || true
    else
        log_error "${schema_file} FAILED"
        (( FAILED++ )) || true
        # Continue with remaining files; report at the end
    fi
done

# ---------------------------------------------------------------------------
# Verification — list tables created in the target database
# ---------------------------------------------------------------------------
log_step "Verifying schema objects in '${CH_DATABASE}'"

VERIFY_SQL="
SELECT
    name                                AS table_name,
    engine                              AS engine,
    formatReadableSize(total_bytes)     AS disk_size,
    total_rows                          AS rows
FROM system.tables
WHERE database = '${CH_DATABASE}'
ORDER BY name;
"

if [[ "${DRY_RUN}" == false ]]; then
    echo ""
    echo "Tables and views:"
    echo "------------------------------------------------------------"
    ch_exec "${VERIFY_SQL}" || log_warn "Could not query system.tables"
    echo "------------------------------------------------------------"

    MV_SQL="
    SELECT name, type
    FROM system.tables
    WHERE database = '${CH_DATABASE}'
      AND engine LIKE '%View%'
    ORDER BY name;
    "
    echo ""
    echo "Materialized / regular views:"
    ch_exec "${MV_SQL}" || true
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
log_step "Initialisation complete"
log_ok  "Schemas installed : ${INSTALLED}"
if (( FAILED > 0 )); then
    log_error "Schemas FAILED    : ${FAILED}"
    exit 1
fi

echo ""
echo "  Database  : ${CH_DATABASE}"
echo "  Host      : ${CH_HOST}:${CH_PORT}"
echo "  User      : ${CH_USER}"
echo ""
echo "Connect with:"
echo "  clickhouse-client --host ${CH_HOST} --port ${CH_PORT} \\"
echo "    --user ${CH_USER} --database ${CH_DATABASE}"
echo ""
