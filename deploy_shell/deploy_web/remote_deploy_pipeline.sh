#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_CONFIG_FILE=""
STAGE_COMMAND_ERROR=""

extract_error_from_log() {
  local log_file="$1"
  local fallback_message="$2"
  local error_line=""
  local raw_error_line=""
  local shell_error_line=""
  local summary=""

  if [[ -s "$log_file" ]]; then
    raw_error_line="$(grep -E '(^ERROR:|^error:|failed to|unable to|not found|denied|TLS|timeout|no such file)' "$log_file" | tail -n 1 || true)"
    if [[ -n "$raw_error_line" ]]; then
      error_line="$(printf '%s' "$raw_error_line")"
    fi
    if [[ -z "$error_line" ]]; then
      shell_error_line="$(grep -E '\[ERROR\]' "$log_file" | tail -n 1 || true)"
      if [[ -n "$shell_error_line" ]]; then
        error_line="$(printf '%s' "$shell_error_line" | sed 's/^.*\[ERROR\] //')"
      fi
    fi
    if [[ -n "$error_line" ]]; then
      printf '%s' "$error_line"
      return 0
    fi

    summary="$(tail -n 20 "$log_file" | tr '\n' ' ' | sed 's/[[:space:]]\+/ /g; s/^ //; s/ $//')"
    if [[ -n "$summary" ]]; then
      printf '%s' "$summary"
      return 0
    fi
  fi

  printf '%s' "$fallback_message"
}

run_stage_command() {
  local fallback_message="$1"
  shift
  local stage_log_file=""

  stage_log_file="$(mktemp "/tmp/pipeline_stage.XXXXXX.log")"
  if ( "$@" ) > >(tee "$stage_log_file") 2> >(tee -a "$stage_log_file" >&2); then
    rm -f "$stage_log_file"
    STAGE_COMMAND_ERROR=""
    return 0
  else
    local rc=$?
    STAGE_COMMAND_ERROR="$(extract_error_from_log "$stage_log_file" "$fallback_message")"
    rm -f "$stage_log_file"
    return "$rc"
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --config)
      [[ $# -ge 2 ]] || { echo "[ERROR] --config 缺少路径参数" >&2; exit 1; }
      DEPLOY_CONFIG_FILE="$2"
      shift 2
      ;;
    *)
      echo "[ERROR] 未知参数: $1" >&2
      echo "[ERROR] 用法: bash deploy_shell/deploy_web/remote_deploy_pipeline.sh --config <deploy_config.sh绝对路径>" >&2
      exit 1
      ;;
  esac
done

if [[ -z "$DEPLOY_CONFIG_FILE" ]]; then
  echo "[ERROR] 必须传入 --config <deploy_config.sh绝对路径>" >&2
  exit 1
fi

if [[ ! -f "$DEPLOY_CONFIG_FILE" ]]; then
  echo "[ERROR] 找不到 deploy_config.sh: $DEPLOY_CONFIG_FILE" >&2
  exit 1
fi
export DEPLOY_CONFIG_FILE

PROJECT_ROOT="$(cd -- "$(dirname -- "$DEPLOY_CONFIG_FILE")" && pwd)"
# shellcheck disable=SC1090
source "$DEPLOY_CONFIG_FILE"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/common.sh"
validate_project_name "${PROJECT_NAME:-}"

cleanup_old_artifacts() {
  local web_dir="${WEB_DIR:-$PROJECT_ROOT}"
  local dist_dir="${DIST_DIR:-${web_dir%/}/dist}"
  local dist_zip_path="${DIST_ZIP_PATH:-${web_dir%/}/dist.zip}"

  [[ -n "$web_dir" ]] || die "WEB_DIR 为空，无法执行构建前清理"
  [[ -n "$dist_dir" ]] || die "DIST_DIR 为空，无法执行构建前清理"
  [[ -n "$dist_zip_path" ]] || die "DIST_ZIP_PATH 为空，无法执行构建前清理"
  [[ -d "$web_dir" ]] || die "WEB_DIR 不存在: $web_dir"

  web_dir="$(cd -- "$web_dir" && pwd)"
  if [[ "$dist_dir" != /* ]]; then
    dist_dir="${web_dir%/}/${dist_dir#./}"
  fi
  if [[ "$dist_zip_path" != /* ]]; then
    dist_zip_path="${web_dir%/}/${dist_zip_path#./}"
  fi

  # 安全保护：仅允许清理 WEB_DIR 目录内的产物路径
  case "$dist_dir" in
    "$web_dir"/*) ;;
    *)
      die "DIST_DIR 不在 WEB_DIR 内，拒绝清理: DIST_DIR=$dist_dir, WEB_DIR=$web_dir"
      ;;
  esac
  case "$dist_zip_path" in
    "$web_dir"/*) ;;
    *)
      die "DIST_ZIP_PATH 不在 WEB_DIR 内，拒绝清理: DIST_ZIP_PATH=$dist_zip_path, WEB_DIR=$web_dir"
      ;;
  esac
  [[ "$dist_dir" != "$web_dir" ]] || die "DIST_DIR 与 WEB_DIR 相同，拒绝清理: $dist_dir"

  if [[ -e "$dist_dir" ]]; then
    log_info "清理旧构建目录: $dist_dir"
    rm -rf "$dist_dir"
  else
    log_info "旧构建目录不存在，跳过: $dist_dir"
  fi

  if [[ -e "$dist_zip_path" ]]; then
    log_info "清理旧打包文件: $dist_zip_path"
    rm -f "$dist_zip_path"
  else
    log_info "旧打包文件不存在，跳过: $dist_zip_path"
  fi
}

run_build_stage() {
  cleanup_old_artifacts
  "$SCRIPT_DIR/npm_build_package.sh" --config "$DEPLOY_CONFIG_FILE"
}

META_FILE="${META_FILE:-$(mktemp "/tmp/${PROJECT_NAME}.deploy.XXXXXX")}"
KEEP_META_FILE="${KEEP_META_FILE:-false}"
export META_FILE

: > "$META_FILE"

if ! is_true "$KEEP_META_FILE"; then
  trap 'rm -f "$META_FILE"' EXIT
fi

log_info "开始执行前端部署流水线"
log_info "元数据文件: $META_FILE"

BUILD_STATUS="success"
BUILD_ERROR=""
if ! run_stage_command "npm build/package 执行失败" run_build_stage; then
  BUILD_STATUS="failed"
  BUILD_ERROR="$STAGE_COMMAND_ERROR"
fi

BUILD_NOTIFY_CMD=("$SCRIPT_DIR/send_notification.sh" --config "$DEPLOY_CONFIG_FILE" --stage build --status "$BUILD_STATUS")
if [[ -n "$BUILD_ERROR" ]]; then
  BUILD_NOTIFY_CMD+=(--error "$BUILD_ERROR")
fi
if ! "${BUILD_NOTIFY_CMD[@]}"; then
  log_warn "构建通知发送失败（不影响主流程）"
fi

if [[ "$BUILD_STATUS" != "success" ]]; then
  die "构建失败，终止部署"
fi

DEPLOY_STATUS="success"
DEPLOY_ERROR=""
if ! run_stage_command "rsync 上传及远端部署目录更新失败" "$SCRIPT_DIR/remote_deploy_core.sh" --config "$DEPLOY_CONFIG_FILE"; then
  DEPLOY_STATUS="failed"
  DEPLOY_ERROR="$STAGE_COMMAND_ERROR"
fi

DEPLOY_NOTIFY_CMD=("$SCRIPT_DIR/send_notification.sh" --config "$DEPLOY_CONFIG_FILE" --stage deploy --status "$DEPLOY_STATUS")
if [[ -n "$DEPLOY_ERROR" ]]; then
  DEPLOY_NOTIFY_CMD+=(--error "$DEPLOY_ERROR")
fi
if ! "${DEPLOY_NOTIFY_CMD[@]}"; then
  log_warn "部署通知发送失败（不影响主流程）"
fi

if [[ "$DEPLOY_STATUS" != "success" ]]; then
  die "部署失败"
fi

log_info "流水线执行成功"
