#!/usr/bin/env bash
# shellcheck shell=bash

SECRET_BASE_RENDER_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SECRET_BASE_RENDER_DIR/common.sh"

sb_render_runtime_config() {
  local secretctl_bin="${1:-}"
  local template_path="${2:-}"
  local bundle_path="${3:-}"
  local identity_path="${4:-}"
  local output_path="${5:-}"
  local audit_log_path="${6:-}"
  local operator_name="${7:-unknown}"
  local repo_name="${8:-}"
  local subproject_name="${9:-}"
  local env_name="${10:-}"

  [[ -x "$secretctl_bin" ]] || sb_die "secretctl 不可执行: $secretctl_bin"
  [[ -n "$template_path" ]] || sb_die "render 缺少模板路径"
  [[ -n "$bundle_path" ]] || sb_die "render 缺少 bundle 路径"
  [[ -n "$identity_path" ]] || sb_die "render 缺少 identity 路径"
  [[ -n "$output_path" ]] || sb_die "render 缺少输出路径"
  [[ -n "$repo_name" ]] || sb_die "render 缺少 repo"
  [[ -n "$subproject_name" ]] || sb_die "render 缺少 subproject"
  [[ -n "$env_name" ]] || sb_die "render 缺少 env"

  "$secretctl_bin" render \
    --template "$template_path" \
    --bundle "$bundle_path" \
    --identity-file "$identity_path" \
    --out "$output_path" \
    --expect-repo "$repo_name" \
    --expect-subproject "$subproject_name" \
    --expect-env "$env_name" \
    --audit-log "$audit_log_path" \
    --operator "$operator_name"
}
