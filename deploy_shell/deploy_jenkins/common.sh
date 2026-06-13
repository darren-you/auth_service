#!/usr/bin/env bash
# shellcheck shell=bash

log_info() {
  echo "[$(date '+%H:%M:%S')][INFO] $*"
}

log_warn() {
  echo "[$(date '+%H:%M:%S')][WARN] $*" >&2
}

log_error() {
  echo "[$(date '+%H:%M:%S')][ERROR] $*" >&2
}

die() {
  log_error "$*"
  exit 1
}

require_cmd() {
  local cmd="$1"
  command -v "$cmd" >/dev/null 2>&1 || die "未检测到命令: $cmd"
}

is_true() {
  local value="${1:-false}"
  value="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')"
  case "$value" in
    1|true|yes|y|on) return 0 ;;
    *) return 1 ;;
  esac
}

shell_quote() {
  local value="${1:-}"
  printf "'%s'" "$(printf '%s' "$value" | sed "s/'/'\\\\''/g")"
}

setup_jenkins_ssh_profile() {
  local profile_name="$1"
  local script_dir=""
  local loader_file=""

  script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
  loader_file="$script_dir/../shared/load_jenkins_profile.sh"
  [[ -f "$loader_file" ]] || die "找不到 Jenkins SSH profile 加载器: $loader_file"

  # shellcheck disable=SC1090
  source "$loader_file"
  apply_jenkins_profile_defaults "$profile_name"
}

setup_jenkins_ssh() {
  [[ -n "${JENKINS_SSH_HOST:-}" ]] || die "JENKINS_SSH_HOST 未定义"
  [[ -n "${JENKINS_SSH_PORT:-}" ]] || die "JENKINS_SSH_PORT 未定义"
  [[ -n "${JENKINS_SSH_USER:-}" ]] || die "JENKINS_SSH_USER 未定义"

  local -a ssh_opts=()
  local known_hosts_file="${JENKINS_SSH_KNOWN_HOSTS_FILE:-${HOME}/.ssh/known_hosts}"
  local known_hosts_dir=""
  local known_hosts_lookup="${JENKINS_SSH_HOST}"
  local keyscan_output=""
  if [[ -n "${JENKINS_SSH_OPTIONS:-}" ]]; then
    read -r -a ssh_opts <<< "${JENKINS_SSH_OPTIONS}"
  fi

  known_hosts_dir="$(dirname "$known_hosts_file")"
  mkdir -p "$known_hosts_dir"
  touch "$known_hosts_file"

  if [[ "$known_hosts_dir" == "${HOME}/.ssh" ]]; then
    chmod 700 "$known_hosts_dir"
  fi
  if [[ "$known_hosts_file" == "${HOME}/.ssh/known_hosts" ]]; then
    chmod 600 "$known_hosts_file"
  fi

  require_cmd ssh-keygen
  require_cmd ssh-keyscan
  if [[ "${JENKINS_SSH_PORT}" != "22" ]]; then
    known_hosts_lookup="[${JENKINS_SSH_HOST}]:${JENKINS_SSH_PORT}"
  fi
  if ! ssh-keygen -F "$known_hosts_lookup" -f "$known_hosts_file" >/dev/null 2>&1; then
    log_info "首次写入 Jenkins SSH 指纹 -> ${known_hosts_file}"
    keyscan_output="$(ssh-keyscan -p "${JENKINS_SSH_PORT}" "${JENKINS_SSH_HOST}" 2>/dev/null || true)"
    [[ -n "$keyscan_output" ]] || die "无法获取 Jenkins SSH 主机指纹: ${JENKINS_SSH_HOST}:${JENKINS_SSH_PORT}"
    printf '%s\n' "$keyscan_output" >> "$known_hosts_file"
  fi

  SSH_TARGET="${JENKINS_SSH_TARGET:-${JENKINS_SSH_USER}@${JENKINS_SSH_HOST}}"
  SSH_PREFIX=()
  if [[ -n "${JENKINS_SSH_PASSWORD:-}" ]]; then
    require_cmd sshpass
    SSH_PREFIX=(sshpass -p "${JENKINS_SSH_PASSWORD}")
  fi

  SSH_BASE=(ssh -p "${JENKINS_SSH_PORT}" "${ssh_opts[@]}" \
    -o UserKnownHostsFile="${known_hosts_file}" \
    -o HostKeyAlias="${known_hosts_lookup}" \
    -o IdentitiesOnly=yes \
    -o StrictHostKeyChecking=yes)
  RSYNC_SSH_OPTS=(-p "${JENKINS_SSH_PORT}" "${ssh_opts[@]}" \
    -o UserKnownHostsFile="${known_hosts_file}" \
    -o HostKeyAlias="${known_hosts_lookup}" \
    -o IdentitiesOnly=yes \
    -o StrictHostKeyChecking=yes)

  if [[ -n "${JENKINS_SSH_KEY_PATH:-}" ]]; then
    [[ -f "${JENKINS_SSH_KEY_PATH}" ]] || die "JENKINS_SSH_KEY_PATH 不存在: ${JENKINS_SSH_KEY_PATH}"
    SSH_BASE+=(-i "${JENKINS_SSH_KEY_PATH}")
    RSYNC_SSH_OPTS+=(-i "${JENKINS_SSH_KEY_PATH}")
  fi

  SSH_CMD=("${SSH_PREFIX[@]}" "${SSH_BASE[@]}")

  RSYNC_RSH="ssh"
  for opt in "${RSYNC_SSH_OPTS[@]}"; do
    RSYNC_RSH+=" ${opt}"
  done
  if [[ -n "${JENKINS_SSH_PASSWORD:-}" ]]; then
    RSYNC_CMD=(sshpass -p "${JENKINS_SSH_PASSWORD}" rsync -az)
  else
    RSYNC_CMD=(rsync -az)
  fi
}

ssh_exec() {
  "${SSH_CMD[@]}" "$SSH_TARGET" "$@"
}

rsync_to_remote() {
  local local_path="$1"
  local remote_path="$2"
  shift 2 || true
  "${RSYNC_CMD[@]}" "$@" -e "$RSYNC_RSH" "$local_path" "${SSH_TARGET}:$(shell_quote "$remote_path")"
}
