#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_CONFIG_FILE=""
STAGE_COMMAND_ERROR=""

early_error() {
  echo "[$(date '+%H:%M:%S')][ERROR] $*" >&2
}

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

  stage_log_file="$(mktemp "/tmp/pipeline_stage.XXXXXX")"
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
      [[ $# -ge 2 ]] || { early_error "--config 缺少路径参数"; exit 1; }
      DEPLOY_CONFIG_FILE="$2"
      shift 2
      ;;
    *)
      early_error "未知参数: $1"
      early_error "用法: remote_deploy_pipeline.sh --config <deploy_config.sh绝对路径>"
      exit 1
      ;;
  esac
done

if [[ -z "$DEPLOY_CONFIG_FILE" ]]; then
  early_error "必须传入 --config <deploy_config.sh绝对路径>"
  exit 1
fi

if [[ ! -f "$DEPLOY_CONFIG_FILE" ]]; then
  early_error "找不到 deploy_config.sh: $DEPLOY_CONFIG_FILE"
  exit 1
fi
export DEPLOY_CONFIG_FILE
PROJECT_ROOT="$(cd -- "$(dirname -- "$DEPLOY_CONFIG_FILE")" && pwd)"
# shellcheck disable=SC1090
source "$DEPLOY_CONFIG_FILE"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/common.sh"
validate_project_name "${PROJECT_NAME:-}"

DEPLOY_IMAGE_REF_FILE="${DEPLOY_IMAGE_REF_FILE:-$(mktemp "/tmp/${PROJECT_NAME}.deploy_image.XXXXXX")}"
KEEP_DEPLOY_IMAGE_REF_FILE="${KEEP_DEPLOY_IMAGE_REF_FILE:-false}"
export DEPLOY_IMAGE_REF_FILE
if ! is_true "$KEEP_DEPLOY_IMAGE_REF_FILE"; then
  trap 'rm -f "$DEPLOY_IMAGE_REF_FILE"' EXIT
fi

BUILD_BRANCH_INPUT="${BuildBranch:-${BUILD_BRANCH:-${GIT_BRANCH:-master}}}"
BUILD_BRANCH="$(normalize_branch "$BUILD_BRANCH_INPUT")"
BRANCH_NAME="$BUILD_BRANCH"
export BUILD_BRANCH BRANCH_NAME

BUILD_ENV_INPUT="${BuildEnv:-test}"
BUILD_ENV_DISPLAY="$(normalize_build_env_input "$BUILD_ENV_INPUT")"
BUILD_ENV="$(to_internal_build_env "$BUILD_ENV_DISPLAY")"
export BUILD_ENV BUILD_ENV_DISPLAY

if [[ "$BUILD_ENV" == "prod" ]]; then
  DEPLOY_CONTAINER="$PROD_CONTAINER_NAME"
  DEPLOY_PORT_MAPPING="${PROD_HOST_PORT}:${REMOTE_CONTAINER_PORT}"
else
  DEPLOY_CONTAINER="$TEST_CONTAINER_NAME"
  DEPLOY_PORT_MAPPING="${TEST_HOST_PORT}:${REMOTE_CONTAINER_PORT}"
fi
DEPLOY_TARGET="${DEPLOY_USER}@${DEPLOY_HOST}:${DEPLOY_PORT}"
DEPLOY_NETWORK="$REMOTE_DOCKER_NETWORK"
export DEPLOY_CONTAINER DEPLOY_PORT_MAPPING DEPLOY_TARGET DEPLOY_NETWORK

log_info "开始执行部署流水线"
log_info "BuildBranch: ${BUILD_BRANCH_INPUT} (归一化: ${BUILD_BRANCH})"
log_info "BuildEnv: $BUILD_ENV"

BUILD_STATUS="success"
BUILD_ERROR=""
BUILD_TIME="$(now_time)"
export BUILD_TIME
if ! run_stage_command "docker build/package 执行失败" "$SCRIPT_DIR/docker_build_push.sh" --config "$DEPLOY_CONFIG_FILE"; then
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

if [[ -s "$DEPLOY_IMAGE_REF_FILE" ]]; then
  DEPLOY_IMAGE="$(tail -n 1 "$DEPLOY_IMAGE_REF_FILE" | tr -d '\r\n')"
  [[ -n "$DEPLOY_IMAGE" ]] || die "部署镜像引用文件为空: $DEPLOY_IMAGE_REF_FILE"
  export DEPLOY_IMAGE
  log_info "本次部署镜像已锁定: $DEPLOY_IMAGE"
else
  log_warn "未读取到本次部署镜像，将由 remote_deploy_core 按默认规则推断"
fi

DEPLOY_STATUS="success"
DEPLOY_ERROR=""
DEPLOY_TIME="$(now_time)"
export DEPLOY_TIME
if ! run_stage_command "镜像拉取及远程部署执行失败" "$SCRIPT_DIR/remote_deploy_core.sh" --config "$DEPLOY_CONFIG_FILE"; then
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
