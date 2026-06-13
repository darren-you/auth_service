#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_CONFIG_FILE="${DEPLOY_CONFIG_FILE:-}"
STAGE=""
STATUS=""
ERROR_MSG=""

early_error() {
  echo "[$(date '+%H:%M:%S')][ERROR] $*" >&2
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --config)
      [[ $# -ge 2 ]] || { early_error "--config 缺少路径参数"; exit 1; }
      DEPLOY_CONFIG_FILE="$2"
      shift 2
      ;;
    --stage)
      STAGE="$2"
      shift 2
      ;;
    --status)
      STATUS="$2"
      shift 2
      ;;
    --error)
      ERROR_MSG="$2"
      shift 2
      ;;
    *)
      early_error "未知参数: $1"
      early_error "用法: send_notification.sh [--config <deploy_config.sh绝对路径>] --stage <build|deploy> --status <success|failed> [--error <message>]"
      exit 1
      ;;
  esac
done

[[ -n "$DEPLOY_CONFIG_FILE" ]] || { early_error "必须通过 --config 传入 deploy_config.sh，或由 remote_deploy_pipeline.sh 自动注入 DEPLOY_CONFIG_FILE"; exit 1; }
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
# shellcheck disable=SC1091
source "$SCRIPT_DIR/../shared/jenkins_build_link.sh"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/../shared/notification_markdown.sh"
WECHAT_WEBHOOK_URL="${WECHAT_WEBHOOK_URL:-}"

format_port_mapping() {
  local mapping="${1:-}"
  local host_port=""
  local container_port=""

  if [[ -z "$mapping" || "$mapping" == "--" ]]; then
    printf '%s' "${mapping:---}"
    return 0
  fi

  # 仅支持 HOST:CONTAINER 两段格式
  if [[ ! "$mapping" =~ ^[0-9]+:[0-9]+$ ]]; then
    printf '%s' "$mapping"
    return 0
  fi

  host_port="${mapping%%:*}"
  container_port="${mapping##*:}"
  printf '主机 %s -> 容器 %s' "$host_port" "$container_port"
}

[[ -n "$STAGE" ]] || die "请通过 --stage 指定阶段(build/deploy)"
[[ -n "$STATUS" ]] || die "请通过 --status 指定状态(success/failed)"

if is_placeholder "$WECHAT_WEBHOOK_URL"; then
  log_info "未配置 WECHAT_WEBHOOK_URL，跳过通知"
  exit 0
fi

require_cmd curl

validate_project_name "${PROJECT_NAME:-}"
BUILD_BRANCH="${BUILD_BRANCH:-$(resolve_branch)}"
GIT_SHA="${GIT_SHA:-$(resolve_short_sha)}"
COMMIT_MSG="${COMMIT_MSG:-$(resolve_commit_msg)}"
if [[ -n "${BUILD_ENV_DISPLAY:-}" ]]; then
  BUILD_ENV_DISPLAY="$(normalize_build_env_input "$BUILD_ENV_DISPLAY")"
else
  BUILD_ENV_DISPLAY="$(normalize_build_env_input "${BuildEnv:-test}")"
fi
BUILD_ID="${BUILD_ID:-${BUILD_NUMBER:-}}"
IMAGE_REPO="${IMAGE_REPO:-${LOCAL_IMAGE_REPO:-$PROJECT_NAME}}"
DEPLOY_IMAGE="${DEPLOY_IMAGE:-unknown}"
BUILD_TIME="${BUILD_TIME:-$(now_time)}"
DEPLOY_TIME="${DEPLOY_TIME:-$(now_time)}"
DEPLOY_TARGET="${DEPLOY_TARGET:-${DEPLOY_USER:-unknown}@${DEPLOY_HOST:-unknown}:${DEPLOY_PORT:-22}}"
DEPLOY_CONTAINER="${DEPLOY_CONTAINER:-unknown}"
DEPLOY_PORT_MAPPING="${DEPLOY_PORT_MAPPING:--}"
DEPLOY_PORT_MAPPING_DISPLAY="$(format_port_mapping "$DEPLOY_PORT_MAPPING")"

JENKINS_BUILD_LINK="$(resolve_jenkins_build_link "$BUILD_ID")"

EVENT_TIME="$BUILD_TIME"
if [[ "$STAGE" == "deploy" ]]; then
  EVENT_TIME="$DEPLOY_TIME"
fi

notification_init_content CONTENT "$STATUS" "server" "$STAGE" "$PROJECT_NAME"
notification_append_emoji_line CONTENT "🕒" "时间" "$EVENT_TIME"
notification_append_emoji_line CONTENT "🌍" "环境" "$BUILD_ENV_DISPLAY"
notification_append_emoji_line CONTENT "🌿" "分支" "$BUILD_BRANCH"
notification_append_emoji_line CONTENT "🆔" "构建ID" "${BUILD_ID:---}"
notification_append_emoji_line CONTENT "📝" "提交信息" "${COMMIT_MSG:---}"

if [[ "$STAGE" == "deploy" ]]; then
  notification_append_emoji_line CONTENT "🖥️" "部署目标" "$DEPLOY_TARGET"
  notification_append_emoji_line CONTENT "📦" "部署容器" "$DEPLOY_CONTAINER"
  notification_append_emoji_line CONTENT "🔌" "端口映射" "$DEPLOY_PORT_MAPPING_DISPLAY"
else
  notification_append_emoji_line CONTENT "🐳" "镜像" "$DEPLOY_IMAGE"
fi

notification_append_emoji_link_line CONTENT "🔗" "详情链接" "查看 Jenkins 构建" "$JENKINS_BUILD_LINK"

if [[ -n "$ERROR_MSG" ]]; then
  notification_append_emoji_line CONTENT "⚠️" "异常信息" "<font color=\"warning\">${ERROR_MSG}</font>"
fi

notification_send_markdown "$WECHAT_WEBHOOK_URL" "$CONTENT"

log_info "通知发送成功: stage=$STAGE, status=$STATUS"
