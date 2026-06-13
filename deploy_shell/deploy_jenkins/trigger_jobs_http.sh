#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

exec bash -s -- "$SCRIPT_DIR" "$@" <<'__TRIGGER_HTTP_BASH__'
# shellcheck shell=bash
set -euo pipefail

SCRIPT_DIR="$1"
shift

readonly DEFAULT_CONFIG_PATH="${SCRIPT_DIR}/jobs_config.json"
readonly PYTHON_HELPER="${SCRIPT_DIR}/sync_jobs.py"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/common.sh"

ACTION=""
CONFIG_PATH="$DEFAULT_CONFIG_PATH"
JOB_TYPE=""
JOB_NAME=""
JENKINS_URL="${JENKINS_URL:-https://jenkins.xdarren.com}"
REMOTE_JENKINS_HOME_PATH="${JENKINS_HOME_PATH:-}"
BRANCH_NAME=""
BUILD_ENV_VALUE=""
WAIT_BUILD="true"
WAIT_TIMEOUT_SECONDS="${WAIT_TIMEOUT_SECONDS:-1800}"

declare -a EXTRA_PARAMS=()

usage() {
  cat <<'EOF'
用法：
  bash deploy_shell/deploy_jenkins/trigger_jobs_http.sh list [--config /abs/path/jobs_config.json] [--type server] [--job job_name]
  bash deploy_shell/deploy_jenkins/trigger_jobs_http.sh trigger [--config /abs/path/jobs_config.json] [--type server] [--job job_name] [--branch origin/master] [--build-env test] [--param KEY=VALUE] [--wait true|false]

说明：
  list    输出可用的 HTTP 触发 job、默认分支与 token
  trigger 通过 Build Authorization Token Root Plugin 的 HTTP 接口触发 Jenkins job

参数：
  --config        jobs_config.json 路径，默认 deploy_shell/deploy_jenkins/jobs_config.json
  --type          按 job.type 过滤，例如 server
  --job           仅触发指定 job.name
  --jenkins-url   Jenkins 根地址，默认 https://jenkins.xdarren.com
  --jenkins-home  远端 Jenkins Home 路径；省略时通过共享 Jenkins SSH profile 解析远端 $HOME/.jenkins
  --branch        覆盖 BuildBranch；省略时使用 job 默认值
  --build-env     传入 BuildEnv；对 server / app 类 job 常用
  --param         追加任意构建参数，可多次传入，例如 --param BuildChannel=official
  --wait          是否等待构建完成，默认 true
EOF
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      list|trigger)
        [[ -z "$ACTION" ]] || die "只能指定一个动作：list 或 trigger"
        ACTION="$1"
        shift
        ;;
      --config)
        [[ $# -ge 2 ]] || die "--config 缺少参数"
        CONFIG_PATH="$2"
        shift 2
        ;;
      --type)
        [[ $# -ge 2 ]] || die "--type 缺少参数"
        JOB_TYPE="$2"
        shift 2
        ;;
      --job)
        [[ $# -ge 2 ]] || die "--job 缺少参数"
        JOB_NAME="$2"
        shift 2
        ;;
      --jenkins-url)
        [[ $# -ge 2 ]] || die "--jenkins-url 缺少参数"
        JENKINS_URL="$2"
        shift 2
        ;;
      --jenkins-home)
        [[ $# -ge 2 ]] || die "--jenkins-home 缺少参数"
        REMOTE_JENKINS_HOME_PATH="$2"
        shift 2
        ;;
      --branch)
        [[ $# -ge 2 ]] || die "--branch 缺少参数"
        BRANCH_NAME="$2"
        shift 2
        ;;
      --build-env)
        [[ $# -ge 2 ]] || die "--build-env 缺少参数"
        BUILD_ENV_VALUE="$2"
        shift 2
        ;;
      --param)
        [[ $# -ge 2 ]] || die "--param 缺少参数"
        EXTRA_PARAMS+=("$2")
        shift 2
        ;;
      --wait)
        [[ $# -ge 2 ]] || die "--wait 缺少参数"
        WAIT_BUILD="$2"
        shift 2
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
    die "必须指定动作：list 或 trigger"
  }
}

read_jobs_manifest() {
  local -a cmd=(python3 "$PYTHON_HELPER" list-http-triggers --config "$CONFIG_PATH")
  if [[ -n "$JOB_TYPE" ]]; then
    cmd+=(--type "$JOB_TYPE")
  fi
  "${cmd[@]}"
}

filter_jobs_manifest() {
  local manifest_json="$1"
  MANIFEST_JSON="$manifest_json" python3 - "$JOB_NAME" <<'PY'
import json
import os
import sys

job_name = sys.argv[1].strip()
payload = json.loads(os.environ["MANIFEST_JSON"])
if job_name:
    payload = [item for item in payload if item["name"] == job_name]
print(json.dumps(payload, ensure_ascii=False, indent=2))
PY
}

print_jobs_list() {
  local manifest_json="$1"
  MANIFEST_JSON="$manifest_json" python3 - "$JENKINS_URL" <<'PY'
import json
import os
import sys

jenkins_url = sys.argv[1].rstrip("/")
payload = json.loads(os.environ["MANIFEST_JSON"])
for item in payload:
    endpoint = f"{jenkins_url}/buildByToken/buildWithParameters?job={item['name']}&token={item['remote_trigger_token']}"
    print(f"{item['name']}\t{item['type']}\t{item['branch_parameter_default']}\t{item['remote_trigger_token']}\t{endpoint}")
PY
}

urlencode() {
  python3 -c 'import sys, urllib.parse; print(urllib.parse.quote(sys.argv[1], safe=""))' "$1"
}

normalize_branch_value() {
  local branch_value="$1"
  branch_value="${branch_value#refs/heads/}"
  branch_value="${branch_value#origin/}"
  printf '%s' "$branch_value"
}

build_trigger_url() {
  local job_name="$1"
  local token="$2"
  local base_url="${JENKINS_URL%/}/buildByToken/buildWithParameters"
  local url="${base_url}?job=$(urlencode "$job_name")&token=$(urlencode "$token")"
  local branch_value=""

  if [[ -n "$BRANCH_NAME" ]]; then
    branch_value="$BRANCH_NAME"
    branch_value="$(normalize_branch_value "$branch_value")"
    url="${url}&BuildBranch=$(urlencode "$branch_value")"
  fi
  if [[ -n "$BUILD_ENV_VALUE" ]]; then
    url="${url}&BuildEnv=$(urlencode "$BUILD_ENV_VALUE")"
  fi

  local param
  local key
  local value
  if (( ${#EXTRA_PARAMS[@]} > 0 )); then
    for param in "${EXTRA_PARAMS[@]}"; do
      [[ "$param" == *=* ]] || die "--param 必须是 KEY=VALUE 形式，当前: $param"
      key="${param%%=*}"
      value="${param#*=}"
      url="${url}&$(urlencode "$key")=$(urlencode "$value")"
    done
  fi

  printf '%s' "$url"
}

run_remote_jenkins_script() {
  local script="$1"
  shift || true
  ssh_exec env \
    "DEPLOY_JENKINS_HOME=${REMOTE_JENKINS_HOME_PATH}" \
    bash -s -- "$@" <<<"$script"
}

prepare_wait_backend() {
  if ! is_true "$WAIT_BUILD"; then
    return 0
  fi

  setup_jenkins_ssh_profile "${JENKINS_SSH_PROFILE:-mac_mini}"
  setup_jenkins_ssh

  if [[ -z "$REMOTE_JENKINS_HOME_PATH" ]]; then
    REMOTE_JENKINS_HOME_PATH="$(ssh_exec 'printf "%s\n" "$HOME/.jenkins"')"
  fi
}

read_next_build_number() {
  local job_name="$1"
  run_remote_jenkins_script "$(cat <<'EOF'
set -euo pipefail

job_name="$1"
file_path="${DEPLOY_JENKINS_HOME%/}/jobs/${job_name}/nextBuildNumber"
[[ -f "$file_path" ]] || { echo "找不到 nextBuildNumber: $file_path" >&2; exit 1; }
tr -d '\r\n' <"$file_path"
EOF
)" "$job_name"
}

read_build_status() {
  local build_xml="$1"
  python3 - "$build_xml" <<'PY'
import sys
import xml.etree.ElementTree as ET

root = ET.parse(sys.argv[1]).getroot()
building = (root.findtext("building", default="false") or "false").strip().lower()
result = (root.findtext("result", default="") or "").strip()
duration = (root.findtext("duration", default="0") or "0").strip()
display = (root.findtext("displayName", default="") or "").strip()
print(building)
print(result)
print(duration)
print(display)
PY
}

wait_build_result() {
  local job_name="$1"
  local build_number="$2"
  run_remote_jenkins_script "$(cat <<'EOF'
set -euo pipefail

job_name="$1"
build_number="$2"
timeout_seconds="$3"
build_dir="${DEPLOY_JENKINS_HOME%/}/jobs/${job_name}/builds/${build_number}"
build_xml="${build_dir}/build.xml"
build_log="${build_dir}/log"
start_ts="$(date +%s)"

read_build_status() {
  python3 - "$1" <<'PY'
import sys
import xml.etree.ElementTree as ET

root = ET.parse(sys.argv[1]).getroot()
building = (root.findtext("building", default="false") or "false").strip().lower()
result = (root.findtext("result", default="") or "").strip()
duration = (root.findtext("duration", default="0") or "0").strip()
display = (root.findtext("displayName", default="") or "").strip()
print(building)
print(result)
print(duration)
print(display)
PY
}

while [[ ! -f "$build_xml" ]]; do
  sleep 2
  now_ts="$(date +%s)"
  if (( now_ts - start_ts > timeout_seconds )); then
    echo "等待 Jenkins 生成构建记录超时: ${job_name} #${build_number}" >&2
    exit 1
  fi
done

while true; do
  status_output="$(read_build_status "$build_xml")"
  building="$(printf '%s\n' "$status_output" | sed -n '1p')"
  result="$(printf '%s\n' "$status_output" | sed -n '2p')"
  duration="$(printf '%s\n' "$status_output" | sed -n '3p')"
  display="$(printf '%s\n' "$status_output" | sed -n '4p')"
  [[ -n "$building" ]] || building="false"
  [[ -n "$duration" ]] || duration="0"
  [[ -n "$display" ]] || display="#${build_number}"
  if [[ "$building" != "true" && -n "$result" ]]; then
    echo "[INFO] 构建完成: ${job_name} ${display} result=${result} duration=${duration}ms"
    if [[ "$result" != "SUCCESS" ]]; then
      [[ -f "$build_log" ]] && tail -n 60 "$build_log" >&2 || true
      exit 1
    fi
    exit 0
  fi

  now_ts="$(date +%s)"
  if (( now_ts - start_ts > timeout_seconds )); then
    echo "[ERROR] 等待构建完成超时: ${job_name} ${display}" >&2
    [[ -f "$build_log" ]] && tail -n 40 "$build_log" >&2 || true
    exit 1
  fi
  sleep 5
done
EOF
)" "$job_name" "$build_number" "$WAIT_TIMEOUT_SECONDS"
}

trigger_job_once() {
  local job_name="$1"
  local token="$2"
  local next_build_number=""
  local trigger_url=""
  local http_code=""
  local response_file=""

  next_build_number="$(read_next_build_number "$job_name")"
  trigger_url="$(build_trigger_url "$job_name" "$token")"
  response_file="$(mktemp "/tmp/${job_name}.http_trigger.XXXXXX.out")"

  log_info "HTTP 触发 Jenkins job: ${job_name} -> #${next_build_number}"
  http_code="$(curl -sS -o "$response_file" -w '%{http_code}' "$trigger_url" || true)"
  case "$http_code" in
    200|201|202)
      ;;
    302)
      ;;
    *)
      cat "$response_file" >&2 || true
      rm -f "$response_file"
      die "HTTP 触发失败: ${job_name}，状态码=${http_code}"
      ;;
  esac
  rm -f "$response_file"

  if is_true "$WAIT_BUILD"; then
    wait_build_result "$job_name" "$next_build_number"
  fi
}

main() {
  parse_args "$@"
  require_cmd python3
  require_cmd curl
  prepare_wait_backend

  local manifest_json=""
  manifest_json="$(read_jobs_manifest)"
  manifest_json="$(filter_jobs_manifest "$manifest_json")"

  if [[ "$manifest_json" == "[]" ]]; then
    die "没有匹配到可用的 HTTP 触发 job"
  fi

  if [[ "$ACTION" == "list" ]]; then
    print_jobs_list "$manifest_json"
    return 0
  fi

  local failures=0
  while IFS=$'\t' read -r job_name job_type branch_default token; do
    [[ -n "$job_name" ]] || continue
    if ! trigger_job_once "$job_name" "$token" "$branch_default"; then
      failures=$((failures + 1))
    fi
  done < <(
    MANIFEST_JSON="$manifest_json" python3 - <<'PY'
import json
import os
import sys

payload = json.loads(os.environ["MANIFEST_JSON"])
for item in payload:
    print("\t".join([
        item["name"],
        item["type"],
        item["branch_parameter_default"],
        item["remote_trigger_token"],
    ]))
PY
  )

  if (( failures > 0 )); then
    die "存在 ${failures} 个 Jenkins job 触发或执行失败"
  fi
  log_info "所有目标 Jenkins job 已完成"
}

main "$@"
__TRIGGER_HTTP_BASH__
