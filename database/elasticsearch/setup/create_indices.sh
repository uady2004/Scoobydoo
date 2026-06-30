#!/usr/bin/env bash
# =============================================================================
# create_indices.sh
# Creates all Elasticsearch indices for the TikTok clone.
#
# Usage:
#   ./create_indices.sh [OPTIONS]
#
# Options:
#   -h HOST         Elasticsearch host (default: http://localhost:9200)
#   -u USER         Basic-auth username
#   -p PASS         Basic-auth password
#   -k API_KEY      API key (Authorization: ApiKey <key>)
#   -e ENV          Environment prefix: dev | staging | prod (default: dev)
#   -d              Dry-run: print the curl commands without executing them
#   --delete        Delete existing indices before recreating (destructive!)
#   --help          Show this message and exit
#
# Example:
#   ./create_indices.sh -h https://es.example.com -u elastic -p secret -e prod
# =============================================================================

set -euo pipefail

# ---------------------------------------------------------------------------
# Defaults
# ---------------------------------------------------------------------------
ES_HOST="http://localhost:9200"
ES_USER=""
ES_PASS=""
ES_API_KEY=""
ENV_PREFIX="dev"
DRY_RUN=false
DELETE_FIRST=false

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INDICES_DIR="$(cd "${SCRIPT_DIR}/../indices" && pwd)"

# ---------------------------------------------------------------------------
# Colour helpers
# ---------------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()    { echo -e "${CYAN}[INFO]${NC}  $*"; }
success() { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error()   { echo -e "${RED}[ERROR]${NC} $*" >&2; }
die()     { error "$*"; exit 1; }

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------
usage() {
  grep '^#' "$0" | sed 's/^# \?//'
  exit 0
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h) ES_HOST="$2"; shift 2 ;;
    -u) ES_USER="$2"; shift 2 ;;
    -p) ES_PASS="$2"; shift 2 ;;
    -k) ES_API_KEY="$2"; shift 2 ;;
    -e) ENV_PREFIX="$2"; shift 2 ;;
    -d) DRY_RUN=true; shift ;;
    --delete) DELETE_FIRST=true; shift ;;
    --help) usage ;;
    *) die "Unknown option: $1. Run with --help for usage." ;;
  esac
done

# ---------------------------------------------------------------------------
# Auth helpers
# ---------------------------------------------------------------------------
auth_flags() {
  local flags=()
  if [[ -n "${ES_API_KEY}" ]]; then
    flags+=(-H "Authorization: ApiKey ${ES_API_KEY}")
  elif [[ -n "${ES_USER}" && -n "${ES_PASS}" ]]; then
    flags+=(-u "${ES_USER}:${ES_PASS}")
  fi
  echo "${flags[@]+"${flags[@]}"}"
}

# ---------------------------------------------------------------------------
# curl wrapper — respects DRY_RUN
# ---------------------------------------------------------------------------
es_curl() {
  local method="$1"
  local url="$2"
  local data="${3:-}"
  local auth
  auth=$(auth_flags)

  local cmd=(
    curl --silent --show-error --fail-with-body
    -X "${method}"
    -H "Content-Type: application/json"
  )

  # shellcheck disable=SC2206
  [[ -n "${auth}" ]] && cmd+=(${auth})

  if [[ -n "${data}" ]]; then
    cmd+=(-d "${data}")
  fi

  cmd+=("${url}")

  if $DRY_RUN; then
    echo "[DRY-RUN] ${cmd[*]}"
    return 0
  fi

  local response
  response=$("${cmd[@]}") || {
    error "Request failed: ${method} ${url}"
    echo "${response}" | python3 -m json.tool 2>/dev/null || echo "${response}"
    return 1
  }

  echo "${response}"
}

# ---------------------------------------------------------------------------
# Wait for Elasticsearch to be available
# ---------------------------------------------------------------------------
wait_for_es() {
  local max_attempts=30
  local attempt=0

  info "Waiting for Elasticsearch at ${ES_HOST} ..."

  while (( attempt < max_attempts )); do
    if es_curl GET "${ES_HOST}/_cluster/health?timeout=5s" >/dev/null 2>&1; then
      success "Elasticsearch is reachable."
      return 0
    fi
    (( attempt++ ))
    warn "Attempt ${attempt}/${max_attempts} — retrying in 5s ..."
    sleep 5
  done

  die "Elasticsearch did not become available after ${max_attempts} attempts."
}

# ---------------------------------------------------------------------------
# Check cluster health
# ---------------------------------------------------------------------------
check_cluster_health() {
  info "Checking cluster health ..."
  local health
  health=$(es_curl GET "${ES_HOST}/_cluster/health")
  local status
  status=$(echo "${health}" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])" 2>/dev/null || echo "unknown")

  case "${status}" in
    green)  success "Cluster status: ${status}" ;;
    yellow) warn "Cluster status: ${status} (some replicas may be unassigned)" ;;
    red)    die "Cluster status: RED — aborting." ;;
    *)      warn "Could not determine cluster status." ;;
  esac
}

# ---------------------------------------------------------------------------
# Create or update an index alias (pointing to the versioned index name)
# ---------------------------------------------------------------------------
ensure_alias() {
  local alias="$1"
  local index="$2"

  local exists
  exists=$(es_curl GET "${ES_HOST}/_alias/${alias}" 2>/dev/null || echo "{}")

  if echo "${exists}" | grep -q "\"${index}\""; then
    info "Alias '${alias}' -> '${index}' already exists."
    return 0
  fi

  info "Creating alias '${alias}' -> '${index}' ..."
  es_curl POST "${ES_HOST}/_aliases" "$(cat <<JSON
{
  "actions": [
    { "add": { "index": "${index}", "alias": "${alias}" } }
  ]
}
JSON
)" >/dev/null
  success "Alias created."
}

# ---------------------------------------------------------------------------
# Create a single index
# ---------------------------------------------------------------------------
create_index() {
  local name="$1"          # logical name, e.g. "users"
  local mapping_file="$2"  # full path to the JSON mapping file

  local full_index="${ENV_PREFIX}_${name}"
  local alias="${name}"    # the alias == logical name (no env prefix)

  info "------------------------------------------------------------"
  info "Processing index: ${full_index}  (alias: ${alias})"

  # Verify mapping file exists
  [[ -f "${mapping_file}" ]] || die "Mapping file not found: ${mapping_file}"

  # ----- Delete if requested -----
  if $DELETE_FIRST; then
    local exists_code
    es_curl GET "${ES_HOST}/${full_index}" >/dev/null 2>&1 && exists_code=0 || exists_code=1

    if [[ "${exists_code}" -eq 0 ]]; then
      warn "Deleting existing index '${full_index}' ..."
      es_curl DELETE "${ES_HOST}/${full_index}" >/dev/null
      success "Deleted '${full_index}'."
    fi
  fi

  # ----- Create index -----
  local mapping_json
  mapping_json=$(cat "${mapping_file}")

  info "Creating index '${full_index}' ..."
  local response
  if response=$(es_curl PUT "${ES_HOST}/${full_index}" "${mapping_json}" 2>&1); then
    success "Index '${full_index}' created."
  else
    # If index already exists and we're not deleting, that's fine
    if echo "${response}" | grep -q "resource_already_exists_exception"; then
      warn "Index '${full_index}' already exists — skipping creation."
    else
      error "Failed to create index '${full_index}':"
      echo "${response}"
      return 1
    fi
  fi

  # ----- Alias -----
  ensure_alias "${alias}" "${full_index}"
}

# ---------------------------------------------------------------------------
# Apply index lifecycle management policy
# ---------------------------------------------------------------------------
apply_ilm_policy() {
  local policy_name="${ENV_PREFIX}_tiktok_policy"
  info "Applying ILM policy: ${policy_name} ..."

  es_curl PUT "${ES_HOST}/_ilm/policy/${policy_name}" "$(cat <<'JSON'
{
  "policy": {
    "phases": {
      "hot": {
        "min_age": "0ms",
        "actions": {
          "rollover": {
            "max_primary_shard_size": "20gb",
            "max_age": "30d"
          },
          "set_priority": { "priority": 100 }
        }
      },
      "warm": {
        "min_age": "30d",
        "actions": {
          "shrink": { "number_of_shards": 1 },
          "forcemerge": { "max_num_segments": 1 },
          "set_priority": { "priority": 50 }
        }
      },
      "cold": {
        "min_age": "90d",
        "actions": {
          "freeze": {},
          "set_priority": { "priority": 0 }
        }
      },
      "delete": {
        "min_age": "365d",
        "actions": { "delete": {} }
      }
    }
  }
}
JSON
)" >/dev/null

  success "ILM policy '${policy_name}' applied."
}

# ---------------------------------------------------------------------------
# Apply cluster-wide index templates for common settings
# ---------------------------------------------------------------------------
apply_index_template() {
  local template_name="${ENV_PREFIX}_tiktok_defaults"
  info "Applying index template: ${template_name} ..."

  es_curl PUT "${ES_HOST}/_index_template/${template_name}" "$(cat <<JSON
{
  "index_patterns": ["${ENV_PREFIX}_*"],
  "priority": 100,
  "template": {
    "settings": {
      "index.codec":                     "best_compression",
      "index.mapping.total_fields.limit": 500,
      "index.max_inner_result_window":    100,
      "index.search.slowlog.threshold.query.warn":  "10s",
      "index.search.slowlog.threshold.query.info":  "5s",
      "index.search.slowlog.threshold.query.debug": "2s",
      "index.indexing.slowlog.threshold.index.warn": "10s"
    }
  }
}
JSON
)" >/dev/null

  success "Index template '${template_name}' applied."
}

# ---------------------------------------------------------------------------
# Print index stats after creation
# ---------------------------------------------------------------------------
print_stats() {
  info "------------------------------------------------------------"
  info "Index summary:"
  local stats
  stats=$(es_curl GET "${ES_HOST}/${ENV_PREFIX}_*/_stats/docs,store" 2>/dev/null || echo "{}")
  echo "${stats}" | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    indices = d.get('indices', {})
    fmt = '{:<35} {:>10} {:>12}'
    print(fmt.format('Index', 'Doc count', 'Store size'))
    print('-' * 60)
    for name, info in sorted(indices.items()):
        docs  = info['primaries']['docs']['count']
        store = info['primaries']['store']['size_in_bytes']
        mb    = store / 1024 / 1024
        print(fmt.format(name, docs, f'{mb:.2f} MB'))
except Exception as e:
    print(f'Could not parse stats: {e}')
" 2>/dev/null || warn "Could not retrieve stats."
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  echo ""
  echo "============================================================"
  echo "  TikTok Clone — Elasticsearch Index Setup"
  echo "  Host:    ${ES_HOST}"
  echo "  Env:     ${ENV_PREFIX}"
  echo "  Dry-run: ${DRY_RUN}"
  echo "  Delete:  ${DELETE_FIRST}"
  echo "============================================================"
  echo ""

  $DRY_RUN || wait_for_es
  $DRY_RUN || check_cluster_health

  # Apply global template and ILM before creating indices
  $DRY_RUN || apply_ilm_policy
  $DRY_RUN || apply_index_template

  # ----- Index definitions -----
  # Format: create_index <logical_name> <path_to_mapping_json>
  create_index "users"    "${INDICES_DIR}/users.json"
  create_index "videos"   "${INDICES_DIR}/videos.json"
  create_index "hashtags" "${INDICES_DIR}/hashtags.json"
  create_index "products" "${INDICES_DIR}/products.json"
  create_index "sounds"   "${INDICES_DIR}/sounds.json"

  $DRY_RUN || print_stats

  echo ""
  success "All indices created successfully for environment '${ENV_PREFIX}'."
  echo ""
}

main "$@"
