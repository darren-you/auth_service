#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_CONFIG_FILE=""

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
      early_error "用法: remote_deploy_core.sh --config <deploy_config.sh绝对路径>"
      exit 1
      ;;
  esac
done

if [[ -z "$DEPLOY_CONFIG_FILE" ]]; then
  early_error "必须传入 --config <deploy_config.sh绝对路径>"
  exit 1
fi

if [[ ! -f "$DEPLOY_CONFIG_FILE" ]]; then
  early_error "找不到 deploy_config.sh: $DEPLOY_CONFIG_FILE"
  exit 1
fi

PROJECT_ROOT="$(cd -- "$(dirname -- "$DEPLOY_CONFIG_FILE")" && pwd)"
# shellcheck disable=SC1090
source "$DEPLOY_CONFIG_FILE"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/common.sh"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/../shared/secret_base/lib/common.sh"
# shellcheck disable=SC1091
source "$SCRIPT_DIR/../shared/secret_base/lib/remote.sh"
require_cmd ssh
require_cmd scp
sb_require_secret_base_enabled
sb_require_project_identity

normalize_nonneg_int() {
  local value="${1:-}"
  local fallback="${2:-0}"
  if [[ "$value" =~ ^[0-9]+$ ]]; then
    printf '%s' "$value"
  else
    printf '%s' "$fallback"
  fi
}

BUILD_ENV_VALUE_RAW="${BuildEnv:-test}"
BUILD_ENV_DISPLAY="$(normalize_build_env_input "$BUILD_ENV_VALUE_RAW")"
BUILD_ENV_VALUE="$(to_internal_build_env "$BUILD_ENV_DISPLAY")"
validate_build_env "$BUILD_ENV_VALUE"
ENV_SHORT="$(env_short "$BUILD_ENV_VALUE")"
SECRET_BASE_ENV="$BUILD_ENV_VALUE"

validate_project_name "${PROJECT_NAME:-}"
IMAGE_NAME_SLUG="$(sanitize_tag_part "$PROJECT_NAME")"

DEFAULT_NEXUS_PULL_REGISTRY="$NEXUS_DOCKER_HOSTED_REGISTRY_DEFAULT"
NEXUS_PULL_REGISTRY="$(normalize_registry_host "${NEXUS_PULL_REGISTRY:-${NEXUS_REGISTRY_PULL_HOST:-$DEFAULT_NEXUS_PULL_REGISTRY}}")"
NEXUS_IMAGE_NAMESPACE="$(trim_space_edges "${NEXUS_IMAGE_NAMESPACE:-$NEXUS_DOCKER_NAMESPACE_DEFAULT}")"
NEXUS_DOCKER_USERNAME="${NEXUS_DOCKER_USERNAME:-$NEXUS_DOCKER_USERNAME_DEFAULT}"
NEXUS_DOCKER_PASSWORD_FILE="${NEXUS_DOCKER_PASSWORD_FILE:-}"
NEXUS_DOCKER_PASSWORD="$(resolve_secret_value "${NEXUS_DOCKER_PASSWORD:-}" "$NEXUS_DOCKER_PASSWORD_FILE" "Nexus Docker 密码")"
if [[ -z "$NEXUS_DOCKER_PASSWORD" ]]; then
  NEXUS_DOCKER_PASSWORD="$NEXUS_DOCKER_PASSWORD_DEFAULT"
fi
REMOTE_DOCKER_LOGIN_TIMEOUT_SECONDS="$(normalize_nonneg_int "${REMOTE_DOCKER_LOGIN_TIMEOUT_SECONDS:-30}" 30)"
REMOTE_DOCKER_PULL_TIMEOUT_SECONDS="$(normalize_nonneg_int "${REMOTE_DOCKER_PULL_TIMEOUT_SECONDS:-300}" 300)"
REMOTE_DOCKER_PULL_RETRY_COUNT="$(normalize_nonneg_int "${REMOTE_DOCKER_PULL_RETRY_COUNT:-2}" 2)"
REMOTE_DOCKER_PULL_RETRY_SLEEP_SECONDS="$(normalize_nonneg_int "${REMOTE_DOCKER_PULL_RETRY_SLEEP_SECONDS:-2}" 2)"
REMOTE_CONTAINER_STABILITY_SECONDS="$(normalize_nonneg_int "${REMOTE_CONTAINER_STABILITY_SECONDS:-5}" 5)"
if (( REMOTE_DOCKER_PULL_RETRY_COUNT < 1 )); then
  REMOTE_DOCKER_PULL_RETRY_COUNT=1
fi

[[ -n "$NEXUS_PULL_REGISTRY" ]] || die "NEXUS_PULL_REGISTRY 不能为空"
[[ -n "$NEXUS_DOCKER_USERNAME" ]] || die "NEXUS_DOCKER_USERNAME 不能为空"
[[ -n "$NEXUS_DOCKER_PASSWORD" ]] || die "Nexus Docker 密码不能为空"

resolve_default_deploy_image() {
  local image_repo=""

  image_repo="$(compose_registry_image_repo "$NEXUS_PULL_REGISTRY" "$NEXUS_IMAGE_NAMESPACE" "$IMAGE_NAME_SLUG")"
  if is_true "${PUSH_LATEST:-false}"; then
    printf '%s:%s_%s-latest' "$image_repo" "$IMAGE_NAME_SLUG" "$ENV_SHORT"
    return 0
  fi

  die "DEPLOY_IMAGE 未设置，且 PUSH_LATEST=false 时无法自动推断。请先执行 docker_build_push.sh 生成 DEPLOY_IMAGE_REF_FILE，或显式传入 DEPLOY_IMAGE。"
}

resolve_deploy_image() {
  local image_ref=""

  if [[ -n "${DEPLOY_IMAGE:-}" ]]; then
    printf '%s' "$DEPLOY_IMAGE"
    return 0
  fi

  if [[ -n "${DEPLOY_IMAGE_REF_FILE:-}" && -s "$DEPLOY_IMAGE_REF_FILE" ]]; then
    image_ref="$(tail -n 1 "$DEPLOY_IMAGE_REF_FILE" | tr -d '\r\n')"
    if [[ -n "$image_ref" ]]; then
      printf '%s' "$image_ref"
      return 0
    fi
  fi

  resolve_default_deploy_image
}

resolve_registry_host_from_image() {
  local image_ref="${1:-}"

  if [[ "$image_ref" == */* ]]; then
    printf '%s' "${image_ref%%/*}"
    return 0
  fi

  die "部署镜像缺少 Registry 前缀，无法远端拉取: ${image_ref:-<empty>}"
}

DEPLOY_IMAGE_FULL="$(resolve_deploy_image)"
PULL_REGISTRY_HOST="$(resolve_registry_host_from_image "$DEPLOY_IMAGE_FULL")"

if [[ "$BUILD_ENV_VALUE" == "prod" ]]; then
  CONTAINER_NAME="$PROD_CONTAINER_NAME"
  HOST_PORT="$PROD_HOST_PORT"
  CONTAINER_IP="$PROD_CONTAINER_IP"
  LOG_PATH="$PROD_LOG_PATH"
else
  CONTAINER_NAME="$TEST_CONTAINER_NAME"
  HOST_PORT="$TEST_HOST_PORT"
  CONTAINER_IP="$TEST_CONTAINER_IP"
  LOG_PATH="$TEST_LOG_PATH"
fi
UPLOAD_FONTS_HOST_PATH="${UPLOAD_FONTS_DIR:-}"
GEN_FONTS_HOST_PATH="${GEN_FONTS_DIR:-}"
SECRET_BASE_TEMPLATE_PATH="$(sb_template_path "$SECRET_BASE_ENV")"
SECRET_BASE_ROOT_PATH="$(sb_resolve_root)"
SECRET_BASE_BUNDLE_FILE="$(sb_bundle_path "$SECRET_BASE_ENV")"
SECRET_BASE_RUNTIME_CONFIG="$(sb_runtime_config_path "$SECRET_BASE_ENV")"
SECRET_BASE_IDENTITY_FILE="$(sb_age_identity_path)"
SECRET_BASE_AUDIT_LOG_FILE="$(sb_audit_log_path)"

[[ -f "$SECRET_BASE_TEMPLATE_PATH" ]] || sb_die "模板 YAML 不存在: $SECRET_BASE_TEMPLATE_PATH"

SSH_BASE=(ssh -p "$DEPLOY_PORT" -o ConnectTimeout=10)
if [[ -n "${DEPLOY_SSH_KEY_PATH:-}" ]]; then
  SSH_BASE+=(-i "$DEPLOY_SSH_KEY_PATH")
fi

KEEPALIVE_OPTS=(-o ServerAliveInterval=30 -o ServerAliveCountMax=6)
SSH_BASE+=("${KEEPALIVE_OPTS[@]}")

if [[ -n "${DEPLOY_SSH_OPTIONS:-}" ]]; then
  # shellcheck disable=SC2206
  EXTRA_OPTS=($DEPLOY_SSH_OPTIONS)
  SSH_BASE+=("${EXTRA_OPTS[@]}")
fi

TARGET_HOST="${DEPLOY_USER}@${DEPLOY_HOST}"
SSH_BASE+=("$TARGET_HOST")

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

SCP_BASE=(scp -P "$DEPLOY_PORT")
if [[ -n "${DEPLOY_SSH_KEY_PATH:-}" ]]; then
  SCP_BASE+=(-i "$DEPLOY_SSH_KEY_PATH")
fi
SCP_BASE+=("${KEEPALIVE_OPTS[@]}")
if [[ -n "${DEPLOY_SSH_OPTIONS:-}" ]]; then
  # shellcheck disable=SC2206
  EXTRA_SCP_OPTS=($DEPLOY_SSH_OPTIONS)
  SCP_BASE+=("${EXTRA_SCP_OPTS[@]}")
fi

if [[ -n "${DEPLOY_SSH_PASSWORD:-}" ]]; then
  if command -v sshpass >/dev/null 2>&1; then
    SCP_CMD=(sshpass -p "$DEPLOY_SSH_PASSWORD" "${SCP_BASE[@]}")
  else
    if [[ -t 0 && -t 1 ]]; then
      log_warn "未安装 sshpass，将使用 scp 交互输入密码。"
      SCP_CMD=("${SCP_BASE[@]}")
    else
      die "未检测到命令: sshpass。请安装 sshpass，或改用 DEPLOY_SSH_KEY_PATH 配置免密登录。"
    fi
  fi
else
  SCP_CMD=("${SCP_BASE[@]}")
fi

SECRETCTL_LOCAL_BIN="$(mktemp "/tmp/secretctl_linux_amd64.XXXXXX")"
trap 'rm -f "$SECRETCTL_LOCAL_BIN"' EXIT
sb_build_secretctl_linux_binary "$SECRETCTL_LOCAL_BIN"
REMOTE_SECRET_WORK_DIR="$(sb_create_remote_temp_dir | tr -d '\r\n')"
[[ -n "$REMOTE_SECRET_WORK_DIR" ]] || sb_die "创建远端 secretctl 临时目录失败"
sb_upload_remote_artifacts "$REMOTE_SECRET_WORK_DIR" "$SECRETCTL_LOCAL_BIN" "$SECRET_BASE_TEMPLATE_PATH"

REMOTE_SECRETCTL_BIN="${REMOTE_SECRET_WORK_DIR}/$(basename -- "$SECRETCTL_LOCAL_BIN")"
REMOTE_TEMPLATE_PATH="${REMOTE_SECRET_WORK_DIR}/$(basename -- "$SECRET_BASE_TEMPLATE_PATH")"

REMOTE_ARGS="$(printf "%q " \
  "$DEPLOY_IMAGE_FULL" \
  "$CONTAINER_NAME" \
  "$HOST_PORT" \
  "$REMOTE_CONTAINER_PORT" \
  "$REMOTE_DOCKER_NETWORK" \
  "$CONTAINER_IP" \
  "$LOG_PATH" \
  "$UPLOAD_FONTS_HOST_PATH" \
  "$GEN_FONTS_HOST_PATH" \
  "$REMOTE_DOCKER_USE_SUDO" \
  "$REMOTE_DOCKER_LOGIN_TIMEOUT_SECONDS" \
  "$REMOTE_DOCKER_PULL_TIMEOUT_SECONDS" \
  "$REMOTE_DOCKER_PULL_RETRY_COUNT" \
  "$REMOTE_DOCKER_PULL_RETRY_SLEEP_SECONDS" \
  "$REMOTE_CONTAINER_STABILITY_SECONDS" \
  "$REMOTE_SECRETCTL_BIN" \
  "$REMOTE_TEMPLATE_PATH" \
  "$SECRET_BASE_ROOT_PATH" \
  "$SECRET_BASE_REPO" \
  "$SECRET_BASE_SUBPROJECT" \
  "$SECRET_BASE_ENV" \
  "$SECRET_BASE_BUNDLE_FILE" \
  "$SECRET_BASE_RUNTIME_CONFIG" \
  "$SECRET_BASE_IDENTITY_FILE" \
  "$SECRET_BASE_AUDIT_LOG_FILE")"

emit_remote_env() {
  printf 'REGISTRY_HOST=%q\n' "$PULL_REGISTRY_HOST"
  printf 'REGISTRY_USERNAME=%q\n' "$NEXUS_DOCKER_USERNAME"
  printf 'REGISTRY_PASSWORD=%q\n' "$NEXUS_DOCKER_PASSWORD"
}

run_remote_deploy_script() {
  {
    emit_remote_env
    cat <<'REMOTE_SCRIPT'
set -euo pipefail

IMAGE_FULL="$1"
CONTAINER_NAME="$2"
HOST_PORT="$3"
CONTAINER_PORT="$4"
NETWORK_NAME="$5"
CONTAINER_IP="${6:-}"
LOG_PATH="${7:-}"
UPLOAD_FONTS_HOST_PATH="${8:-}"
GEN_FONTS_HOST_PATH="${9:-}"
REMOTE_DOCKER_USE_SUDO="${10:-false}"
REMOTE_DOCKER_LOGIN_TIMEOUT_SECONDS="${11:-30}"
REMOTE_DOCKER_PULL_TIMEOUT_SECONDS="${12:-300}"
REMOTE_DOCKER_PULL_RETRY_COUNT="${13:-2}"
REMOTE_DOCKER_PULL_RETRY_SLEEP_SECONDS="${14:-2}"
REMOTE_CONTAINER_STABILITY_SECONDS="${15:-5}"
SECRETCTL_BIN="${16:-}"
TEMPLATE_PATH="${17:-}"
SECRET_BASE_ROOT="${18:-/srv/darren_secret_base}"
SECRET_BASE_REPO="${19:-}"
SECRET_BASE_SUBPROJECT="${20:-}"
SECRET_BASE_ENV="${21:-}"
BUNDLE_PATH="${22:-}"
RUNTIME_CONFIG_PATH="${23:-}"
IDENTITY_PATH="${24:-}"
AUDIT_LOG_PATH="${25:-}"

USE_SUDO_LOWER="$(printf '%s' "$REMOTE_DOCKER_USE_SUDO" | tr '[:upper:]' '[:lower:]')"
if [[ ! "$REMOTE_DOCKER_LOGIN_TIMEOUT_SECONDS" =~ ^[0-9]+$ ]]; then
  REMOTE_DOCKER_LOGIN_TIMEOUT_SECONDS=30
fi
if [[ ! "$REMOTE_DOCKER_PULL_TIMEOUT_SECONDS" =~ ^[0-9]+$ ]]; then
  REMOTE_DOCKER_PULL_TIMEOUT_SECONDS=300
fi
if [[ ! "$REMOTE_DOCKER_PULL_RETRY_COUNT" =~ ^[0-9]+$ ]]; then
  REMOTE_DOCKER_PULL_RETRY_COUNT=2
fi
if [[ ! "$REMOTE_DOCKER_PULL_RETRY_SLEEP_SECONDS" =~ ^[0-9]+$ ]]; then
  REMOTE_DOCKER_PULL_RETRY_SLEEP_SECONDS=2
fi
if [[ ! "$REMOTE_CONTAINER_STABILITY_SECONDS" =~ ^[0-9]+$ ]]; then
  REMOTE_CONTAINER_STABILITY_SECONDS=5
fi
if (( REMOTE_DOCKER_PULL_RETRY_COUNT < 1 )); then
  REMOTE_DOCKER_PULL_RETRY_COUNT=1
fi

log_step() {
  printf '[REMOTE][%s] %s\n' "$(date '+%H:%M:%S')" "$*"
}

run_with_timeout() {
  local timeout_seconds="${1:-0}"
  shift || true
  if [[ "$timeout_seconds" =~ ^[0-9]+$ ]] && (( timeout_seconds > 0 )); then
    if command -v timeout >/dev/null 2>&1; then
      timeout "$timeout_seconds" "$@"
      return $?
    fi
    if command -v gtimeout >/dev/null 2>&1; then
      gtimeout "$timeout_seconds" "$@"
      return $?
    fi
  fi
  "$@"
}

docker_cmd() {
  if [[ "$USE_SUDO_LOWER" == "true" || "$USE_SUDO_LOWER" == "1" || "$USE_SUDO_LOWER" == "yes" ]]; then
    sudo -n docker "$@"
  else
    docker "$@"
  fi
}

docker_login_once() {
  if [[ -z "$REGISTRY_HOST" || -z "$REGISTRY_USERNAME" || -z "$REGISTRY_PASSWORD" ]]; then
    log_step "Docker 私仓登录信息不完整"
    return 1
  fi

  if [[ "$USE_SUDO_LOWER" == "true" || "$USE_SUDO_LOWER" == "1" || "$USE_SUDO_LOWER" == "yes" ]]; then
    printf '%s' "$REGISTRY_PASSWORD" | run_with_timeout "$REMOTE_DOCKER_LOGIN_TIMEOUT_SECONDS" sudo -n docker login "$REGISTRY_HOST" -u "$REGISTRY_USERNAME" --password-stdin
  else
    printf '%s' "$REGISTRY_PASSWORD" | run_with_timeout "$REMOTE_DOCKER_LOGIN_TIMEOUT_SECONDS" docker login "$REGISTRY_HOST" -u "$REGISTRY_USERNAME" --password-stdin
  fi
}

docker_pull_once() {
  local image_ref="$1"

  if [[ "$USE_SUDO_LOWER" == "true" || "$USE_SUDO_LOWER" == "1" || "$USE_SUDO_LOWER" == "yes" ]]; then
    run_with_timeout "$REMOTE_DOCKER_PULL_TIMEOUT_SECONDS" sudo -n docker pull "$image_ref"
  else
    run_with_timeout "$REMOTE_DOCKER_PULL_TIMEOUT_SECONDS" docker pull "$image_ref"
  fi
}

host_cmd() {
  if [[ "$USE_SUDO_LOWER" == "true" || "$USE_SUDO_LOWER" == "1" || "$USE_SUDO_LOWER" == "yes" ]]; then
    sudo -n "$@"
  else
    "$@"
  fi
}

ensure_dir_with_mode() {
  local dir_path="$1"
  local mode="$2"

  host_cmd mkdir -p "$dir_path"
  host_cmd chmod "$mode" "$dir_path"
}

ensure_user_dir_with_mode() {
  local dir_path="$1"
  local mode="$2"

  mkdir -p "$dir_path"
  chmod "$mode" "$dir_path"
}

wait_for_regular_file() {
  local file_path="$1"
  local timeout_seconds="${2:-5}"
  local interval_seconds="${3:-1}"
  local elapsed=0

  while (( elapsed <= timeout_seconds )); do
    if host_cmd test -f "$file_path"; then
      return 0
    fi
    if (( elapsed == timeout_seconds )); then
      break
    fi
    sleep "$interval_seconds"
    elapsed=$((elapsed + interval_seconds))
  done

  return 1
}

append_bind_mount() {
  local env_name="$1"
  local mount_path="$2"

  if [[ -z "$mount_path" ]]; then
    return 0
  fi

  host_cmd mkdir -p "$mount_path"
  host_cmd chown -R www-data:www-data "$mount_path"
  host_cmd chmod -R 0755 "$mount_path"
  log_step "挂载目录 ${env_name}: ${mount_path}"
  DOCKER_RUN_ARGS+=(-v "${mount_path}:${mount_path}")
  DOCKER_RUN_ARGS+=(-e "${env_name}=${mount_path}")
}

show_container_diagnostics() {
  local container_name="$1"
  local upper_dir=""
  local app_log_path=""

  docker_cmd logs --tail 80 "$container_name" || true

  upper_dir="$(docker_cmd inspect -f '{{.GraphDriver.Data.UpperDir}}' "$container_name" 2>/dev/null || true)"
  if [[ -n "$upper_dir" ]]; then
    app_log_path="${upper_dir%/}/app/logs/app.log"
    if host_cmd test -f "$app_log_path"; then
      log_step "应用日志: $app_log_path"
      host_cmd tail -n 80 "$app_log_path" || true
    fi
  fi
}

docker_pull_with_retry() {
  local image_ref="$1"
  local attempt=1
  local pull_log=""
  local rc=0
  local failure_reason=""

  pull_log="$(mktemp "/tmp/${CONTAINER_NAME}.docker_pull.XXXXXX")"
  while (( attempt <= REMOTE_DOCKER_PULL_RETRY_COUNT )); do
    if docker_pull_once "$image_ref" >"$pull_log" 2>&1; then
      rm -f "$pull_log"
      return 0
    fi

    rc=$?
    failure_reason=""
    if [[ -s "$pull_log" ]]; then
      failure_reason="$(tail -n 8 "$pull_log" | tr '\n' ' ' | sed 's/[[:space:]]\+/ /g; s/^ //; s/ $//')"
    fi

    if [[ "$rc" -eq 124 ]]; then
      if [[ -n "$failure_reason" ]]; then
        log_step "docker pull 超时(${REMOTE_DOCKER_PULL_TIMEOUT_SECONDS}s): ${image_ref}（${attempt}/${REMOTE_DOCKER_PULL_RETRY_COUNT}），原因: ${failure_reason}"
      else
        log_step "docker pull 超时(${REMOTE_DOCKER_PULL_TIMEOUT_SECONDS}s): ${image_ref}（${attempt}/${REMOTE_DOCKER_PULL_RETRY_COUNT}）"
      fi
    else
      if [[ -n "$failure_reason" ]]; then
        log_step "docker pull 失败(rc=${rc}): ${image_ref}（${attempt}/${REMOTE_DOCKER_PULL_RETRY_COUNT}），原因: ${failure_reason}"
      else
        log_step "docker pull 失败(rc=${rc}): ${image_ref}（${attempt}/${REMOTE_DOCKER_PULL_RETRY_COUNT}）"
      fi
    fi

    if (( attempt < REMOTE_DOCKER_PULL_RETRY_COUNT )) && (( REMOTE_DOCKER_PULL_RETRY_SLEEP_SECONDS > 0 )); then
      sleep "$REMOTE_DOCKER_PULL_RETRY_SLEEP_SECONDS"
    fi
    attempt=$((attempt + 1))
  done

  rm -f "$pull_log"
  return 1
}

collect_port_listener_pids() {
  local port="$1"

  if command -v lsof >/dev/null 2>&1; then
    if [[ "$USE_SUDO_LOWER" == "true" || "$USE_SUDO_LOWER" == "1" || "$USE_SUDO_LOWER" == "yes" ]]; then
      sudo -n lsof -t -iTCP:"$port" -sTCP:LISTEN 2>/dev/null | sort -u || true
    else
      lsof -t -iTCP:"$port" -sTCP:LISTEN 2>/dev/null | sort -u || true
    fi
    return 0
  fi

  if command -v ss >/dev/null 2>&1; then
    local ss_output=""
    if [[ "$USE_SUDO_LOWER" == "true" || "$USE_SUDO_LOWER" == "1" || "$USE_SUDO_LOWER" == "yes" ]]; then
      ss_output="$(sudo -n ss -ltnp 2>/dev/null || true)"
    else
      ss_output="$(ss -ltnp 2>/dev/null || true)"
    fi
    printf '%s\n' "$ss_output" | awk -v p=":$port" '
      index($4, p) {
        line=$0
        while (match(line, /pid=[0-9]+/)) {
          pid=substr(line, RSTART+4, RLENGTH-4)
          print pid
          line=substr(line, RSTART+RLENGTH)
        }
      }
    ' | sort -u
    return 0
  fi
}

port_is_listening() {
  local port="$1"

  if command -v lsof >/dev/null 2>&1; then
    if [[ "$USE_SUDO_LOWER" == "true" || "$USE_SUDO_LOWER" == "1" || "$USE_SUDO_LOWER" == "yes" ]]; then
      sudo -n lsof -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1
    else
      lsof -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1
    fi
    return $?
  fi

  if command -v ss >/dev/null 2>&1; then
    local ss_output=""
    if [[ "$USE_SUDO_LOWER" == "true" || "$USE_SUDO_LOWER" == "1" || "$USE_SUDO_LOWER" == "yes" ]]; then
      ss_output="$(sudo -n ss -ltn 2>/dev/null || true)"
    else
      ss_output="$(ss -ltn 2>/dev/null || true)"
    fi
    printf '%s\n' "$ss_output" | awk -v p=":$port" '$4 ~ p"$" {found=1} END{exit(found?0:1)}'
    return $?
  fi

  return 1
}

free_host_port() {
  local port="$1"
  local cids=""
  local cid
  local pids=""
  local pid

  cids="$(docker_cmd ps -q --filter "publish=${port}" 2>/dev/null || true)"
  if [[ -n "$cids" ]]; then
    log_step "检测到端口 ${port} 被容器占用，准备释放"
    for cid in $cids; do
      [[ -n "$cid" ]] || continue
      docker_cmd stop "$cid" >/dev/null 2>&1 || true
      docker_cmd rm "$cid" >/dev/null 2>&1 || true
    done
  fi

  pids="$(collect_port_listener_pids "$port" || true)"
  if [[ -n "$pids" ]]; then
    log_step "检测到端口 ${port} 被进程占用，准备终止: ${pids}"
    for pid in $pids; do
      [[ -n "$pid" ]] || continue
      host_cmd kill -TERM "$pid" >/dev/null 2>&1 || true
    done

    sleep 2
    for pid in $pids; do
      [[ -n "$pid" ]] || continue
      if host_cmd kill -0 "$pid" >/dev/null 2>&1; then
        host_cmd kill -KILL "$pid" >/dev/null 2>&1 || true
      fi
    done
  fi
}

log_step "开始远端部署脚本"
SECRETCTL_WORK_DIR="$(dirname -- "$SECRETCTL_BIN")"
trap 'rm -rf "$SECRETCTL_WORK_DIR"' EXIT

if [[ -z "$SECRETCTL_BIN" || -z "$TEMPLATE_PATH" || -z "$BUNDLE_PATH" || -z "$RUNTIME_CONFIG_PATH" || -z "$IDENTITY_PATH" ]]; then
  log_step "secret_base 参数不完整"
  exit 1
fi

if [[ ! -x "$SECRETCTL_BIN" ]]; then
  log_step "secretctl 不可执行: $SECRETCTL_BIN"
  exit 1
fi

if [[ ! -f "$TEMPLATE_PATH" ]]; then
  log_step "模板 YAML 不存在: $TEMPLATE_PATH"
  exit 1
fi

if [[ ! -f "$BUNDLE_PATH" ]]; then
  log_step "secret bundle 不存在: $BUNDLE_PATH"
  exit 1
fi

if [[ ! -f "$IDENTITY_PATH" ]]; then
  log_step "age identity 不存在: $IDENTITY_PATH"
  exit 1
fi

log_step "校验 secret bundle 与模板"
if ! "$SECRETCTL_BIN" validate \
  --template "$TEMPLATE_PATH" \
  --bundle "$BUNDLE_PATH" \
  --identity-file "$IDENTITY_PATH" \
  --expect-repo "$SECRET_BASE_REPO" \
  --expect-subproject "$SECRET_BASE_SUBPROJECT" \
  --expect-env "$SECRET_BASE_ENV"; then
  log_step "secret bundle 校验失败"
  exit 1
fi

ensure_user_dir_with_mode "$(dirname "$RUNTIME_CONFIG_PATH")" 0750
ensure_user_dir_with_mode "$(dirname "$AUDIT_LOG_PATH")" 0750

RUNTIME_CONFIG_STAGING_PATH="${RUNTIME_CONFIG_PATH}.staging"
if [[ -d "$RUNTIME_CONFIG_STAGING_PATH" ]]; then
  log_step "清理错误的运行时配置暂存目录: $RUNTIME_CONFIG_STAGING_PATH"
  rm -rf "$RUNTIME_CONFIG_STAGING_PATH"
fi
rm -f "$RUNTIME_CONFIG_STAGING_PATH"

log_step "渲染运行时配置: $RUNTIME_CONFIG_STAGING_PATH"
if ! "$SECRETCTL_BIN" render \
  --template "$TEMPLATE_PATH" \
  --bundle "$BUNDLE_PATH" \
  --identity-file "$IDENTITY_PATH" \
  --out "$RUNTIME_CONFIG_STAGING_PATH" \
  --expect-repo "$SECRET_BASE_REPO" \
  --expect-subproject "$SECRET_BASE_SUBPROJECT" \
  --expect-env "$SECRET_BASE_ENV" \
  --audit-log "$AUDIT_LOG_PATH" \
  --operator "${SUDO_USER:-${USER:-unknown}}"; then
  log_step "运行时配置渲染失败"
  exit 1
fi

if ! wait_for_regular_file "$RUNTIME_CONFIG_STAGING_PATH" 5 1; then
  log_step "运行时配置未生成常规文件: $RUNTIME_CONFIG_STAGING_PATH"
  host_cmd ls -la "$(dirname "$RUNTIME_CONFIG_STAGING_PATH")" || true
  exit 1
fi

if [[ -d "$RUNTIME_CONFIG_PATH" ]]; then
  log_step "清理错误的运行时配置目录: $RUNTIME_CONFIG_PATH"
  rm -rf "$RUNTIME_CONFIG_PATH"
fi
mv -f "$RUNTIME_CONFIG_STAGING_PATH" "$RUNTIME_CONFIG_PATH"

log_step "登录 Docker 私仓: $REGISTRY_HOST"
if ! docker_login_once; then
  log_step "Docker 私仓登录失败"
  exit 1
fi

log_step "拉取镜像: $IMAGE_FULL"
if ! docker_pull_with_retry "$IMAGE_FULL"; then
  log_step "镜像拉取失败: $IMAGE_FULL"
  exit 1
fi

if ! docker_cmd image inspect "$IMAGE_FULL" >/dev/null 2>&1; then
  log_step "docker pull 完成，但未找到镜像标签: $IMAGE_FULL"
  exit 1
fi

log_step "停止并移除旧容器: $CONTAINER_NAME"
docker_cmd stop "$CONTAINER_NAME" 2>/dev/null || true
docker_cmd rm "$CONTAINER_NAME" 2>/dev/null || true

free_host_port "$HOST_PORT"
if port_is_listening "$HOST_PORT"; then
  log_step "端口 ${HOST_PORT} 仍被占用，无法启动容器"
  exit 1
fi

log_step "启动新容器: $CONTAINER_NAME"
DOCKER_RUN_ARGS=(-d --name "$CONTAINER_NAME" --restart unless-stopped -p "${HOST_PORT}:${CONTAINER_PORT}")
if [[ -n "$NETWORK_NAME" ]]; then
  DOCKER_RUN_ARGS+=(--network "$NETWORK_NAME")
fi
if [[ -n "$CONTAINER_IP" ]]; then
  DOCKER_RUN_ARGS+=(--ip "$CONTAINER_IP")
fi
if [[ -n "$LOG_PATH" ]]; then
  mkdir -p "$LOG_PATH"
  DOCKER_RUN_ARGS+=(-v "${LOG_PATH}:/app/logs")
fi
append_bind_mount "UPLOAD_FONTS_DIR" "$UPLOAD_FONTS_HOST_PATH"
append_bind_mount "GEN_FONTS_DIR" "$GEN_FONTS_HOST_PATH"
DOCKER_RUN_ARGS+=(-v "$(dirname "$RUNTIME_CONFIG_PATH"):/app/config:ro")
DOCKER_RUN_ARGS+=("$IMAGE_FULL")

docker_cmd run "${DOCKER_RUN_ARGS[@]}" >/dev/null

STATUS=""
ELAPSED=0
MAX_WAIT=20
INTERVAL=2
while (( ELAPSED < MAX_WAIT )); do
  STATUS="$(docker_cmd inspect -f '{{.State.Status}}' "$CONTAINER_NAME" 2>/dev/null || echo unknown)"
  if [[ "$STATUS" == "running" ]]; then
    break
  fi
  if [[ "$STATUS" == "exited" || "$STATUS" == "dead" ]]; then
    log_step "容器启动失败，状态: $STATUS"
    show_container_diagnostics "$CONTAINER_NAME"
    exit 1
  fi
  sleep "$INTERVAL"
  ELAPSED=$((ELAPSED + INTERVAL))
done

if [[ "$STATUS" != "running" ]]; then
  log_step "容器未在 ${MAX_WAIT}s 内进入 running 状态，当前: $STATUS"
  show_container_diagnostics "$CONTAINER_NAME"
  exit 1
fi

RESTART_COUNT_BEFORE="$(docker_cmd inspect -f '{{.RestartCount}}' "$CONTAINER_NAME" 2>/dev/null || echo -1)"
if (( REMOTE_CONTAINER_STABILITY_SECONDS > 0 )); then
  log_step "进入稳定性观察窗口: ${REMOTE_CONTAINER_STABILITY_SECONDS}s"
  sleep "$REMOTE_CONTAINER_STABILITY_SECONDS"
fi
STATUS_AFTER="$(docker_cmd inspect -f '{{.State.Status}}' "$CONTAINER_NAME" 2>/dev/null || echo unknown)"
RESTART_COUNT_AFTER="$(docker_cmd inspect -f '{{.RestartCount}}' "$CONTAINER_NAME" 2>/dev/null || echo -1)"

if [[ "$STATUS_AFTER" != "running" ]]; then
  log_step "容器未通过稳定性检查，当前状态: $STATUS_AFTER"
  show_container_diagnostics "$CONTAINER_NAME"
  exit 1
fi

if [[ "$RESTART_COUNT_BEFORE" =~ ^[0-9]+$ && "$RESTART_COUNT_AFTER" =~ ^[0-9]+$ ]]; then
  if (( RESTART_COUNT_AFTER > RESTART_COUNT_BEFORE )); then
    log_step "容器在稳定性窗口内发生重启: ${RESTART_COUNT_BEFORE} -> ${RESTART_COUNT_AFTER}"
    show_container_diagnostics "$CONTAINER_NAME"
    exit 1
  fi
fi

docker_cmd image prune -f
log_step "远端部署完成"
REMOTE_SCRIPT
  } | "${SSH_CMD[@]}" "bash -s -- ${REMOTE_ARGS}"
}

log_info "开始远程部署: ${DEPLOY_USER}@${DEPLOY_HOST}:${DEPLOY_PORT}"
log_info "容器: $CONTAINER_NAME"
log_info "镜像: ${DEPLOY_IMAGE_FULL}"
log_info "端口映射: ${HOST_PORT}:${REMOTE_CONTAINER_PORT}"
log_info "Nexus 拉取地址: ${PULL_REGISTRY_HOST}"
log_info "secret bundle: ${SECRET_BASE_BUNDLE_FILE}"
log_info "运行时配置: ${SECRET_BASE_RUNTIME_CONFIG}"

if ! run_remote_deploy_script; then
  die "远端部署命令执行失败"
fi

log_info "远程部署完成: $CONTAINER_NAME"
