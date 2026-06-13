#!/usr/bin/env bash
# shellcheck shell=bash

SECRET_BASE_VALIDATE_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SECRET_BASE_VALIDATE_DIR/common.sh"

sb_validate_render_input() {
  local secretctl_bin="${1:-}"
  local template_path="${2:-}"
  local bundle_path="${3:-}"
  local identity_path="${4:-}"
  local repo_name="${5:-}"
  local subproject_name="${6:-}"
  local env_name="${7:-}"

  [[ -x "$secretctl_bin" ]] || sb_die "secretctl 不可执行: $secretctl_bin"
  [[ -n "$template_path" ]] || sb_die "validate 缺少模板路径"
  [[ -n "$bundle_path" ]] || sb_die "validate 缺少 bundle 路径"
  [[ -n "$identity_path" ]] || sb_die "validate 缺少 identity 路径"
  [[ -n "$repo_name" ]] || sb_die "validate 缺少 repo"
  [[ -n "$subproject_name" ]] || sb_die "validate 缺少 subproject"
  [[ -n "$env_name" ]] || sb_die "validate 缺少 env"

  "$secretctl_bin" validate \
    --template "$template_path" \
    --bundle "$bundle_path" \
    --identity-file "$identity_path" \
    --expect-repo "$repo_name" \
    --expect-subproject "$subproject_name" \
    --expect-env "$env_name"
}
