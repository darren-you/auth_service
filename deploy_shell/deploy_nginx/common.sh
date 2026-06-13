#!/usr/bin/env bash
# shellcheck shell=bash

COMMON_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$COMMON_DIR/../shared/jenkins_build_link.sh"
# shellcheck disable=SC1091
source "$COMMON_DIR/../shared/notification_markdown.sh"

log_info() {
  echo "[$(date '+%H:%M:%S')][INFO] $*"
}

log_warn() {
  echo "[$(date '+%H:%M:%S')][WARN] $*" >&2
}

log_error() {
  echo "[$(date '+%H:%M:%S')][ERROR] $*" >&2
}

die() {
  log_error "$*"
  exit 1
}

require_cmd() {
  local cmd="$1"
  command -v "$cmd" >/dev/null 2>&1 || die "未检测到命令: $cmd"
}

is_true() {
  local value="${1:-false}"
  value="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')"
  case "$value" in
    1|true|yes|y|on) return 0 ;;
    *) return 1 ;;
  esac
}

is_placeholder() {
  local value="${1:-}"
  [[ -z "$value" || "$value" == your_* ]]
}

validate_project_name() {
  local value="${1:-}"
  [[ -n "$value" ]] || die "PROJECT_NAME 不能为空，请在 deploy_config.sh 中定义。命名规范：xxx_server、xxx_app、xxx_web、xxx_nginx"
  if [[ ! "$value" =~ ^[a-z0-9]+(_[a-z0-9]+)*_(server|app|web|nginx)$ ]]; then
    die "PROJECT_NAME 不符合命名规范: ${value}。仅支持下划线命名，示例：xxx_server、xxx_app、xxx_web、xxx_nginx"
  fi
}

resolve_branch() {
  local branch="${BRANCH_NAME:-${GIT_BRANCH:-}}"
  if [[ -z "$branch" ]]; then
    branch="$(git -C "$SERVER_DIR" rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)"
  fi
  branch="${branch#origin/}"
  branch="${branch#refs/heads/}"
  echo "$branch"
}

sanitize_tag_part() {
  local raw="${1:-unknown}"
  raw="$(echo "$raw" | tr '/:@ ' '-' | tr -cd 'a-zA-Z0-9_.-')"
  if [[ -z "$raw" ]]; then
    raw="unknown"
  fi
  echo "$raw"
}

validate_build_env() {
  local value="${1:-}"
  case "$value" in
    production|development) return 0 ;;
    *)
      die "BUILD_ENV 仅支持 production 或 development，当前: ${value:-<empty>}"
      ;;
  esac
}

env_short() {
  local env_name="${1:-development}"
  if [[ "$env_name" == "production" ]]; then
    echo "prod"
  else
    echo "dev"
  fi
}

now_time() {
  date '+%Y-%m-%d %H:%M:%S'
}

resolve_short_sha() {
  git -C "$SERVER_DIR" rev-parse --short HEAD 2>/dev/null || echo "unknown"
}

resolve_commit_msg() {
  git -C "$SERVER_DIR" log -1 --pretty=%s 2>/dev/null || echo "No commit message"
}

json_escape() {
  local text="${1:-}"
  printf '%s' "$text" | awk '
    BEGIN { first = 1 }
    {
      gsub(/\\/, "\\\\")
      gsub(/"/, "\\\"")
      if (!first) {
        printf "\\n"
      }
      printf "%s", $0
      first = 0
    }
  '
}
