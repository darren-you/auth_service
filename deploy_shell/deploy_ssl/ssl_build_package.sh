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
      echo "[ERROR] 用法: ssl_build_package.sh [--config <deploy_config.sh绝对路径>]" >&2
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
SERVER_DIR="${SERVER_DIR:-$PROJECT_ROOT}"
validate_project_name "${PROJECT_NAME:-}"

BUILD_ENV_VALUE="${BUILD_ENV:-production}"
validate_build_env "$BUILD_ENV_VALUE"

LOCAL_HOOK_SOURCE_FILE="${LOCAL_HOOK_SOURCE_FILE:-$PROJECT_ROOT/certbot_renewal/scripts/cert-renewal-hook.sh}"
ARTIFACT_LOCAL_PATH="${ARTIFACT_LOCAL_PATH:-/tmp/${PROJECT_NAME}-cert-renewal-hook.sh}"

[[ -f "$LOCAL_HOOK_SOURCE_FILE" ]] || die "找不到 Hook 源文件: $LOCAL_HOOK_SOURCE_FILE"
mkdir -p "$(dirname -- "$ARTIFACT_LOCAL_PATH")"

log_info "构建 SSL renew hook 产物"
log_info "源文件: $LOCAL_HOOK_SOURCE_FILE"
log_info "产物路径: $ARTIFACT_LOCAL_PATH"

cp "$LOCAL_HOOK_SOURCE_FILE" "$ARTIFACT_LOCAL_PATH"
chmod 755 "$ARTIFACT_LOCAL_PATH"
bash -n "$ARTIFACT_LOCAL_PATH"

SHORT_SHA="$(resolve_short_sha)"
BRANCH_NAME="$(resolve_branch)"
COMMIT_MSG="$(resolve_commit_msg)"
BUILD_TIME="$(now_time)"

log_info "SSL Hook 构建完成: $ARTIFACT_LOCAL_PATH"
log_info "构建信息: env=$BUILD_ENV_VALUE branch=$BRANCH_NAME sha=$SHORT_SHA time=$BUILD_TIME"
log_info "提交信息: $COMMIT_MSG"
