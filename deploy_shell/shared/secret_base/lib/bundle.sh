#!/usr/bin/env bash
# shellcheck shell=bash

SECRET_BASE_BUNDLE_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SECRET_BASE_BUNDLE_DIR/common.sh"

sb_secretctl_exec() {
  local secretctl_dir
  secretctl_dir="$(sb_secretctl_dir)"
  (cd "$secretctl_dir" && go run -mod=vendor . "$@")
}

sb_encrypt_bundle() {
  local input_path="${1:-}"
  local output_path="${2:-}"
  local recipient_file="${3:-}"

  [[ -n "$input_path" ]] || sb_die "sb_encrypt_bundle 缺少明文 bundle 路径"
  [[ -n "$output_path" ]] || sb_die "sb_encrypt_bundle 缺少输出 bundle 路径"
  [[ -n "$recipient_file" ]] || sb_die "sb_encrypt_bundle 缺少 recipient 文件路径"

  sb_secretctl_exec encrypt --in "$input_path" --out "$output_path" --recipient-file "$recipient_file"
}

sb_decrypt_bundle() {
  local bundle_path="${1:-}"
  local identity_file="${2:-}"
  local output_path="${3:-}"

  [[ -n "$bundle_path" ]] || sb_die "sb_decrypt_bundle 缺少 bundle 路径"
  [[ -n "$identity_file" ]] || sb_die "sb_decrypt_bundle 缺少 identity 文件路径"

  if [[ -n "$output_path" ]]; then
    sb_secretctl_exec decrypt --bundle "$bundle_path" --identity-file "$identity_file" --out "$output_path"
    return 0
  fi

  sb_secretctl_exec decrypt --bundle "$bundle_path" --identity-file "$identity_file"
}
