#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/../shared/jenkins_build_link.sh"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/../shared/notification_markdown.sh"

DEPLOY_CONFIG_FILE="${DEPLOY_CONFIG_FILE:-}"
VERSION=""
VERSION_CODE=""
BRANCH_NAME=""
BUILD_ID=""
ARTIFACT_PATH=""
CHANNEL=""
BUILD_STATUS=""
ENVIRONMENT=""
ARTIFACT_KIND=""
COMMIT_MESSAGE=""
ERROR_MSG=""

log_info() {
  printf '[INFO] %s\n' "$*"
}

log_warn() {
  printf '[WARN] %s\n' "$*" >&2
}

validate_project_name() {
  local value="${1:-}"
  [[ -n "$value" ]] || {
    echo "[ERROR] PROJECT_NAME_ANDROID_APP 不能为空" >&2
    exit 1
  }
  [[ "$value" =~ ^[a-z0-9]+(_[a-z0-9]+)*_android_app$ ]] || {
    echo "[ERROR] PROJECT_NAME_ANDROID_APP 不符合命名规范: ${value}" >&2
    exit 1
  }
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --config) DEPLOY_CONFIG_FILE="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --version_code) VERSION_CODE="$2"; shift 2 ;;
    --branch_name) BRANCH_NAME="$2"; shift 2 ;;
    --build_id) BUILD_ID="$2"; shift 2 ;;
    --artifact_path) ARTIFACT_PATH="$2"; shift 2 ;;
    --channel) CHANNEL="$2"; shift 2 ;;
    --artifact_kind) ARTIFACT_KIND="$2"; shift 2 ;;
    --build_status) BUILD_STATUS="$2"; shift 2 ;;
    --environment) ENVIRONMENT="$2"; shift 2 ;;
    --commit_message) COMMIT_MESSAGE="$2"; shift 2 ;;
    --error) ERROR_MSG="$2"; shift 2 ;;
    *) echo "[ERROR] 未知参数: $1" >&2; exit 1 ;;
  esac
done

[[ -n "$DEPLOY_CONFIG_FILE" && -f "$DEPLOY_CONFIG_FILE" ]] || exit 0
PROJECT_ROOT="$(cd -- "$(dirname -- "$DEPLOY_CONFIG_FILE")" && pwd)"

# shellcheck disable=SC1090
source "$DEPLOY_CONFIG_FILE"

PROJECT_NAME="${PROJECT_NAME_ANDROID_APP:-}"
validate_project_name "$PROJECT_NAME"
WECHAT_WEBHOOK_URL="${WECHAT_WEBHOOK_URL:-}"
[[ -n "$WECHAT_WEBHOOK_URL" ]] || exit 0
command -v curl >/dev/null 2>&1 || exit 0

BUILD_TIME="$(date '+%Y-%m-%d %H:%M:%S')"

if [[ -z "$COMMIT_MESSAGE" ]]; then
  if command -v git >/dev/null 2>&1 && git -C "$PROJECT_ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    COMMIT_MESSAGE="$(git -C "$PROJECT_ROOT" log -1 --pretty=%s 2>/dev/null || true)"
  fi
fi

JENKINS_BUILD_LINK="$(resolve_jenkins_build_link "$BUILD_ID")"
notification_init_content CONTENT "$BUILD_STATUS" "android" "build" "$PROJECT_NAME"
notification_append_emoji_line CONTENT "🕒" "时间" "$BUILD_TIME"
notification_append_emoji_line CONTENT "🏷️" "版本" "${VERSION:---}(${VERSION_CODE:---})"
notification_append_emoji_line CONTENT "🌍" "环境" "${ENVIRONMENT:---}"
notification_append_emoji_line CONTENT "🌿" "分支" "${BRANCH_NAME:---}"
notification_append_emoji_line CONTENT "🆔" "构建ID" "${BUILD_ID:---}"
notification_append_emoji_line CONTENT "📝" "提交信息" "${COMMIT_MESSAGE:---}"
notification_append_emoji_line CONTENT "🤖" "构建平台" "Android"
notification_append_emoji_line CONTENT "🧭" "构建渠道" "$CHANNEL"
notification_append_emoji_line CONTENT "📦" "产物类型" "$ARTIFACT_KIND"
if [[ "$BUILD_STATUS" == "success" ]]; then
  notification_append_emoji_line CONTENT "🧾" "构建产物" "$ARTIFACT_PATH"
fi
notification_append_emoji_link_line CONTENT "🔗" "详情链接" "查看 Jenkins 构建" "$JENKINS_BUILD_LINK"

if [[ -n "$ERROR_MSG" ]]; then
  notification_append_emoji_line CONTENT "⚠️" "异常信息" "<font color=\"warning\">${ERROR_MSG}</font>"
fi

if ! notification_send_markdown "$WECHAT_WEBHOOK_URL" "$CONTENT"; then
  log_warn "通知发送失败（不会中断构建流程）"
fi

log_info "通知发送成功: status=${BUILD_STATUS}, project=${PROJECT_NAME}"
