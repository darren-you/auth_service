#!/usr/bin/env bash
# shellcheck shell=bash

COMMON_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$COMMON_DIR/../shared/jenkins_build_link.sh"
# shellcheck disable=SC1091
source "$COMMON_DIR/../shared/notification_markdown.sh"

# Nexus Docker 私仓默认配置（默认值定义在部署脚本侧，而非业务项目配置）
readonly NEXUS_DOCKER_HOSTED_REGISTRY_DEFAULT="docker-hosted.xdarren.com"
readonly NEXUS_DOCKER_GROUP_REGISTRY_DEFAULT="docker-group.xdarren.com"
readonly NEXUS_DOCKER_HOSTED_LOCAL_REGISTRY_DEFAULT="127.0.0.1:8082"
readonly NEXUS_DOCKER_NAMESPACE_DEFAULT="server"
readonly NEXUS_DOCKER_USERNAME_DEFAULT="admin"
readonly NEXUS_DOCKER_PASSWORD_DEFAULT="admin123"
readonly GO_MODULE_PROXY_DEFAULT="https://goproxy.cn,direct"
readonly GO_SUM_DB_DEFAULT="sum.golang.google.cn"

ensure_common_tool_path() {
  local dir="${1:-}"
  [[ -n "$dir" && -d "$dir" ]] || return 0
  case ":${PATH:-}:" in
    *":$dir:"*) ;;
    *) PATH="$dir:${PATH:-}" ;;
  esac
}

bootstrap_common_tool_paths() {
  ensure_common_tool_path "/opt/homebrew/bin"
  ensure_common_tool_path "/opt/homebrew/sbin"
  ensure_common_tool_path "/usr/local/go/bin"
  ensure_common_tool_path "/usr/local/bin"
  ensure_common_tool_path "/usr/local/sbin"
  export PATH
}

bootstrap_common_tool_paths

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
  [[ -n "$value" ]] || die "PROJECT_NAME 不能为空，请在 deploy_config.sh 中定义。命名规范：xxx_server、xxx_app、xxx_web"
  if [[ ! "$value" =~ ^[a-z0-9]+(_[a-z0-9]+)*_(server|app|web)$ ]]; then
    die "PROJECT_NAME 不符合命名规范: ${value}。仅支持下划线命名，示例：xxx_server、xxx_app、xxx_web"
  fi
}

resolve_branch() {
  local branch="${BUILD_BRANCH:-${BuildBranch:-${BRANCH_NAME:-${GIT_BRANCH:-}}}}"
  if [[ -z "$branch" ]]; then
    branch="$(git -C "$SERVER_DIR" rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)"
  fi
  normalize_branch "$branch"
}

normalize_branch() {
  local branch="${1:-}"
  branch="${branch#origin/}"
  branch="${branch#refs/heads/}"
  if [[ -z "$branch" ]]; then
    branch="master"
  fi
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
    prod|test) return 0 ;;
    *)
      die "BuildEnv 仅支持 test 或 prod，当前: ${value:-<empty>}"
      ;;
  esac
}

normalize_build_env_input() {
  local value="${1:-}"
  value="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')"
  case "$value" in
    prod)
      echo "prod"
      ;;
    test|"")
      echo "test"
      ;;
    *)
      die "BuildEnv 仅支持 test 或 prod，当前: ${value:-<empty>}"
      ;;
  esac
}

to_internal_build_env() {
  normalize_build_env_input "${1:-test}"
}

env_display() {
  normalize_build_env_input "${1:-test}"
}

env_short() {
  local env_name="${1:-test}"
  validate_build_env "$env_name"
  if [[ "$env_name" == "prod" ]]; then
    echo "prod"
  else
    echo "test"
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

trim_space_edges() {
  local text="${1:-}"
  printf '%s' "$text" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//'
}

normalize_registry_host() {
  local value="${1:-}"
  value="$(trim_space_edges "$value")"
  value="${value#http://}"
  value="${value#https://}"
  value="${value%/}"
  printf '%s' "$value"
}

resolve_secret_value() {
  local direct_value="${1:-}"
  local file_path="${2:-}"
  local label="${3:-secret}"

  if [[ -n "$direct_value" ]]; then
    printf '%s' "$direct_value"
    return 0
  fi

  if [[ -n "$file_path" ]]; then
    [[ -f "$file_path" ]] || die "${label} 文件不存在: $file_path"
    awk 'NR == 1 { sub(/\r$/, "", $0); printf "%s", $0; exit }' "$file_path"
    return 0
  fi

  printf ''
}

compose_registry_image_repo() {
  local registry_host="${1:-}"
  local namespace="${2:-}"
  local image_name="${3:-}"

  registry_host="$(normalize_registry_host "$registry_host")"
  namespace="$(trim_space_edges "$namespace")"
  [[ -n "$registry_host" ]] || die "Docker Registry 不能为空"
  [[ -n "$image_name" ]] || die "镜像名不能为空"

  if [[ -n "$namespace" ]]; then
    printf '%s/%s/%s' "$registry_host" "$namespace" "$image_name"
  else
    printf '%s/%s' "$registry_host" "$image_name"
  fi
}

is_jenkins_build_context() {
  if [[ -n "${JENKINS_URL:-}" || -n "${JENKINS_HOME:-}" ]]; then
    return 0
  fi

  if [[ -n "${JOB_NAME:-}" && -n "${BUILD_NUMBER:-}" ]]; then
    return 0
  fi

  return 1
}
