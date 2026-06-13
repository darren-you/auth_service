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
      echo "[ERROR] 用法: remote_deploy_pipeline.sh --config <deploy_config.sh绝对路径>" >&2
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
SERVER_DIR="${SERVER_DIR:-$PROJECT_ROOT}"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/common.sh"
validate_project_name "${PROJECT_NAME:-}"

BUILD_ENV="${BUILD_ENV:-production}"
validate_build_env "$BUILD_ENV"
export BUILD_ENV

run_build_stage() {
  "$SCRIPT_DIR/ssl_build_package.sh" --config "$DEPLOY_CONFIG_FILE"
}

run_deploy_stage() {
  LOCAL_HOOK_FILE_PATH="${ARTIFACT_LOCAL_PATH:-/tmp/${PROJECT_NAME}-cert-renewal-hook.sh}"
  [[ -f "$LOCAL_HOOK_FILE_PATH" ]] || die "本地 Hook 产物不存在: $LOCAL_HOOK_FILE_PATH"

  DEPLOY_HOST_VALUE="${DEPLOY_HOST:-}"
  DEPLOY_USER_VALUE="${DEPLOY_USER:-}"
  DEPLOY_PORT_VALUE="${DEPLOY_PORT:-22}"
  [[ -n "$DEPLOY_HOST_VALUE" ]] || die "DEPLOY_HOST 不能为空"
  [[ -n "$DEPLOY_USER_VALUE" ]] || die "DEPLOY_USER 不能为空"

  REMOTE_HOOK_SCRIPT_PATH_VALUE="${REMOTE_HOOK_SCRIPT_PATH:-/etc/letsencrypt/cert-renewal-hook.sh}"
  REMOTE_TMP_DIR_VALUE="${REMOTE_TMP_DIR:-/tmp}"
  REMOTE_TMP_FILE_VALUE="${REMOTE_TMP_FILE:-${REMOTE_TMP_DIR_VALUE%/}/$(basename "$REMOTE_HOOK_SCRIPT_PATH_VALUE").${PROJECT_NAME}.$(date +%s)}"
  REMOTE_MODE_VALUE="${REMOTE_MODE:-755}"
  REMOTE_USE_SUDO_VALUE="${REMOTE_USE_SUDO:-true}"
  if [[ -n "${REMOTE_HOOK_CHECK_CMD:-}" ]]; then
    REMOTE_HOOK_CHECK_CMD_VALUE="${REMOTE_HOOK_CHECK_CMD}"
  else
    REMOTE_HOOK_CHECK_CMD_VALUE="bash -n '${REMOTE_HOOK_SCRIPT_PATH_VALUE}'"
  fi

  require_cmd ssh
  require_cmd scp

  TARGET_HOST="${DEPLOY_USER_VALUE}@${DEPLOY_HOST_VALUE}"
  export TARGET_HOST DEPLOY_PORT_VALUE
  export LOCAL_HOOK_FILE_PATH REMOTE_HOOK_SCRIPT_PATH_VALUE REMOTE_TMP_DIR_VALUE REMOTE_TMP_FILE_VALUE REMOTE_MODE_VALUE
  export REMOTE_USE_SUDO_VALUE REMOTE_HOOK_CHECK_CMD_VALUE

  SSH_OPTS=(-p "$DEPLOY_PORT_VALUE" -o ConnectTimeout=10 -o ServerAliveInterval=30 -o ServerAliveCountMax=6)
  SCP_OPTS=(-P "$DEPLOY_PORT_VALUE" -o ConnectTimeout=10 -o ServerAliveInterval=30 -o ServerAliveCountMax=6)
  if [[ -n "${DEPLOY_SSH_KEY_PATH:-}" ]]; then
    SSH_OPTS+=(-i "$DEPLOY_SSH_KEY_PATH")
    SCP_OPTS+=(-i "$DEPLOY_SSH_KEY_PATH")
  fi
  if [[ -n "${DEPLOY_SSH_OPTIONS:-}" ]]; then
    # shellcheck disable=SC2206
    EXTRA_SSH_OPTS=($DEPLOY_SSH_OPTIONS)
    SSH_OPTS+=("${EXTRA_SSH_OPTS[@]}")
    SCP_OPTS+=("${EXTRA_SSH_OPTS[@]}")
  fi

  SSH_BASE=(ssh "${SSH_OPTS[@]}")
  SCP_BASE=(scp "${SCP_OPTS[@]}")

  if [[ -n "${DEPLOY_SSH_PASSWORD:-}" ]]; then
    if command -v sshpass >/dev/null 2>&1; then
      SSH_CMD=(sshpass -p "$DEPLOY_SSH_PASSWORD" "${SSH_BASE[@]}")
      SCP_CMD=(sshpass -p "$DEPLOY_SSH_PASSWORD" "${SCP_BASE[@]}")
    else
      if [[ -t 0 && -t 1 ]]; then
        log_warn "未安装 sshpass，将使用交互式输入密码。"
        SSH_CMD=("${SSH_BASE[@]}")
        SCP_CMD=("${SCP_BASE[@]}")
      else
        die "未检测到命令: sshpass。请安装 sshpass，或改用 DEPLOY_SSH_KEY_PATH 配置免密登录。"
      fi
    fi
  else
    SSH_CMD=("${SSH_BASE[@]}")
    SCP_CMD=("${SCP_BASE[@]}")
  fi

  remote_exec() {
    local cmd="$1"
    "${SSH_CMD[@]}" "$TARGET_HOST" "$cmd"
  }

  if is_true "$REMOTE_USE_SUDO_VALUE"; then
    REMOTE_SUDO_PREFIX="sudo -n "
  else
    REMOTE_SUDO_PREFIX=""
  fi

  export DEPLOY_TARGET="${TARGET_HOST}:${DEPLOY_PORT_VALUE}"
  export DEPLOY_ARTIFACT_REMOTE="$REMOTE_HOOK_SCRIPT_PATH_VALUE"

  log_info "开始上传 SSL renew hook"
  log_info "本地文件: $LOCAL_HOOK_FILE_PATH"
  log_info "目标机器: ${TARGET_HOST}:${DEPLOY_PORT_VALUE}"
  log_info "远端路径: $REMOTE_HOOK_SCRIPT_PATH_VALUE"

  "${SCP_CMD[@]}" "$LOCAL_HOOK_FILE_PATH" "${TARGET_HOST}:${REMOTE_TMP_FILE_VALUE}"
  remote_exec "${REMOTE_SUDO_PREFIX}install -m ${REMOTE_MODE_VALUE} '${REMOTE_TMP_FILE_VALUE}' '${REMOTE_HOOK_SCRIPT_PATH_VALUE}' && ${REMOTE_SUDO_PREFIX}rm -f '${REMOTE_TMP_FILE_VALUE}'"

  if is_true "$REMOTE_USE_SUDO_VALUE"; then
    remote_exec "${REMOTE_SUDO_PREFIX}bash -lc $(printf '%q' "$REMOTE_HOOK_CHECK_CMD_VALUE")"
  else
    remote_exec "$REMOTE_HOOK_CHECK_CMD_VALUE"
  fi
}

log_info "开始执行 SSL Hook 部署流水线"
log_info "BUILD_ENV: $BUILD_ENV"

BUILD_STATUS="success"
BUILD_ERROR=""
if ! run_stage_command "SSL Hook 构建失败" run_build_stage; then
  BUILD_STATUS="failed"
  BUILD_ERROR="$STAGE_COMMAND_ERROR"
fi

BUILD_NOTIFY_CMD=("$SCRIPT_DIR/send_notification.sh" --stage build --status "$BUILD_STATUS")
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
if ! run_stage_command "SSL Hook 上传与远端校验失败" run_deploy_stage; then
  DEPLOY_STATUS="failed"
  DEPLOY_ERROR="$STAGE_COMMAND_ERROR"
fi

DEPLOY_NOTIFY_CMD=("$SCRIPT_DIR/send_notification.sh" --stage deploy --status "$DEPLOY_STATUS")
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
