#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_CONFIG_FILE="${DEPLOY_CONFIG_FILE:-}"

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
    *)
      early_error "未知参数: $1"
      early_error "用法: docker_build_push.sh [--config <deploy_config.sh绝对路径>]"
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
source "$SCRIPT_DIR/base_image_defaults.sh"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/../shared/secret_base/lib/common.sh"

apply_deploy_server_base_image_defaults

require_cmd docker
require_cmd git
sb_require_secret_base_enabled
sb_require_project_identity

[[ -n "${PROJECT_NAME:-}" ]] || die "deploy_config.sh 缺少 PROJECT_NAME"
[[ -n "${DOCKERFILE_PATH:-}" ]] || die "deploy_config.sh 缺少 DOCKERFILE_PATH"
[[ -n "${DOCKER_BUILD_CONTEXT:-}" ]] || die "deploy_config.sh 缺少 DOCKER_BUILD_CONTEXT"
[[ -n "${BASE_IMAGE_REGISTRY:-}" ]] || die "基础镜像默认配置加载失败：BASE_IMAGE_REGISTRY 为空"
[[ -n "${BUILDER_BASE_IMAGE:-}" ]] || die "基础镜像默认配置加载失败：BUILDER_BASE_IMAGE 为空"
[[ -n "${RUNTIME_BASE_IMAGE:-}" ]] || die "基础镜像默认配置加载失败：RUNTIME_BASE_IMAGE 为空"
[[ -n "${REMOTE_CONTAINER_PORT:-}" ]] || die "deploy_config.sh 缺少 REMOTE_CONTAINER_PORT"
[[ "$REMOTE_CONTAINER_PORT" =~ ^[0-9]+$ ]] || die "REMOTE_CONTAINER_PORT 必须是数字，当前: ${REMOTE_CONTAINER_PORT}"

normalize_nonneg_int() {
  local value="${1:-}"
  local fallback="${2:-0}"
  if [[ "$value" =~ ^[0-9]+$ ]]; then
    printf '%s' "$value"
  else
    printf '%s' "$fallback"
  fi
}

resolve_timeout_cmd() {
  if command -v timeout >/dev/null 2>&1; then
    printf '%s' "timeout"
    return 0
  fi
  if command -v gtimeout >/dev/null 2>&1; then
    printf '%s' "gtimeout"
    return 0
  fi
  printf '%s' ""
}

TIMEOUT_CMD="$(resolve_timeout_cmd)"
TIMEOUT_WARNED="false"

run_with_timeout() {
  local timeout_seconds="${1:-0}"
  shift || true
  if [[ -n "$TIMEOUT_CMD" && "$timeout_seconds" =~ ^[0-9]+$ ]] && (( timeout_seconds > 0 )); then
    "$TIMEOUT_CMD" "$timeout_seconds" "$@"
  else
    if [[ -z "$TIMEOUT_CMD" && "$TIMEOUT_WARNED" != "true" ]]; then
      log_warn "未检测到 timeout/gtimeout，超时控制未生效（建议 brew install coreutils）"
      TIMEOUT_WARNED="true"
    fi
    "$@"
  fi
}

resolve_docker_context_name() {
  local context="${DOCKER_CONTEXT:-}"
  if [[ -n "$context" ]]; then
    printf '%s' "$context"
    return 0
  fi

  docker context show 2>/dev/null || true
}

resolve_docker_endpoint() {
  local context="${1:-}"
  local endpoint=""

  if [[ -n "$context" ]]; then
    endpoint="$(docker context inspect "$context" --format '{{(index .Endpoints "docker").Host}}' 2>/dev/null || true)"
  else
    endpoint="$(docker context inspect --format '{{(index .Endpoints "docker").Host}}' 2>/dev/null || true)"
  fi

  if [[ -z "$endpoint" ]]; then
    endpoint="${DOCKER_HOST:-unix:///var/run/docker.sock}"
  fi

  printf '%s' "$endpoint"
}

is_colima_context_or_endpoint() {
  local context="${1:-}"
  local endpoint="${2:-}"

  if [[ "$context" == "colima" || "$context" == colima-* ]]; then
    return 0
  fi

  [[ "$endpoint" =~ \.colima/[^/]+/docker\.sock$ ]]
}

resolve_colima_profile() {
  local context="${1:-}"
  local endpoint="${2:-}"
  local profile="${DOCKER_COLIMA_PROFILE:-}"

  if [[ -n "$profile" ]]; then
    printf '%s' "$profile"
    return 0
  fi

  if [[ "$endpoint" =~ \.colima/([^/]+)/docker\.sock$ ]]; then
    printf '%s' "${BASH_REMATCH[1]}"
    return 0
  fi

  if [[ "$context" == colima-* ]]; then
    printf '%s' "${context#colima-}"
    return 0
  fi

  if [[ "$context" == "colima" ]]; then
    printf '%s' "default"
    return 0
  fi

  printf '%s' ""
}

try_start_docker_runtime_for_context() {
  local context="${1:-}"
  local endpoint="${2:-}"
  local colima_profile=""

  [[ "$(uname -s)" == "Darwin" ]] || return 0
  is_true "${DOCKER_AUTO_START_ON_DARWIN:-true}" || return 0

  if is_colima_context_or_endpoint "$context" "$endpoint"; then
    if ! command -v colima >/dev/null 2>&1; then
      log_warn "检测到当前 Docker CLI 指向 Colima，但未找到 colima 命令，请先安装并启动对应 profile"
      return 0
    fi

    colima_profile="$(resolve_colima_profile "$context" "$endpoint")"
    if [[ -n "$colima_profile" ]]; then
      log_info "检测到当前 Docker CLI 指向 Colima，尝试启动 profile: ${colima_profile}"
      if ! colima start --profile "$colima_profile" >/dev/null 2>&1; then
        log_warn "自动执行 colima start --profile ${colima_profile} 失败，请手动检查 Colima 状态"
      fi
    else
      log_info "检测到当前 Docker CLI 指向 Colima，尝试启动默认 profile"
      if ! colima start >/dev/null 2>&1; then
        log_warn "自动执行 colima start 失败，请手动检查 Colima 状态"
      fi
    fi
    return 0
  fi

  log_warn "Docker 服务未就绪，当前上下文(${context:-default}) 不是 Colima。本地标准环境要求 Docker CLI 指向 Colima；如在 Jenkins 中运行，请确认对应 Docker daemon 已启动"
}

wait_for_docker_ready() {
  local timeout="${DOCKER_READY_TIMEOUT:-10}"
  local interval="${DOCKER_READY_INTERVAL:-3}"
  local elapsed=0
  local context=""
  local endpoint=""

  if docker info >/dev/null 2>&1; then
    return 0
  fi

  context="$(resolve_docker_context_name)"
  endpoint="$(resolve_docker_endpoint "$context")"
  log_warn "Docker 服务未就绪，开始等待（超时 ${timeout}s，context=${context:-default}，endpoint=${endpoint}）"
  try_start_docker_runtime_for_context "$context" "$endpoint"

  while (( elapsed < timeout )); do
    sleep "$interval"
    elapsed=$((elapsed + interval))
    if docker info >/dev/null 2>&1; then
      log_info "Docker 服务就绪（等待 ${elapsed}s）"
      return 0
    fi
  done

  context="$(resolve_docker_context_name)"
  endpoint="$(resolve_docker_endpoint "$context")"
  die "Docker 服务不可用，当前 context: ${context:-default}，endpoint: ${endpoint}。本地标准环境要求 Docker CLI 指向 Colima；请先执行 colima start 或检查 DOCKER_COLIMA_PROFILE / DOCKER_CONTEXT / DOCKER_HOST 配置；如在 Jenkins 中运行，请确认对应 Docker daemon 可用"
}

wait_for_docker_ready

BRANCH_NAME="$(resolve_branch)"
BRANCH_SAFE="$(sanitize_tag_part "$BRANCH_NAME")"
SHORT_SHA="$(resolve_short_sha)"
COMMIT_MSG="$(resolve_commit_msg)"
BUILD_ENV_VALUE_RAW="${BuildEnv:-test}"
BUILD_ENV_DISPLAY="$(normalize_build_env_input "$BUILD_ENV_VALUE_RAW")"
BUILD_ENV_VALUE="$(to_internal_build_env "$BUILD_ENV_DISPLAY")"
validate_build_env "$BUILD_ENV_VALUE"
ENV_SHORT="$(env_short "$BUILD_ENV_VALUE")"
BUILD_TIME="$(now_time)"
NEXUS_PUSH_REGISTRY_EXPLICIT="${NEXUS_PUSH_REGISTRY:-${NEXUS_REGISTRY_PUSH_HOST:-}}"
DEFAULT_NEXUS_PUSH_REGISTRY="$NEXUS_DOCKER_HOSTED_REGISTRY_DEFAULT"
if is_jenkins_build_context; then
  DEFAULT_NEXUS_PUSH_REGISTRY="$NEXUS_DOCKER_HOSTED_LOCAL_REGISTRY_DEFAULT"
fi
DEFAULT_NEXUS_PULL_REGISTRY="$NEXUS_DOCKER_HOSTED_REGISTRY_DEFAULT"
NEXUS_PUSH_REGISTRY="$(normalize_registry_host "${NEXUS_PUSH_REGISTRY:-${NEXUS_REGISTRY_PUSH_HOST:-$DEFAULT_NEXUS_PUSH_REGISTRY}}")"
NEXUS_PULL_REGISTRY="$(normalize_registry_host "${NEXUS_PULL_REGISTRY:-${NEXUS_REGISTRY_PULL_HOST:-$DEFAULT_NEXUS_PULL_REGISTRY}}")"
NEXUS_IMAGE_NAMESPACE="$(trim_space_edges "${NEXUS_IMAGE_NAMESPACE:-$NEXUS_DOCKER_NAMESPACE_DEFAULT}")"
NEXUS_DOCKER_USERNAME="${NEXUS_DOCKER_USERNAME:-$NEXUS_DOCKER_USERNAME_DEFAULT}"
NEXUS_DOCKER_PASSWORD_FILE="${NEXUS_DOCKER_PASSWORD_FILE:-}"
NEXUS_DOCKER_PASSWORD="$(resolve_secret_value "${NEXUS_DOCKER_PASSWORD:-}" "$NEXUS_DOCKER_PASSWORD_FILE" "Nexus Docker 密码")"
if [[ -z "$NEXUS_DOCKER_PASSWORD" ]]; then
  NEXUS_DOCKER_PASSWORD="$NEXUS_DOCKER_PASSWORD_DEFAULT"
fi
NEXUS_PUSH_RETRY_COUNT="$(normalize_nonneg_int "${NEXUS_PUSH_RETRY_COUNT:-2}" 2)"
NEXUS_PUSH_RETRY_SLEEP_SECONDS="$(normalize_nonneg_int "${NEXUS_PUSH_RETRY_SLEEP_SECONDS:-2}" 2)"
[[ -n "$NEXUS_PUSH_REGISTRY" ]] || die "NEXUS_PUSH_REGISTRY 不能为空"
[[ -n "$NEXUS_PULL_REGISTRY" ]] || die "NEXUS_PULL_REGISTRY 不能为空"
[[ -n "$NEXUS_DOCKER_USERNAME" ]] || die "NEXUS_DOCKER_USERNAME 不能为空"
[[ -n "$NEXUS_DOCKER_PASSWORD" ]] || die "Nexus Docker 密码不能为空"
if (( NEXUS_PUSH_RETRY_COUNT < 1 )); then
  NEXUS_PUSH_RETRY_COUNT=1
fi
BASE_REGISTRY_VALUE="${BASE_IMAGE_REGISTRY}"
BASE_REGISTRY_VALUE="${BASE_REGISTRY_VALUE#http://}"
BASE_REGISTRY_VALUE="${BASE_REGISTRY_VALUE#https://}"
BASE_REGISTRY_VALUE="${BASE_REGISTRY_VALUE%/}"
BASE_CANDIDATES_RAW="${BASE_IMAGE_REGISTRY_CANDIDATES:-$BASE_REGISTRY_VALUE}"
REGISTRY_PRECHECK_ENABLED="${REGISTRY_PRECHECK_ENABLED:-true}"
REGISTRY_PULL_TIMEOUT_SECONDS="$(normalize_nonneg_int "${REGISTRY_PULL_TIMEOUT_SECONDS:-20}" 20)"
REGISTRY_PULL_RETRY_COUNT="$(normalize_nonneg_int "${REGISTRY_PULL_RETRY_COUNT:-2}" 2)"
REGISTRY_BUILD_TIMEOUT_SECONDS="$(normalize_nonneg_int "${REGISTRY_BUILD_TIMEOUT_SECONDS:-0}" 0)"
REGISTRY_BUILD_RETRY_COUNT="$(normalize_nonneg_int "${REGISTRY_BUILD_RETRY_COUNT:-1}" 1)"
REGISTRY_RETRY_SLEEP_SECONDS="$(normalize_nonneg_int "${REGISTRY_RETRY_SLEEP_SECONDS:-2}" 2)"
GO_MODULE_PROXY="${GO_MODULE_PROXY:-$GO_MODULE_PROXY_DEFAULT}"
GO_SUM_DB="${GO_SUM_DB:-$GO_SUM_DB_DEFAULT}"
if (( REGISTRY_PULL_RETRY_COUNT < 1 )); then
  REGISTRY_PULL_RETRY_COUNT=1
fi
if (( REGISTRY_BUILD_RETRY_COUNT < 1 )); then
  REGISTRY_BUILD_RETRY_COUNT=1
fi

extract_server_port_from_yaml() {
  local yaml_file="$1"
  awk '
    /^server:[[:space:]]*$/ {
      in_server = 1
      next
    }
    in_server && /^[^[:space:]]/ {
      in_server = 0
    }
    in_server && /^[[:space:]]+port:[[:space:]]*/ {
      value = $0
      sub(/^[[:space:]]+port:[[:space:]]*/, "", value)
      sub(/[[:space:]]+#.*$/, "", value)
      print value
      exit
    }
  ' "$yaml_file"
}

TEMPLATE_YAML_SOURCE="$(sb_template_path "$BUILD_ENV_VALUE")"
[[ -f "$TEMPLATE_YAML_SOURCE" ]] || die "找不到模板 YAML 配置源: $TEMPLATE_YAML_SOURCE"
RUNTIME_SERVER_PORT="$(extract_server_port_from_yaml "$TEMPLATE_YAML_SOURCE")"
[[ "$RUNTIME_SERVER_PORT" =~ ^[0-9]+$ ]] || die "无法从 $TEMPLATE_YAML_SOURCE 解析 server.port"
if [[ "$RUNTIME_SERVER_PORT" != "$REMOTE_CONTAINER_PORT" ]]; then
  die "模板 YAML 中的 server.port($RUNTIME_SERVER_PORT) 与 deploy_config.sh 中的 REMOTE_CONTAINER_PORT($REMOTE_CONTAINER_PORT) 不一致"
fi

IMAGE_REPO="$LOCAL_IMAGE_REPO"
validate_project_name "${PROJECT_NAME:-}"
IMAGE_NAME_SLUG="$(sanitize_tag_part "$PROJECT_NAME")"
NEXUS_PUSH_IMAGE_REPO="$(compose_registry_image_repo "$NEXUS_PUSH_REGISTRY" "$NEXUS_IMAGE_NAMESPACE" "$IMAGE_NAME_SLUG")"
NEXUS_PULL_IMAGE_REPO="$(compose_registry_image_repo "$NEXUS_PULL_REGISTRY" "$NEXUS_IMAGE_NAMESPACE" "$IMAGE_NAME_SLUG")"
if [[ -n "${IMAGE_TAG:-}" ]]; then
  PRIMARY_TAG="$IMAGE_TAG"
else
  PRIMARY_TAG="${ENV_SHORT}-$(date +%Y%m%d-%H%M%S)-${SHORT_SHA}"
fi
SHA_TAG="${BRANCH_SAFE}-${SHORT_SHA}"
ENV_LATEST_TAG="${IMAGE_NAME_SLUG}_${ENV_SHORT}-latest"

declare -a TAGS=()
add_tag() {
  local tag="$1"
  local existing
  for existing in "${TAGS[@]-}"; do
    [[ -z "$existing" ]] && continue
    [[ "$existing" == "$tag" ]] && return 0
  done
  TAGS+=("$tag")
}

add_tag "$PRIMARY_TAG"
add_tag "$SHA_TAG"

if is_true "$PUSH_LATEST"; then
  add_tag "$ENV_LATEST_TAG"
  if [[ "$BUILD_ENV_VALUE" == "prod" ]]; then
    add_tag "latest"
  fi
fi

log_info "项目: $PROJECT_NAME"
log_info "分支: $BRANCH_NAME"
log_info "构建环境: ${BUILD_ENV_VALUE}"
if is_jenkins_build_context && [[ -z "$NEXUS_PUSH_REGISTRY_EXPLICIT" ]]; then
  log_info "检测到 Jenkins 构建环境，Nexus 推送优先走本地 connector: $DEFAULT_NEXUS_PUSH_REGISTRY"
fi
log_info "模板 YAML 配置源: $TEMPLATE_YAML_SOURCE"
log_info "secret bundle 目标路径: $(sb_bundle_path "$BUILD_ENV_VALUE")"
log_info "本地镜像仓库: $IMAGE_REPO"
log_info "Nexus 推送仓库: $NEXUS_PUSH_IMAGE_REPO"
log_info "Nexus 拉取仓库: $NEXUS_PULL_IMAGE_REPO"
log_info "主标签: $PRIMARY_TAG"
log_info "构建基础镜像: ${BASE_REGISTRY_VALUE}/${BUILDER_BASE_IMAGE}"
log_info "运行时基础镜像: ${BASE_REGISTRY_VALUE}/${RUNTIME_BASE_IMAGE}"
log_info "基础镜像候选源: $BASE_CANDIDATES_RAW"
log_info "Go 模块代理: $GO_MODULE_PROXY"
log_info "Go 校验和数据库: $GO_SUM_DB"
log_info "镜像源预检: ${REGISTRY_PRECHECK_ENABLED}，拉取超时: ${REGISTRY_PULL_TIMEOUT_SECONDS}s，拉取重试: ${REGISTRY_PULL_RETRY_COUNT} 次，构建超时: ${REGISTRY_BUILD_TIMEOUT_SECONDS}s，构建重试: ${REGISTRY_BUILD_RETRY_COUNT} 次"

normalize_registry() {
  local r="$1"
  r="$(printf '%s' "$r" | xargs)"
  r="${r#http://}"
  r="${r#https://}"
  r="${r%/}"
  printf '%s' "$r"
}

declare -a BUILD_REGISTRIES=()
add_registry() {
  local registry="$1"
  local existing
  [[ -z "$registry" ]] && return 0
  for existing in "${BUILD_REGISTRIES[@]-}"; do
    [[ "$existing" == "$registry" ]] && return 0
  done
  BUILD_REGISTRIES+=("$registry")
}

IFS=',' read -r -a RAW_REGISTRIES <<<"$BASE_CANDIDATES_RAW"
for raw_registry in "${RAW_REGISTRIES[@]-}"; do
  raw_registry="$(normalize_registry "$raw_registry")"
  add_registry "$raw_registry"
done
add_registry "$BASE_REGISTRY_VALUE"
add_registry "docker.io"

docker_pull_with_retry() {
  local image_ref="$1"
  local attempt=1
  local pull_log
  pull_log="$(mktemp "/tmp/${PROJECT_NAME}.docker_pull.XXXXXX")"
  while (( attempt <= REGISTRY_PULL_RETRY_COUNT )); do
    if run_with_timeout "$REGISTRY_PULL_TIMEOUT_SECONDS" docker pull --platform "linux/amd64" "$image_ref" >"$pull_log" 2>&1; then
      rm -f "$pull_log"
      return 0
    fi
    local rc=$?
    local failure_reason=""
    if [[ -s "$pull_log" ]]; then
      failure_reason="$(tail -n 5 "$pull_log" | tr '\n' ' ' | sed 's/[[:space:]]\+/ /g; s/^ //; s/ $//')"
    fi
    if [[ "$rc" -eq 124 ]]; then
      if [[ -n "$failure_reason" ]]; then
        log_warn "拉取超时(${REGISTRY_PULL_TIMEOUT_SECONDS}s): ${image_ref}（${attempt}/${REGISTRY_PULL_RETRY_COUNT}），原因: ${failure_reason}"
      else
        log_warn "拉取超时(${REGISTRY_PULL_TIMEOUT_SECONDS}s): ${image_ref}（${attempt}/${REGISTRY_PULL_RETRY_COUNT}）"
      fi
    else
      if [[ -n "$failure_reason" ]]; then
        log_warn "拉取失败: ${image_ref}（${attempt}/${REGISTRY_PULL_RETRY_COUNT}），原因: ${failure_reason}"
      else
        log_warn "拉取失败: ${image_ref}（${attempt}/${REGISTRY_PULL_RETRY_COUNT}）"
      fi
    fi
    if (( attempt < REGISTRY_PULL_RETRY_COUNT )) && (( REGISTRY_RETRY_SLEEP_SECONDS > 0 )); then
      sleep "$REGISTRY_RETRY_SLEEP_SECONDS"
    fi
    attempt=$((attempt + 1))
  done
  rm -f "$pull_log"
  return 1
}

registry_precheck() {
  local registry="$1"
  local golang_ref="${registry}/${BUILDER_BASE_IMAGE}"
  local alpine_ref="${registry}/${RUNTIME_BASE_IMAGE}"
  log_info "预检基础镜像可达性: $registry"
  docker_pull_with_retry "$golang_ref" && docker_pull_with_retry "$alpine_ref"
}

docker_build_once() {
  local registry="$1"
  local remote_container_port="${REMOTE_CONTAINER_PORT}"
  run_with_timeout "$REGISTRY_BUILD_TIMEOUT_SECONDS" docker build \
    --platform "linux/amd64" \
    --build-arg "BUILD_ENV=$BUILD_ENV_VALUE" \
    --build-arg "PROJECT_NAME=$PROJECT_NAME" \
    --build-arg "REMOTE_CONTAINER_PORT=$remote_container_port" \
    --build-arg "GO_MODULE_PROXY=$GO_MODULE_PROXY" \
    --build-arg "GO_SUM_DB=$GO_SUM_DB" \
    --build-arg "BASE_IMAGE_REGISTRY=$registry" \
    --build-arg "BUILDER_BASE_IMAGE=$BUILDER_BASE_IMAGE" \
    --build-arg "RUNTIME_BASE_IMAGE=$RUNTIME_BASE_IMAGE" \
    -f "$DOCKERFILE_PATH" \
    -t "${IMAGE_REPO}:${PRIMARY_TAG}" \
    "$DOCKER_BUILD_CONTEXT"
}

docker_build_with_retry() {
  local registry="$1"
  local attempt=1
  while (( attempt <= REGISTRY_BUILD_RETRY_COUNT )); do
    if docker_build_once "$registry"; then
      return 0
    fi
    local rc=$?
    if [[ "$rc" -eq 124 ]]; then
      log_warn "构建超时(${REGISTRY_BUILD_TIMEOUT_SECONDS}s): ${registry}（${attempt}/${REGISTRY_BUILD_RETRY_COUNT}）"
    else
      log_warn "构建失败: ${registry}（${attempt}/${REGISTRY_BUILD_RETRY_COUNT}）"
    fi
    if (( attempt < REGISTRY_BUILD_RETRY_COUNT )) && (( REGISTRY_RETRY_SLEEP_SECONDS > 0 )); then
      sleep "$REGISTRY_RETRY_SLEEP_SECONDS"
    fi
    attempt=$((attempt + 1))
  done
  return 1
}

docker_login_nexus() {
  printf '%s' "$NEXUS_DOCKER_PASSWORD" | docker login "$NEXUS_PUSH_REGISTRY" -u "$NEXUS_DOCKER_USERNAME" --password-stdin >/dev/null
}

docker_push_with_retry() {
  local image_ref="$1"
  local attempt=1
  local push_log=""
  local rc=0
  local failure_reason=""

  push_log="$(mktemp "/tmp/${PROJECT_NAME}.docker_push.XXXXXX")"
  while (( attempt <= NEXUS_PUSH_RETRY_COUNT )); do
    if docker push "$image_ref" >"$push_log" 2>&1; then
      rm -f "$push_log"
      return 0
    fi

    rc=$?
    failure_reason=""
    if [[ -s "$push_log" ]]; then
      failure_reason="$(tail -n 8 "$push_log" | tr '\n' ' ' | sed 's/[[:space:]]\+/ /g; s/^ //; s/ $//')"
    fi

    if [[ -n "$failure_reason" ]]; then
      log_warn "推送失败: ${image_ref}（${attempt}/${NEXUS_PUSH_RETRY_COUNT}），原因: ${failure_reason}"
    else
      log_warn "推送失败: ${image_ref}（${attempt}/${NEXUS_PUSH_RETRY_COUNT}）"
    fi

    if (( attempt < NEXUS_PUSH_RETRY_COUNT )) && (( NEXUS_PUSH_RETRY_SLEEP_SECONDS > 0 )); then
      sleep "$NEXUS_PUSH_RETRY_SLEEP_SECONDS"
    fi
    attempt=$((attempt + 1))
  done

  rm -f "$push_log"
  return 1
}

BUILD_OK="false"
BUILD_REGISTRY_USED=""
declare -a PRECHECK_OK_REGISTRIES=()
declare -a PRECHECK_FAILED_REGISTRIES=()
declare -a PRECHECK_DEFERRED_REGISTRIES=()
PREFERRED_PRECHECK_FOUND="false"

for registry in "${BUILD_REGISTRIES[@]-}"; do
  [[ -z "$registry" ]] && continue
  if is_true "$REGISTRY_PRECHECK_ENABLED"; then
    if [[ "$PREFERRED_PRECHECK_FOUND" == "true" ]]; then
      PRECHECK_DEFERRED_REGISTRIES+=("$registry")
      continue
    fi
    if registry_precheck "$registry"; then
      PRECHECK_OK_REGISTRIES+=("$registry")
      PREFERRED_PRECHECK_FOUND="true"
      log_info "已锁定首个可用基础镜像源: $registry"
    else
      log_warn "镜像源预检失败，保留为兜底构建候选: $registry"
      PRECHECK_FAILED_REGISTRIES+=("$registry")
    fi
  else
    PRECHECK_OK_REGISTRIES+=("$registry")
  fi
done

if is_true "$REGISTRY_PRECHECK_ENABLED" && (( ${#PRECHECK_OK_REGISTRIES[@]} == 0 )) && (( ${#PRECHECK_FAILED_REGISTRIES[@]} > 0 )); then
  log_warn "所有镜像源预检均失败，继续尝试实际构建以避免误判"
fi

for registry in "${PRECHECK_OK_REGISTRIES[@]-}"; do
  [[ -z "$registry" ]] && continue
  log_info "尝试基础镜像源构建: $registry"
  if docker_build_with_retry "$registry"; then
    BUILD_OK="true"
    BUILD_REGISTRY_USED="$registry"
    break
  fi
done

if [[ "$BUILD_OK" != "true" ]]; then
  for registry in "${PRECHECK_DEFERRED_REGISTRIES[@]-}"; do
    [[ -z "$registry" ]] && continue
    log_warn "首选镜像源构建失败，尝试后续候选镜像源: $registry"
    if is_true "$REGISTRY_PRECHECK_ENABLED" && ! registry_precheck "$registry"; then
      log_warn "后续候选镜像源预检失败，保留为最终兜底构建候选: $registry"
      PRECHECK_FAILED_REGISTRIES+=("$registry")
      continue
    fi
    if docker_build_with_retry "$registry"; then
      BUILD_OK="true"
      BUILD_REGISTRY_USED="$registry"
      break
    fi
  done
fi

if [[ "$BUILD_OK" != "true" ]]; then
  for registry in "${PRECHECK_FAILED_REGISTRIES[@]-}"; do
    [[ -z "$registry" ]] && continue
    log_warn "尝试预检失败的兜底镜像源构建: $registry"
    if docker_build_with_retry "$registry"; then
      BUILD_OK="true"
      BUILD_REGISTRY_USED="$registry"
      break
    fi
  done
fi

if [[ "$BUILD_OK" != "true" ]]; then
  die "所有基础镜像源构建均失败，请检查网络或更换 BASE_IMAGE_REGISTRY_CANDIDATES"
fi

log_info "构建成功，使用基础镜像源: $BUILD_REGISTRY_USED"

for tag in "${TAGS[@]-}"; do
  [[ -z "$tag" ]] && continue
  if [[ "$tag" != "$PRIMARY_TAG" ]]; then
    docker tag "${IMAGE_REPO}:${PRIMARY_TAG}" "${IMAGE_REPO}:${tag}"
  fi
done

for tag in "${TAGS[@]-}"; do
  [[ -z "$tag" ]] && continue
  log_info "本地镜像已就绪: ${IMAGE_REPO}:${tag}"
done

log_info "开始登录 Nexus Docker 私仓: $NEXUS_PUSH_REGISTRY"
docker_login_nexus
log_info "Nexus Docker 登录成功"

for tag in "${TAGS[@]-}"; do
  local_ref="${IMAGE_REPO}:${tag}"
  remote_ref="${NEXUS_PUSH_IMAGE_REPO}:${tag}"
  log_info "推送镜像到 Nexus: $remote_ref"
  docker tag "$local_ref" "$remote_ref"
  if ! docker_push_with_retry "$remote_ref"; then
    die "推送镜像到 Nexus 失败: $remote_ref"
  fi
done

if is_true "$PUSH_LATEST"; then
  DEPLOY_TAG="$ENV_LATEST_TAG"
else
  DEPLOY_TAG="$PRIMARY_TAG"
fi

DEPLOY_IMAGE="${NEXUS_PULL_IMAGE_REPO}:${DEPLOY_TAG}"
if [[ -n "${DEPLOY_IMAGE_REF_FILE:-}" ]]; then
  printf '%s\n' "$DEPLOY_IMAGE" > "$DEPLOY_IMAGE_REF_FILE"
  log_info "已写入部署镜像引用文件: $DEPLOY_IMAGE_REF_FILE"
fi

log_info "构建并推送完成，部署镜像: $DEPLOY_IMAGE"
