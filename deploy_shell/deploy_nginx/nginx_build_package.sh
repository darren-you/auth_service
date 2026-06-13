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
      echo "[ERROR] 用法: nginx_build_package.sh [--config <deploy_config.sh绝对路径>]" >&2
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
LOCAL_NGINX_CONF="${LOCAL_NGINX_CONF:-$PROJECT_ROOT/nginx.conf}"

BUILD_ENV_VALUE="${BUILD_ENV:-production}"
validate_build_env "$BUILD_ENV_VALUE"

LOCAL_NGINX_CONF_PATH="${LOCAL_NGINX_CONF:-$PROJECT_ROOT/nginx.conf}"
[[ -f "$LOCAL_NGINX_CONF_PATH" ]] || die "本地 nginx 配置不存在: $LOCAL_NGINX_CONF_PATH"

if is_true "${NGINX_LOCAL_TEST_ENABLED:-false}"; then
  if [[ -n "${NGINX_LOCAL_TEST_CMD:-}" ]]; then
    log_info "执行本地配置校验命令: $NGINX_LOCAL_TEST_CMD"
    (
      cd "$PROJECT_ROOT"
      eval "$NGINX_LOCAL_TEST_CMD"
    )
  else
    require_cmd nginx
    log_info "执行本地配置校验: nginx -t -c $LOCAL_NGINX_CONF_PATH"
    nginx -t -c "$LOCAL_NGINX_CONF_PATH"
  fi
fi

SHORT_SHA="$(resolve_short_sha)"
BRANCH_NAME="$(resolve_branch)"
COMMIT_MSG="$(resolve_commit_msg)"
BUILD_TIME="$(now_time)"

log_info "Nginx 配置构建完成: $LOCAL_NGINX_CONF_PATH"
log_info "构建信息: env=$BUILD_ENV_VALUE branch=$BRANCH_NAME sha=$SHORT_SHA time=$BUILD_TIME"
log_info "提交信息: $COMMIT_MSG"
