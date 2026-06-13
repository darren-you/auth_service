#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_CONFIG_FILE="${DEPLOY_CONFIG_FILE:-}"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/common.sh"

STAGE=""
STATUS=""
ERROR_MSG=""
META_FILE="${META_FILE:-}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --config)
      [[ $# -ge 2 ]] || { echo "[ERROR] --config 缺少路径参数" >&2; exit 1; }
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
      die "未知参数: $1"
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
WECHAT_WEBHOOK_URL="${WECHAT_WEBHOOK_URL:-}"

[[ -n "$STAGE" ]] || die "请通过 --stage 指定阶段(build/deploy)"
[[ -n "$STATUS" ]] || die "请通过 --status 指定状态(success/failed)"

if is_placeholder "$WECHAT_WEBHOOK_URL"; then
  log_info "未配置 WECHAT_WEBHOOK_URL，跳过通知"
  exit 0
fi

require_cmd curl

if [[ -n "$META_FILE" && -f "$META_FILE" ]]; then
  export META_FILE
  load_meta
fi

BUILD_BRANCH="${BUILD_BRANCH:-$(resolve_branch)}"
GIT_SHA="${GIT_SHA:-$(resolve_short_sha)}"
COMMIT_MSG="${COMMIT_MSG:-$(resolve_commit_msg)}"
BUILD_ID="${BUILD_ID:-${BUILD_NUMBER:-}}"
JENKINS_BUILD_LINK="$(resolve_jenkins_build_link "$BUILD_ID")"
BUILD_TIME="${BUILD_TIME:-$(now_time)}"
DEPLOY_TIME="${DEPLOY_TIME:-$(now_time)}"
EVENT_TIME="$BUILD_TIME"
ARTIFACT_LOCAL_PATH="${ARTIFACT_LOCAL_PATH:-$DIST_ZIP_PATH}"
DEPLOY_TARGET="${DEPLOY_TARGET:-${DEPLOY_USER:-unknown}@${DEPLOY_HOST:-unknown}:${DEPLOY_PORT:-22}}"
DEPLOY_ARTIFACT_REMOTE="${DEPLOY_ARTIFACT_REMOTE:-unknown}"
DEPLOY_REMOTE_OWNER="${DEPLOY_REMOTE_OWNER:-${REMOTE_OWNER}:${REMOTE_GROUP}}"
DEPLOY_REMOTE_MODE="${DEPLOY_REMOTE_MODE:-$REMOTE_MODE}"

if [[ "$STAGE" == "deploy" ]]; then
  EVENT_TIME="$DEPLOY_TIME"
fi

notification_init_content CONTENT "$STATUS" "web" "$STAGE" "$PROJECT_NAME"
notification_append_emoji_line CONTENT "🕒" "时间" "$EVENT_TIME"
notification_append_emoji_line CONTENT "🌿" "分支" "$BUILD_BRANCH"
notification_append_emoji_line CONTENT "🆔" "构建ID" "${BUILD_ID:---}"
notification_append_emoji_line CONTENT "📝" "提交信息" "${COMMIT_MSG:---}"

if [[ "$STAGE" == "build" ]]; then
  notification_append_emoji_line CONTENT "🧾" "构建产物" "$ARTIFACT_LOCAL_PATH"
else
  notification_append_emoji_line CONTENT "🖥️" "部署目标" "$DEPLOY_TARGET"
  notification_append_emoji_line CONTENT "📁" "部署路径" "$DEPLOY_ARTIFACT_REMOTE"
  notification_append_emoji_line CONTENT "👤" "文件属主" "$DEPLOY_REMOTE_OWNER"
  notification_append_emoji_line CONTENT "🔐" "目录权限" "$DEPLOY_REMOTE_MODE"
fi

notification_append_emoji_link_line CONTENT "🔗" "详情链接" "查看 Jenkins 构建" "$JENKINS_BUILD_LINK"

if [[ -n "$ERROR_MSG" ]]; then
  notification_append_emoji_line CONTENT "⚠️" "异常信息" "<font color=\"warning\">${ERROR_MSG}</font>"
fi

notification_send_markdown "$WECHAT_WEBHOOK_URL" "$CONTENT"

log_info "通知发送成功: stage=$STAGE, status=$STATUS"
