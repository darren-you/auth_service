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
      echo "[ERROR] 用法: npm_build_package.sh --config <deploy_config.sh绝对路径>" >&2
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
[[ -n "${WEB_NAME:-}" ]] || die "WEB_NAME 不能为空，请在 deploy_config.sh 中定义（用于浏览器 Tab 网站名）"
is_placeholder "$WEB_NAME" && die "WEB_NAME 不能是占位值，请在 deploy_config.sh 中填写真实名称"

if [[ -n "${VITE_WEB_NAME:-}" && "$VITE_WEB_NAME" != "$WEB_NAME" ]]; then
  die "VITE_WEB_NAME 与 deploy_config.sh 的 WEB_NAME 不一致。网站名称仅允许来自 WEB_NAME。"
fi

cleanup_old_web_artifacts() {
  local web_dir="${WEB_DIR:-$PROJECT_ROOT}"
  local dist_dir="${DIST_DIR:-${web_dir%/}/dist}"
  local dist_zip_path="${DIST_ZIP_PATH:-${web_dir%/}/dist.zip}"

  [[ -n "$web_dir" ]] || die "WEB_DIR 为空，无法执行构建前清理"
  [[ -d "$web_dir" ]] || die "WEB_DIR 不存在: $web_dir"
  web_dir="$(cd -- "$web_dir" && pwd)"

  if [[ "$dist_dir" != /* ]]; then
    dist_dir="${web_dir%/}/${dist_dir#./}"
  fi
  if [[ "$dist_zip_path" != /* ]]; then
    dist_zip_path="${web_dir%/}/${dist_zip_path#./}"
  fi

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
  fi
  if [[ -e "$dist_zip_path" ]]; then
    log_info "清理旧打包文件: $dist_zip_path"
    rm -f "$dist_zip_path"
  fi
}

META_FILE="${META_FILE:-}"

if [[ -z "$META_FILE" ]]; then
  META_FILE="$(mktemp "/tmp/${PROJECT_NAME}.build.XXXXXX")"
fi
export META_FILE
ensure_meta_file

require_cmd npm
require_cmd git
require_cmd zip
SHORT_SHA="$(resolve_short_sha)"
BRANCH_NAME="$(resolve_branch)"
COMMIT_MSG="$(resolve_commit_msg)"
BUILD_TIME="$(now_time)"

log_info "项目: $PROJECT_NAME"
log_info "分支: $BRANCH_NAME"
VITE_WEB_NAME="$WEB_NAME"
export VITE_WEB_NAME
unset VITE_SITE_NAME
log_info "注入网站名称: VITE_WEB_NAME=$VITE_WEB_NAME (来源: deploy_config.sh/WEB_NAME)"

cleanup_old_web_artifacts

log_info "开始安装依赖: $NPM_INSTALL_CMD"
(
  cd "$WEB_DIR"
  eval "$NPM_INSTALL_CMD"
)

log_info "开始构建: $NPM_BUILD_CMD"
(
  cd "$WEB_DIR"
  eval "$NPM_BUILD_CMD"
)

[[ -d "$DIST_DIR" ]] || die "构建失败：未找到产物目录 $DIST_DIR"

log_info "打包 dist 目录: $DIST_ZIP_PATH"
rm -f "$DIST_ZIP_PATH"
(
  cd "$WEB_DIR"
  zip -qry "$DIST_ZIP_PATH" dist
)
[[ -f "$DIST_ZIP_PATH" ]] || die "打包失败：未生成 dist.zip"

write_meta PROJECT_NAME "$PROJECT_NAME"
write_meta BUILD_BRANCH "$BRANCH_NAME"
write_meta GIT_SHA "$SHORT_SHA"
write_meta COMMIT_MSG "$COMMIT_MSG"
write_meta BUILD_TIME "$BUILD_TIME"
write_meta ARTIFACT_LOCAL_PATH "$DIST_ZIP_PATH"
write_meta ARTIFACT_NAME "$(basename "$DIST_ZIP_PATH")"
write_meta REMOTE_PROJECT_DIR "$REMOTE_DEPLOY_PROJECT_DIR"

log_info "构建并打包完成: $DIST_ZIP_PATH"
log_info "元数据文件: $META_FILE"
