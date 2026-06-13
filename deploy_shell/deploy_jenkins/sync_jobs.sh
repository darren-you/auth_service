#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

# 单文件兼容入口：外层使用 POSIX sh，自举拉起 bash 执行真正实现。
exec bash -s -- "$SCRIPT_DIR" "$@" <<'__SYNC_JOBS_BASH__'
# shellcheck shell=bash
set -euo pipefail

SCRIPT_DIR="$1"
shift
DEPLOY_SHELL_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# shellcheck disable=SC1091
source "${SCRIPT_DIR}/common.sh"

readonly DEFAULT_CONFIG_PATH="${SCRIPT_DIR}/jobs_config.json"
readonly PYTHON_HELPER="${SCRIPT_DIR}/sync_jobs.py"
readonly MANAGED_MARKER_FILE=".deploy_shell_managed_job.json"

ACTION=""
CONFIG_PATH="${DEFAULT_CONFIG_PATH}"
REMOTE_JENKINS_HOME=""
RELOAD_AFTER_PUSH="true"
DRY_RUN="false"

usage() {
  cat <<'EOF'
用法：
  bash deploy_shell/deploy_jenkins/sync_jobs.sh pull [--config /abs/path/jobs_config.json] [--jenkins-home /abs/path/.jenkins]
  bash deploy_shell/deploy_jenkins/sync_jobs.sh push [--config /abs/path/jobs_config.json] [--jenkins-home /abs/path/.jenkins] [--reload true|false] [--dry-run]

说明：
  pull: 从 Jenkins 机器上的 Freestyle 工程反向生成 jobs_config.json，识别 swift_app/android_app/web/server/nginx/ssl 六类 deploy_shell 工程
  push:  将 jobs_config.json 渲染为 Jenkins config.xml，通过 SSH 同步到 Jenkins 机器

参数：
  --config          Jenkins 工程配置文件路径，默认 deploy_shell/deploy_jenkins/jobs_config.json
  --jenkins-home    远端 Jenkins Home 绝对路径；省略时默认使用远端 $HOME/.jenkins
  --reload          push 完成后是否重载 Jenkins，默认 true
  --dry-run         仅渲染并打印计划，不写远端
EOF
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      pull|push)
        [[ -z "$ACTION" ]] || die "只能指定一个动作：pull 或 push"
        ACTION="$1"
        shift
        ;;
      --config)
        [[ $# -ge 2 ]] || die "--config 缺少参数"
        CONFIG_PATH="$2"
        shift 2
        ;;
      --jenkins-home)
        [[ $# -ge 2 ]] || die "--jenkins-home 缺少参数"
        REMOTE_JENKINS_HOME="$2"
        shift 2
        ;;
      --reload)
        [[ $# -ge 2 ]] || die "--reload 缺少参数"
        RELOAD_AFTER_PUSH="$2"
        shift 2
        ;;
      --dry-run)
        DRY_RUN="true"
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die "未知参数: $1"
        ;;
    esac
  done

  [[ -n "$ACTION" ]] || {
    usage
    die "必须指定动作：pull 或 push"
  }
}

resolve_remote_jenkins_home() {
  if [[ -n "$REMOTE_JENKINS_HOME" ]]; then
    printf '%s\n' "$REMOTE_JENKINS_HOME"
    return
  fi

  ssh_exec 'printf "%s\n" "$HOME/.jenkins"'
}

run_remote_script() {
  local script="$1"
  shift || true
  ssh_exec env \
    "DEPLOY_JENKINS_URL=${JENKINS_URL:-https://jenkins.xdarren.com}" \
    "DEPLOY_JENKINS_HOME=${RESOLVED_REMOTE_JENKINS_HOME}" \
    "DEPLOY_MARKER_FILE=${MANAGED_MARKER_FILE}" \
    bash -s -- "$@" <<<"$script"
}

pull_remote_jobs() {
  local raw_file
  local root_config_file
  raw_file="$(mktemp)"
  root_config_file="$(mktemp)"

  run_remote_script "$(cat <<'EOF'
set -euo pipefail
JHOME="${DEPLOY_JENKINS_HOME:-$HOME/.jenkins}"

if [[ ! -d "$JHOME/jobs" ]]; then
  exit 0
fi

find "$JHOME/jobs" -mindepth 2 -maxdepth 2 -type f -name config.xml | sort | while IFS= read -r config_file; do
  job_name="$(basename "$(dirname "$config_file")")"
  printf '%s\t' "$job_name"
  base64 < "$config_file" | tr -d '\n'
  printf '\n'
done
EOF
)" >"$raw_file"

  ssh_exec "cat $(shell_quote "${RESOLVED_REMOTE_JENKINS_HOME}/config.xml")" >"$root_config_file"

  python3 "$PYTHON_HELPER" import-remote \
    --input "$raw_file" \
    --output "$CONFIG_PATH" \
    --jenkins-config "$root_config_file"
  rm -f "$raw_file"
  rm -f "$root_config_file"

  log_info "已从远端 Jenkins 拉取工程配置 -> ${CONFIG_PATH}"
}

render_local_jobs() {
  local render_dir="$1"
  python3 "$PYTHON_HELPER" render --config "$CONFIG_PATH" --out-dir "$render_dir"
}

read_manifest_job_names() {
  local manifest_path="$1"
  python3 - "$manifest_path" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as fh:
    payload = json.load(fh)

for item in payload.get("jobs", []):
    print(item["name"])
PY
}

backup_remote_jobs() {
  local -a keep_jobs=("$@")
  run_remote_script "$(cat <<'EOF'
set -euo pipefail
JHOME="${DEPLOY_JENKINS_HOME:-$HOME/.jenkins}"

mkdir -p "$JHOME/jobs"
for job_name in "$@"; do
  remote_job_dir="$JHOME/jobs/$job_name"
  remote_config="$remote_job_dir/config.xml"
  mkdir -p "$remote_job_dir"
  if [[ -f "$remote_config" ]]; then
    cp "$remote_config" "${remote_config}.bak.deploy_shell"
  fi
done
EOF
)" "${keep_jobs[@]}"
}

rsync_jobs_to_remote() {
  local render_dir="$1"
  local remote_jobs_dir="${RESOLVED_REMOTE_JENKINS_HOME}/jobs"
  ssh_exec "mkdir -p $(shell_quote "$remote_jobs_dir")"
  rsync_to_remote "${render_dir}/" "${remote_jobs_dir}/" --exclude=manifest.json
}

delete_remote_orphans() {
  local -a keep_jobs=("$@")
  run_remote_script "$(cat <<'EOF'
set -euo pipefail
python3 - "$@" <<'PY'
from __future__ import annotations

import os
import re
import shutil
import sys
from pathlib import Path

jhome = Path(os.environ.get("DEPLOY_JENKINS_HOME", f"{Path.home()}/.jenkins"))
marker_file = os.environ["DEPLOY_MARKER_FILE"]
keep_jobs = set(sys.argv[1:])
pattern = re.compile(r"deploy_shell/(deploy_swift_app|deploy_android_app|deploy_flutter_ios|deploy_flutter_android|deploy_web|deploy_server|deploy_nginx|deploy_ssl)/remote_deploy_pipeline\.sh")

jobs_dir = jhome / "jobs"
if not jobs_dir.is_dir():
    raise SystemExit(0)

for job_dir in sorted(path for path in jobs_dir.iterdir() if path.is_dir()):
    if job_dir.name in keep_jobs:
        continue
    managed = (job_dir / marker_file).is_file()
    if not managed:
        config_file = job_dir / "config.xml"
        if config_file.is_file():
            managed = pattern.search(config_file.read_text(encoding="utf-8", errors="ignore")) is not None
    if not managed:
        continue
    shutil.rmtree(job_dir)
    print(f"deleted: {job_dir.name}")
PY
EOF
)" "${keep_jobs[@]}"
}

sync_remote_views() {
  local manifest_path="$1"
  local remote_root_config="${RESOLVED_REMOTE_JENKINS_HOME}/config.xml"
  local local_root_config
  local_root_config="$(mktemp)"

  ssh_exec "cat $(shell_quote "$remote_root_config")" >"$local_root_config"
  python3 "$PYTHON_HELPER" rewrite-views \
    --manifest "$manifest_path" \
    --jenkins-config "$local_root_config"
  ssh_exec "if [[ -f $(shell_quote "$remote_root_config") ]]; then cp $(shell_quote "$remote_root_config") $(shell_quote "${remote_root_config}.bak.deploy_shell"); fi"
  rsync_to_remote "$local_root_config" "$remote_root_config"
  rm -f "$local_root_config"
}

reload_remote_jenkins() {
  run_remote_script "$(cat <<'EOF'
set -euo pipefail

uid="$(id -u)"
service_label="gui/${uid}/homebrew.mxcl.jenkins-lts"

if launchctl print "$service_label" >/dev/null 2>&1; then
  launchctl kickstart -k "$service_label"
else
  echo "未检测到 ${service_label}，无法自动重载 Jenkins" >&2
  exit 1
fi

for _ in $(seq 1 60); do
  if curl -fsS "${DEPLOY_JENKINS_URL%/}/login" >/dev/null 2>&1; then
    echo "jenkins-ready"
    exit 0
  fi
  sleep 2
done

echo "Jenkins 重载后 120 秒内未恢复" >&2
exit 1
EOF
)"
}

push_local_jobs() {
  local render_dir
  render_dir="$(mktemp -d)"
  render_local_jobs "$render_dir"

  local manifest_path="${render_dir}/manifest.json"
  local -a job_names=()
  while IFS= read -r job_name; do
    [[ -n "$job_name" ]] || continue
    job_names+=("$job_name")
  done < <(read_manifest_job_names "$manifest_path")
  [[ ${#job_names[@]} -gt 0 ]] || die "配置文件中没有可同步的 Jenkins 工程"

  log_info "本次准备同步 ${#job_names[@]} 个 Jenkins 工程 -> ${RESOLVED_REMOTE_JENKINS_HOME}"
  printf '%s\n' "${job_names[@]}" | sed 's/^/  - /'

  if is_true "$DRY_RUN"; then
    log_info "dry-run 模式，不会写入远端 Jenkins"
    rm -rf "$render_dir"
    return
  fi

  log_info "开始备份远端 Jenkins 工程配置"
  backup_remote_jobs "${job_names[@]}"

  log_info "开始通过 rsync 批量同步 Jenkins 工程目录"
  rsync_jobs_to_remote "$render_dir"

  log_info "开始删除远端多余的托管工程"
  delete_remote_orphans "${job_names[@]}"

  log_info "开始同步 Jenkins 视图"
  sync_remote_views "$manifest_path"

  if is_true "$RELOAD_AFTER_PUSH"; then
    log_info "开始重载远端 Jenkins"
    reload_remote_jenkins
  else
    log_warn "已跳过 Jenkins 重载。新的 config.xml 已写入磁盘，但 UI 中可能还看不到变更。"
  fi

  rm -rf "$render_dir"
  log_info "Jenkins 工程同步完成"
}

main() {
  parse_args "$@"
  require_cmd python3

  case "$ACTION" in
    pull)
      require_cmd ssh
      setup_jenkins_ssh_profile "${JENKINS_SSH_PROFILE:-mac_mini}"
      setup_jenkins_ssh
      RESOLVED_REMOTE_JENKINS_HOME="$(resolve_remote_jenkins_home)"
      [[ -n "$RESOLVED_REMOTE_JENKINS_HOME" ]] || die "无法解析远端 Jenkins Home"
      pull_remote_jobs
      ;;
    push)
      [[ -f "$CONFIG_PATH" ]] || die "找不到配置文件: $CONFIG_PATH"
      require_cmd ssh
      require_cmd rsync
      setup_jenkins_ssh_profile "${JENKINS_SSH_PROFILE:-mac_mini}"
      setup_jenkins_ssh
      RESOLVED_REMOTE_JENKINS_HOME="$(resolve_remote_jenkins_home)"
      [[ -n "$RESOLVED_REMOTE_JENKINS_HOME" ]] || die "无法解析远端 Jenkins Home"
      push_local_jobs
      ;;
    *)
      die "不支持的动作: $ACTION"
      ;;
  esac
}

main "$@"
__SYNC_JOBS_BASH__
