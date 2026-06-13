#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_CONFIG_FILE=""

validate_project_name() {
  local value="${1:-}"
  [[ -n "$value" ]] || {
    echo "[ERROR] PROJECT_NAME_ANDROID_APP 不能为空，请在 deploy_config.sh 中定义。" >&2
    exit 1
  }
  [[ "$value" =~ ^[a-z0-9]+(_[a-z0-9]+)*_android_app$ ]] || {
    echo "[ERROR] PROJECT_NAME_ANDROID_APP 不符合命名规范: ${value}" >&2
    exit 1
  }
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --config)
      [[ $# -ge 2 ]] || {
        echo "[ERROR] --config 缺少路径参数" >&2
        exit 1
      }
      DEPLOY_CONFIG_FILE="$2"
      shift 2
      ;;
    *)
      echo "[ERROR] 未知参数: $1" >&2
      exit 1
      ;;
  esac
done

[[ -n "$DEPLOY_CONFIG_FILE" && -f "$DEPLOY_CONFIG_FILE" ]] || {
  echo "[ERROR] 必须传入 --config <deploy_config.sh绝对路径>" >&2
  exit 1
}

CONFIG_DIR="$(cd -- "$(dirname -- "${DEPLOY_CONFIG_FILE}")" && pwd)"
DEPLOY_CONFIG_FILE="${CONFIG_DIR}/$(basename -- "${DEPLOY_CONFIG_FILE}")"
export DEPLOY_CONFIG_FILE
# shellcheck disable=SC1090
source "${DEPLOY_CONFIG_FILE}"

PROJECT_NAME="${PROJECT_NAME_ANDROID_APP:-}"
validate_project_name "${PROJECT_NAME}"

if [[ -n "${PROJECT_ROOT:-}" ]]; then
  case "${PROJECT_ROOT}" in
    /*) ;;
    *) PROJECT_ROOT="${CONFIG_DIR}/${PROJECT_ROOT}" ;;
  esac
else
  PROJECT_ROOT="${CONFIG_DIR}"
fi

PROJECT_ROOT="$(cd -- "${PROJECT_ROOT}" && pwd)"
cd "${PROJECT_ROOT}"

app_name="${APP_NAME:-${PROJECT_NAME}}"
module_name="${ANDROID_APP_MODULE_NAME:-app}"
gradlew_path="${ANDROID_GRADLEW_PATH:-./gradlew}"
version_file="${ANDROID_VERSION_FILE:-${module_name}/version.properties}"
local_build_dir="${BUILD_OUTPUT_ROOT:-${LOCAL_BUILD_DIR:-/Users/darrenyou/BuildPackages/app}}"
remote="${BuildBranch:-origin/master}"
environment="${BuildEnv:-test}"
build_channel="${BuildChannel:-${ANDROID_DEFAULT_CHANNEL:-official}}"
build_artifact="${BuildArtifact:-${ANDROID_DEFAULT_ARTIFACT:-apk}}"
channels_config="${ANDROID_BUILD_CHANNELS:-official}"
build_id="${BUILD_NUMBER:-}"
artifact_timestamp=""
artifact_path=""
version=""
version_code=""
build_error_msg=""

ensure_git_commit_identity() {
  local default_name default_email
  default_name="$(git config --global --get user.name 2>/dev/null || true)"
  default_email="$(git config --global --get user.email 2>/dev/null || true)"
  [[ -n "${default_email}" ]] || {
    echo "[ERROR] 未检测到 Git user.email，请先配置 Jenkins 机器 Git 身份" >&2
    exit 1
  }
  [[ -n "${default_name}" ]] || default_name="${USER:-jenkins}"
  git config user.name "${default_name}"
  git config user.email "${default_email}"
}

resolve_branch_name() {
  local ref="${1:-origin/master}"
  ref="${ref#refs/heads/}"
  ref="${ref#origin/}"
  printf '%s\n' "${ref}"
}

read_version_file() {
  [[ -f "${version_file}" ]] || {
    echo "[ERROR] 找不到 Android 版本文件: ${version_file}" >&2
    exit 1
  }

  version="$(grep '^VERSION_NAME=' "${version_file}" | sed 's/^VERSION_NAME=//')"
  version_code="$(grep '^VERSION_CODE=' "${version_file}" | sed 's/^VERSION_CODE=//')"
  [[ -n "${version}" ]] || {
    echo "[ERROR] ${version_file} 缺少 VERSION_NAME" >&2
    exit 1
  }
  [[ "${version_code}" =~ ^[0-9]+$ ]] || {
    echo "[ERROR] ${version_file} 缺少合法的 VERSION_CODE" >&2
    exit 1
  }
}

write_version_file() {
  cat > "${version_file}" <<EOF
VERSION_NAME=${version}
VERSION_CODE=${version_code}
EOF
}

update_build_version() {
  local branch
  branch="$(resolve_branch_name "${remote}")"

  git fetch origin "${branch}" >/dev/null 2>&1 || true
  if git show-ref --verify --quiet "refs/heads/${branch}"; then
    git checkout "${branch}" >/dev/null 2>&1
  elif git show-ref --verify --quiet "refs/remotes/origin/${branch}"; then
    git checkout -B "${branch}" "origin/${branch}" >/dev/null 2>&1
  else
    git checkout -B "${branch}" >/dev/null 2>&1
  fi

  git pull --ff-only origin "${branch}" >/dev/null 2>&1 || true
  read_version_file
  version_code="$((version_code + 1))"
  write_version_file

  git add "${version_file}"
  if git diff --cached --quiet; then
    echo "⏭️ 没有需要提交的版本号变更"
    return
  fi

  ensure_git_commit_identity
  git commit -m "修改Build号为: ${version_code}" >/dev/null
  git push --set-upstream origin "${branch}"
}

send_notification() {
  local build_status="$1"
  local channel="$2"
  local commit_message=""
  if command -v git >/dev/null 2>&1 && git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    commit_message="$(git log -1 --pretty=%s 2>/dev/null || true)"
  fi
  local cmd=(
    bash "${SCRIPT_DIR}/notification_android.sh"
    --config "${DEPLOY_CONFIG_FILE}"
    --version "${version}"
    --version_code "${version_code}"
    --branch_name "${remote}"
    --build_id "${build_id}"
    --artifact_path "${artifact_path}"
    --channel "${channel}"
    --artifact_kind "${build_artifact}"
    --build_status "${build_status}"
    --environment "${environment}"
  )
  if [[ -n "${commit_message}" ]]; then
    cmd+=(--commit_message "${commit_message}")
  fi
  if [[ "${build_status}" != "success" && -n "${build_error_msg}" ]]; then
    cmd+=(--error "${build_error_msg}")
  fi
  "${cmd[@]}"
}

fail_build() {
  build_error_msg="$1"
  echo "❌ ${build_error_msg}" >&2
  send_notification "failure" "${2:-${build_channel}}"
  exit 1
}

to_pascal_case() {
  local input="${1:-}"
  local result="" part first_char rest
  IFS='_-'
  read -r -a parts <<< "${input}"
  unset IFS
  for part in "${parts[@]}"; do
    [[ -n "${part}" ]] || continue
    first_char="${part:0:1}"
    rest="${part:1}"
    result+="${first_char^^}${rest}"
  done
  printf '%s\n' "${result}"
}

find_artifact_path() {
  local channel="$1"
  local extension="$2"
  find "${PROJECT_ROOT}/${module_name}/build/outputs" -type f \
    \( -name "*${channel}*release*.${extension}" -o -name "*release*.${extension}" \) \
    | sort | tail -n 1
}

build_single_channel() {
  local channel="$1"
  local extension task_name output_path version_dir file_name variant_name

  [[ "${build_artifact}" == "apk" || "${build_artifact}" == "aab" ]] || {
    fail_build "BuildArtifact 仅支持 apk 或 aab" "${channel}"
  }

  [[ -x "${gradlew_path}" ]] || chmod +x "${gradlew_path}"
  variant_name="$(to_pascal_case "${channel}")Release"

  if [[ "${build_artifact}" == "aab" ]]; then
    task_name="bundle${variant_name}"
    extension="aab"
  else
    task_name="assemble${variant_name}"
    extension="apk"
  fi

  "${gradlew_path}" ":${module_name}:${task_name}" "-PbuildEnv=${environment}" --no-daemon \
    || fail_build "Android ${build_artifact^^} 构建失败" "${channel}"

  output_path="$(find_artifact_path "${channel}" "${extension}")"
  [[ -n "${output_path}" && -f "${output_path}" ]] || {
    fail_build "未找到 Android 产物（channel=${channel}, ext=${extension}）" "${channel}"
  }

  artifact_timestamp="$(date +%y%m%d%H%M)"
  version_dir="${local_build_dir}/${app_name}/android/${build_artifact}/${version}"
  mkdir -p "${version_dir}"
  file_name="${app_name}_${channel}_${build_artifact}_V${version}_${version_code}_${environment}_${artifact_timestamp}.${extension}"
  artifact_path="${version_dir}/${file_name}"
  rm -f "${version_dir}/${app_name}_${channel}_${build_artifact}_V${version}_*_${environment}_*.${extension}"
  cp "${output_path}" "${artifact_path}"
  send_notification "success" "${channel}"
}

main() {
  local channel channel_found

  IFS=',' read -r -a available_channels <<< "${channels_config}"
  update_build_version
  read_version_file

  if [[ "${build_channel}" == "all" ]]; then
    for channel in "${available_channels[@]}"; do
      channel="$(echo "${channel}" | xargs)"
      [[ -n "${channel}" ]] || continue
      build_single_channel "${channel}"
    done
    exit 0
  fi

  channel_found="false"
  for channel in "${available_channels[@]}"; do
    channel="$(echo "${channel}" | xargs)"
    if [[ "${channel}" == "${build_channel}" ]]; then
      channel_found="true"
      break
    fi
  done
  [[ "${channel_found}" == "true" ]] || fail_build "无效的 BuildChannel: ${build_channel}" "${build_channel}"
  build_single_channel "${build_channel}"
}

main "$@"
