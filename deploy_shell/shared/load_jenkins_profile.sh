#!/usr/bin/env bash
# shellcheck shell=bash

apply_jenkins_profile_defaults() {
  local profile_name="${1:-}"
  local script_dir=""
  local profile_file=""

  if [[ -z "$profile_name" ]]; then
    echo "[ERROR] JENKINS_SSH_PROFILE 不能为空" >&2
    return 1
  fi

  script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
  profile_file="$script_dir/jenkins_profiles/${profile_name}.sh"
  if [[ ! -f "$profile_file" ]]; then
    echo "[ERROR] 找不到 Jenkins SSH profile: $profile_file" >&2
    return 1
  fi

  # shellcheck disable=SC1090
  source "$profile_file"
  if ! declare -F apply_jenkins_ssh_profile >/dev/null 2>&1; then
    echo "[ERROR] Jenkins SSH profile 缺少 apply_jenkins_ssh_profile 函数: $profile_file" >&2
    return 1
  fi

  apply_jenkins_ssh_profile
  unset -f apply_jenkins_ssh_profile
}
