#!/usr/bin/env bash
# shellcheck shell=bash

apply_deploy_profile_defaults() {
  local profile_name="${1:-}"
  local deploy_kind="${2:-}"
  local script_dir=""
  local profile_file=""

  if [[ -z "${PROJECT_ROOT:-}" ]]; then
    echo "[ERROR] PROJECT_ROOT 不能为空，请先在 deploy_config.sh 中定义 PROJECT_ROOT" >&2
    return 1
  fi

  if [[ -z "${PROJECT_NAME:-}" ]]; then
    echo "[ERROR] PROJECT_NAME 不能为空，请先在 deploy_config.sh 中定义 PROJECT_NAME" >&2
    return 1
  fi

  if [[ -z "$profile_name" ]]; then
    echo "[ERROR] DEPLOY_SERVER_PROFILE 不能为空，请先在 deploy_config.sh 中定义部署服务器 profile" >&2
    return 1
  fi

  case "$deploy_kind" in
    web|server|nginx) ;;
    *)
      echo "[ERROR] 未知部署类型: ${deploy_kind:-<empty>}，仅支持 web/server/nginx" >&2
      return 1
      ;;
  esac

  script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
  profile_file="$script_dir/server_profiles/${profile_name}.sh"
  if [[ ! -f "$profile_file" ]]; then
    echo "[ERROR] 找不到部署服务器 profile: $profile_file" >&2
    return 1
  fi

  # shellcheck disable=SC1090
  source "$profile_file"
  if ! declare -F apply_server_profile_defaults >/dev/null 2>&1; then
    echo "[ERROR] 部署服务器 profile 缺少 apply_server_profile_defaults 函数: $profile_file" >&2
    return 1
  fi

  case "$deploy_kind" in
    web)
      WEB_DIR="${WEB_DIR:-$PROJECT_ROOT}"
      SERVER_DIR="${SERVER_DIR:-$WEB_DIR}"
      NPM_INSTALL_CMD="${NPM_INSTALL_CMD:-npm ci}"
      NPM_BUILD_CMD="${NPM_BUILD_CMD:-npm run build}"
      DIST_DIR="${DIST_DIR:-$WEB_DIR/dist}"
      DIST_ZIP_PATH="${DIST_ZIP_PATH:-$WEB_DIR/dist.zip}"
      ;;
    server)
      SERVER_DIR="${SERVER_DIR:-$PROJECT_ROOT}"
      DOCKER_REGISTRY="${DOCKER_REGISTRY:-docker.io}"
      DOCKERFILE_PATH="${DOCKERFILE_PATH:-$SERVER_DIR/Dockerfile}"
      DOCKER_BUILD_CONTEXT="${DOCKER_BUILD_CONTEXT:-$SERVER_DIR}"
      LOCAL_IMAGE_REPO="$PROJECT_NAME"
      IMAGE_NAME_SLUG="$PROJECT_NAME"
      PROD_CONTAINER_NAME="${PROJECT_NAME}_prod"
      TEST_CONTAINER_NAME="${PROJECT_NAME}_test"
      ;;
    nginx)
      SERVER_DIR="${SERVER_DIR:-$(cd -- "$PROJECT_ROOT/.." && pwd)}"
      BUILD_ENV="${BUILD_ENV:-production}"
      ;;
  esac

  apply_server_profile_defaults "$deploy_kind"
  unset -f apply_server_profile_defaults
}
