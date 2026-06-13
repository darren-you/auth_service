#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_CONFIG_FILE="${DEPLOY_CONFIG_FILE:-}"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/common.sh"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --config)
      [[ $# -ge 2 ]] || { echo "[ERROR] --config 缺少路径参数" >&2; exit 1; }
      DEPLOY_CONFIG_FILE="$2"
      shift 2
      ;;
    *)
      echo "[ERROR] 未知参数: $1" >&2
      echo "[ERROR] 用法: remote_deploy_core.sh --config <deploy_config.sh绝对路径>" >&2
      exit 1
      ;;
  esac
done

[[ -n "$DEPLOY_CONFIG_FILE" ]] || die "必须通过 --config 传入 deploy_config.sh，或由 remote_deploy_pipeline.sh 自动注入 DEPLOY_CONFIG_FILE"
[[ -f "$DEPLOY_CONFIG_FILE" ]] || die "找不到 deploy_config.sh: $DEPLOY_CONFIG_FILE"
export DEPLOY_CONFIG_FILE

PROJECT_ROOT="$(cd -- "$(dirname -- "$DEPLOY_CONFIG_FILE")" && pwd)"
# shellcheck disable=SC1090
source "$DEPLOY_CONFIG_FILE"
validate_project_name "${PROJECT_NAME:-}"

META_FILE="${META_FILE:-}"

[[ -n "$META_FILE" ]] || die "META_FILE 未设置（由 remote_deploy_pipeline.sh 自动注入）"
export META_FILE
load_meta

require_cmd rsync
require_cmd ssh

LOCAL_ARTIFACT_PATH="${ARTIFACT_LOCAL_PATH:-}"
[[ -n "$LOCAL_ARTIFACT_PATH" ]] || die "metadata 缺少 ARTIFACT_LOCAL_PATH"
[[ -f "$LOCAL_ARTIFACT_PATH" ]] || die "产物文件不存在: $LOCAL_ARTIFACT_PATH"
REMOTE_ARTIFACT_NAME="$(basename "$LOCAL_ARTIFACT_PATH")"

REMOTE_PROJECT_DIR="${REMOTE_PROJECT_DIR:-$REMOTE_DEPLOY_PROJECT_DIR}"
[[ -n "$REMOTE_PROJECT_DIR" ]] || die "REMOTE_PROJECT_DIR 不能为空"

TARGET_HOST="${DEPLOY_USER}@${DEPLOY_HOST}"
USE_SUDO="$(printf '%s' "${REMOTE_USE_SUDO:-false}" | tr '[:upper:]' '[:lower:]')"

SSH_OPTS=(-p "$DEPLOY_PORT" -o ConnectTimeout=10 -o ServerAliveInterval=30 -o ServerAliveCountMax=6)
if [[ -n "$DEPLOY_SSH_KEY_PATH" ]]; then
  SSH_OPTS+=(-i "$DEPLOY_SSH_KEY_PATH")
fi
if [[ -n "$DEPLOY_SSH_OPTIONS" ]]; then
  # shellcheck disable=SC2206
  EXTRA_SSH_OPTS=($DEPLOY_SSH_OPTIONS)
  SSH_OPTS+=("${EXTRA_SSH_OPTS[@]}")
fi

SSH_BASE=(ssh "${SSH_OPTS[@]}")
if [[ -n "${DEPLOY_SSH_PASSWORD:-}" ]]; then
  if command -v sshpass >/dev/null 2>&1; then
    SSH_CMD=(sshpass -p "$DEPLOY_SSH_PASSWORD" "${SSH_BASE[@]}")
  else
    if [[ -t 0 && -t 1 ]]; then
      log_warn "未安装 sshpass，将使用 ssh 交互输入密码。"
      SSH_CMD=("${SSH_BASE[@]}")
    else
      die "未检测到命令: sshpass。请安装 sshpass，或改用 DEPLOY_SSH_KEY_PATH 配置免密登录。"
    fi
  fi
else
  SSH_CMD=("${SSH_BASE[@]}")
fi

RSYNC_RSH="ssh"
for opt in "${SSH_OPTS[@]}"; do
  RSYNC_RSH+=" ${opt}"
done
if [[ -n "${DEPLOY_SSH_PASSWORD:-}" && -x "$(command -v sshpass 2>/dev/null || true)" ]]; then
  RSYNC_CMD=(sshpass -p "$DEPLOY_SSH_PASSWORD" rsync -az --progress)
else
  RSYNC_CMD=(rsync -az --progress)
fi

mkdir_cmd="mkdir -p \"${REMOTE_DEPLOY_BASE_DIR}\""
delete_cmd="rm -rf \"${REMOTE_PROJECT_DIR}\""
create_cmd="mkdir -p \"${REMOTE_PROJECT_DIR}\""
chmod_cmd="chmod -R ${REMOTE_MODE} \"${REMOTE_PROJECT_DIR}\""
chown_cmd="chown -R ${REMOTE_OWNER}:${REMOTE_GROUP} \"${REMOTE_PROJECT_DIR}\""

if [[ "$USE_SUDO" == "true" || "$USE_SUDO" == "1" || "$USE_SUDO" == "yes" ]]; then
  mkdir_cmd="sudo -n ${mkdir_cmd}"
  delete_cmd="sudo -n ${delete_cmd}"
  create_cmd="sudo -n ${create_cmd}"
  chmod_cmd="sudo -n ${chmod_cmd}"
  chown_cmd="sudo -n ${chown_cmd}"
fi

remote_exec() {
  local cmd="$1"
  "${SSH_CMD[@]}" "$TARGET_HOST" "$cmd"
}

log_info "开始远程部署: ${TARGET_HOST}:${DEPLOY_PORT}"
log_info "远端目录: $REMOTE_PROJECT_DIR"
log_info "本地产物: $LOCAL_ARTIFACT_PATH"

remote_exec "$mkdir_cmd"
log_info "清理远端目录（若存在）: $REMOTE_PROJECT_DIR"
remote_exec "$delete_cmd"
log_info "重建远端目录: $REMOTE_PROJECT_DIR"
remote_exec "$create_cmd"

RSYNC_EXTRA=()
if [[ "$USE_SUDO" == "true" || "$USE_SUDO" == "1" || "$USE_SUDO" == "yes" ]]; then
  RSYNC_EXTRA+=(--rsync-path="sudo -n rsync")
fi

"${RSYNC_CMD[@]}" "${RSYNC_EXTRA[@]}" -e "$RSYNC_RSH" "$LOCAL_ARTIFACT_PATH" "${TARGET_HOST}:${REMOTE_PROJECT_DIR}/"

EXTRACT_CMD="cd \"${REMOTE_PROJECT_DIR}\" \
&& unzip -oq \"${REMOTE_ARTIFACT_NAME}\" \
&& if [ -d \"dist\" ]; then find \"dist\" -mindepth 1 -maxdepth 1 -exec mv -f {} . \\; && rmdir \"dist\" || true; fi \
&& rm -f \"${REMOTE_ARTIFACT_NAME}\""
if [[ "$USE_SUDO" == "true" || "$USE_SUDO" == "1" || "$USE_SUDO" == "yes" ]]; then
  remote_exec "sudo -n bash -lc $(printf '%q' "$EXTRACT_CMD")"
else
  remote_exec "$EXTRACT_CMD"
fi

remote_exec "$chown_cmd"
remote_exec "$chmod_cmd"

DEPLOY_TIME_NOW="$(now_time)"
DEPLOY_ARTIFACT_REMOTE="${REMOTE_PROJECT_DIR}/index.html"
write_meta DEPLOY_TIME "$DEPLOY_TIME_NOW"
write_meta DEPLOY_TARGET "${TARGET_HOST}:${DEPLOY_PORT}"
write_meta DEPLOY_ARTIFACT_REMOTE "$DEPLOY_ARTIFACT_REMOTE"
write_meta DEPLOY_REMOTE_OWNER "${REMOTE_OWNER}:${REMOTE_GROUP}"
write_meta DEPLOY_REMOTE_MODE "$REMOTE_MODE"

log_info "产物上传完成: $DEPLOY_ARTIFACT_REMOTE"
log_info "权限设置完成: owner=${REMOTE_OWNER}:${REMOTE_GROUP}, mode=${REMOTE_MODE}"
