#!/usr/bin/env bash
# shellcheck shell=bash

if [[ "${SECRET_BASE_COMMON_SH_LOADED:-0}" == "1" ]]; then
  return 0
fi
readonly SECRET_BASE_COMMON_SH_LOADED=1

readonly SECRET_BASE_DEFAULT_ROOT="/srv/darren_secret_base"

sb_ensure_common_tool_path() {
  local dir="${1:-}"
  [[ -n "$dir" && -d "$dir" ]] || return 0
  case ":${PATH:-}:" in
    *":$dir:"*) ;;
    *) PATH="$dir:${PATH:-}" ;;
  esac
}

sb_bootstrap_common_tool_paths() {
  sb_ensure_common_tool_path "/opt/homebrew/bin"
  sb_ensure_common_tool_path "/opt/homebrew/sbin"
  sb_ensure_common_tool_path "/usr/local/go/bin"
  sb_ensure_common_tool_path "/usr/local/bin"
  sb_ensure_common_tool_path "/usr/local/sbin"
  export PATH
}

sb_bootstrap_common_tool_paths

sb_log_info() {
  echo "[$(date '+%H:%M:%S')][SECRET_BASE][INFO] $*"
}

sb_log_warn() {
  echo "[$(date '+%H:%M:%S')][SECRET_BASE][WARN] $*" >&2
}

sb_log_error() {
  echo "[$(date '+%H:%M:%S')][SECRET_BASE][ERROR] $*" >&2
}

sb_die() {
  sb_log_error "$*"
  exit 1
}

sb_is_true() {
  local value="${1:-false}"
  value="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')"
  case "$value" in
    1|true|yes|y|on) return 0 ;;
    *) return 1 ;;
  esac
}

sb_require_cmd() {
  local cmd="${1:-}"
  command -v "$cmd" >/dev/null 2>&1 || sb_die "未检测到命令: $cmd"
}

sb_normalize_env() {
  local value="${1:-}"
  value="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')"
  case "$value" in
    prod)
      printf '%s' "prod"
      ;;
    test|"")
      printf '%s' "test"
      ;;
    *)
      sb_die "workspace_secret_base 仅支持 test / prod，当前: ${value:-<empty>}"
      ;;
  esac
}

sb_require_secret_base_enabled() {
  sb_is_true "${SECRET_BASE_ENABLED:-false}" || sb_die "当前 deploy_config.sh 未启用 workspace_secret_base，请设置 SECRET_BASE_ENABLED=true"
}

sb_require_project_identity() {
  [[ -n "${SECRET_BASE_REPO:-}" ]] || sb_die "deploy_config.sh 缺少 SECRET_BASE_REPO"
  [[ -n "${SECRET_BASE_SUBPROJECT:-}" ]] || sb_die "deploy_config.sh 缺少 SECRET_BASE_SUBPROJECT"
}

sb_resolve_root() {
  printf '%s' "${SECRET_BASE_ROOT:-$SECRET_BASE_DEFAULT_ROOT}"
}

sb_resolve_repo() {
  printf '%s' "${SECRET_BASE_REPO:-}"
}

sb_resolve_subproject() {
  printf '%s' "${SECRET_BASE_SUBPROJECT:-}"
}

sb_bundle_path() {
  local env_name="${1:-}"
  local root repo subproject normalized_env
  normalized_env="$(sb_normalize_env "$env_name")"
  root="$(sb_resolve_root)"
  repo="$(sb_resolve_repo)"
  subproject="$(sb_resolve_subproject)"
  printf '%s/bundles/%s/%s/%s.secrets.yaml.age' "$root" "$repo" "$subproject" "$normalized_env"
}

sb_runtime_dir() {
  local env_name="${1:-}"
  local root repo subproject normalized_env
  normalized_env="$(sb_normalize_env "$env_name")"
  root="$(sb_resolve_root)"
  repo="$(sb_resolve_repo)"
  subproject="$(sb_resolve_subproject)"
  printf '%s/runtime/%s/%s/%s' "$root" "$repo" "$subproject" "$normalized_env"
}

sb_runtime_config_path() {
  local env_name="${1:-}"
  printf '%s/config.yaml' "$(sb_runtime_dir "$env_name")"
}

sb_age_identity_path() {
  local root
  root="$(sb_resolve_root)"
  printf '%s/keys/age/active_key.txt' "$root"
}

sb_audit_log_path() {
  local root
  root="$(sb_resolve_root)"
  printf '%s/logs/render/%s.log' "$root" "$(date '+%F')"
}

sb_template_path() {
  local env_name="${1:-}"
  local normalized_env
  normalized_env="$(sb_normalize_env "$env_name")"
  [[ -n "${PROJECT_ROOT:-}" ]] || sb_die "PROJECT_ROOT 不能为空，无法定位模板配置"
  case "$normalized_env" in
    prod)
      printf '%s/config/config.prod.yaml' "$PROJECT_ROOT"
      ;;
    test)
      printf '%s/config/config.dev.yaml' "$PROJECT_ROOT"
      ;;
  esac
}

sb_secretctl_dir() {
  local script_dir
  script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
  printf '%s/secretctl' "$script_dir"
}
