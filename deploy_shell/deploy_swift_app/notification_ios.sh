#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# iOS 构建通知脚本
# 依赖 deploy_config.sh 配置项：
#   - 必填：PROJECT_NAME
#   - 可选：WECHAT_WEBHOOK_URL（未配置则跳过通知）
#   - 可选：JENKINS_BUILD_URL_BASE（用于拼接构建链接）
#   - 可选兼容字段：JENKINS_BUILD_URL
#   - 可选兜底字段（当命令行参数未传入时使用）：BuildBranch、BuildType、BuildEnv
# =============================================================================

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
  if [[ -z "$value" ]]; then
    echo "[ERROR] PROJECT_NAME 不能为空，请在 deploy_config.sh 中定义。命名规范：xxx_app" >&2
    exit 1
  fi
  if [[ ! "$value" =~ ^[a-z0-9]+(_[a-z0-9]+)*_app$ ]]; then
    echo "[ERROR] PROJECT_NAME 不符合命名规范: ${value}。仅支持下划线命名，示例：xxx_app" >&2
    exit 1
  fi
}

is_true() {
  case "${1:-}" in
    1|true|TRUE|True|yes|YES|Yes|y|Y|on|ON|On) return 0 ;;
    *) return 1 ;;
  esac
}

is_placeholder() {
  local value="${1:-}"
  [[ -z "$value" ]] && return 0
  case "$value" in
    YOUR_*|your_*|CHANGEME*|changeme*|PLACEHOLDER*|placeholder*) return 0 ;;
    *) return 1 ;;
  esac
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --config)
      [[ $# -ge 2 ]] || { echo "[ERROR] --config 缺少路径参数" >&2; exit 1; }
      DEPLOY_CONFIG_FILE="$2"
      shift 2
      ;;
    --version)
      VERSION="$2"
      shift 2
      ;;
    --version_code)
      VERSION_CODE="$2"
      shift 2
      ;;
    --branch_name)
      BRANCH_NAME="$2"
      shift 2
      ;;
    --build_id)
      BUILD_ID="$2"
      shift 2
      ;;
    --artifact_path)
      ARTIFACT_PATH="$2"
      shift 2
      ;;
    --channel)
      CHANNEL="$2"
      shift 2
      ;;
    --build_status)
      BUILD_STATUS="$2"
      shift 2
      ;;
    --environment)
      ENVIRONMENT="$2"
      shift 2
      ;;
    --commit_message)
      COMMIT_MESSAGE="$2"
      shift 2
      ;;
    --error)
      ERROR_MSG="$2"
      shift 2
      ;;
    *)
      echo "[ERROR] 未知参数: $1" >&2
      echo "[ERROR] 用法: notification_ios.sh --config <deploy_config.sh绝对路径> --version <版本号> --version_code <构建号> --branch_name <分支> --build_id <构建ID> --artifact_path <产物路径> --channel <打包渠道> --build_status <success|failure> --environment <环境> [--error <message>]" >&2
      exit 1
      ;;
  esac
done

[[ -n "$DEPLOY_CONFIG_FILE" ]] || { echo "[ERROR] 必须传入 --config <deploy_config.sh绝对路径>" >&2; exit 1; }
[[ -f "$DEPLOY_CONFIG_FILE" ]] || { echo "[ERROR] 找不到 deploy_config.sh: $DEPLOY_CONFIG_FILE" >&2; exit 1; }
PROJECT_ROOT="$(cd -- "$(dirname -- "$DEPLOY_CONFIG_FILE")" && pwd)"

# shellcheck disable=SC1090
source "$DEPLOY_CONFIG_FILE"

validate_project_name "${PROJECT_NAME:-}"
WECHAT_WEBHOOK_URL="${WECHAT_WEBHOOK_URL:-}"

if is_placeholder "$WECHAT_WEBHOOK_URL"; then
  log_warn "未配置 WECHAT_WEBHOOK_URL，跳过通知"
  exit 0
fi

if ! command -v curl >/dev/null 2>&1; then
  log_warn "未检测到 curl，跳过通知"
  exit 0
fi

BUILD_TIME="$(date '+%Y-%m-%d %H:%M:%S')"
if [[ -z "$BRANCH_NAME" ]]; then
  BRANCH_NAME="${BuildBranch:-master}"
fi
if [[ -z "$CHANNEL" ]]; then
  CHANNEL="${BuildType:-app-store}"
fi
if [[ -z "$ENVIRONMENT" ]]; then
  ENVIRONMENT="${BuildEnv:-test}"
fi
if [[ -z "$BUILD_ID" ]]; then
  BUILD_ID="${BUILD_NUMBER:-}"
fi

# 读取提交信息：优先使用显式传入，其次读取当前仓库最新提交
if [[ -z "$COMMIT_MESSAGE" ]]; then
  if command -v git >/dev/null 2>&1 && git -C "$PROJECT_ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    COMMIT_MESSAGE="$(git -C "$PROJECT_ROOT" log -1 --pretty=%s 2>/dev/null || true)"
  fi
fi

JENKINS_BUILD_LINK="$(resolve_jenkins_build_link "$BUILD_ID")"
notification_init_content CONTENT "$BUILD_STATUS" "ios" "build" "$PROJECT_NAME"
notification_append_emoji_line CONTENT "🕒" "时间" "$BUILD_TIME"
notification_append_emoji_line CONTENT "🏷️" "版本" "${VERSION:---}(${VERSION_CODE:---})"
notification_append_emoji_line CONTENT "🌍" "环境" "${ENVIRONMENT:---}"
notification_append_emoji_line CONTENT "🌿" "分支" "${BRANCH_NAME:---}"
notification_append_emoji_line CONTENT "🆔" "构建ID" "${BUILD_ID:---}"
notification_append_emoji_line CONTENT "📝" "提交信息" "${COMMIT_MESSAGE:---}"
notification_append_emoji_line CONTENT "📱" "构建平台" "iOS"
notification_append_emoji_line CONTENT "🧭" "构建渠道" "$CHANNEL"
if [[ "$BUILD_STATUS" == "success" ]]; then
  notification_append_emoji_line CONTENT "🧾" "构建产物" "$ARTIFACT_PATH"
fi
notification_append_emoji_link_line CONTENT "🔗" "详情链接" "查看 Jenkins 构建" "$JENKINS_BUILD_LINK"

if [[ -n "$ERROR_MSG" ]]; then
  notification_append_emoji_line CONTENT "⚠️" "异常信息" "<font color=\"warning\">${ERROR_MSG}</font>"
fi

if ! notification_send_markdown "$WECHAT_WEBHOOK_URL" "$CONTENT"; then
  log_warn "通知发送失败（不会中断构建流程）"
  exit 0
fi

log_info "通知发送成功: status=${BUILD_STATUS}, project=${PROJECT_NAME}"
