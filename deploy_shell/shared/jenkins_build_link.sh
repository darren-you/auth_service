#!/usr/bin/env bash
# shellcheck shell=bash

jenkins_job_name_to_url_path() {
  local job_name="${1:-}"
  local path=""
  local segment=""
  local old_ifs="$IFS"

  [[ -n "$job_name" ]] || return 1

  IFS='/'
  read -r -a jenkins_job_segments <<< "$job_name"
  IFS="$old_ifs"

  for segment in "${jenkins_job_segments[@]}"; do
    [[ -n "$segment" ]] || continue
    path="${path}/job/${segment}"
  done

  [[ -n "$path" ]] || return 1
  printf '%s' "$path"
}

resolve_jenkins_public_url() {
  local value="${JENKINS_PUBLIC_URL:-${DEPLOY_JENKINS_URL:-${CICD_JENKINS_URL:-${JENKINS_URL_PUBLIC:-https://jenkins.xdarren.com}}}}"
  printf '%s' "${value%/}"
}

resolve_jenkins_build_base_url() {
  local configured_base="${JENKINS_BUILD_URL_BASE:-}"
  local configured_url="${JENKINS_BUILD_URL:-}"
  local job_name="${JOB_NAME:-${PROJECT_NAME_ANDROID_APP:-${PROJECT_NAME:-}}}"
  local public_url=""
  local job_path=""

  if [[ -n "$configured_base" ]]; then
    printf '%s' "${configured_base%/}"
    return 0
  fi

  if [[ -n "$configured_url" ]]; then
    printf '%s' "${configured_url%/}"
    return 0
  fi

  [[ -n "$job_name" ]] || return 0

  public_url="$(resolve_jenkins_public_url)"
  [[ -n "$public_url" ]] || return 0

  job_path="$(jenkins_job_name_to_url_path "$job_name" || true)"
  [[ -n "$job_path" ]] || return 0

  printf '%s%s' "$public_url" "$job_path"
}

resolve_jenkins_build_link() {
  local build_id="${1:-${BUILD_ID:-${BUILD_NUMBER:-}}}"
  local base_url=""

  if [[ -n "$build_id" ]]; then
    base_url="$(resolve_jenkins_build_base_url)"
    if [[ -n "$base_url" ]]; then
      printf '%s/%s/' "${base_url%/}" "$build_id"
      return 0
    fi
  fi

  if [[ -n "${BUILD_URL:-}" ]]; then
    printf '%s/' "${BUILD_URL%/}"
    return 0
  fi

  printf ''
}
