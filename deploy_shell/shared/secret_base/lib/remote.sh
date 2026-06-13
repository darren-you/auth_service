#!/usr/bin/env bash
# shellcheck shell=bash

if [[ "${SECRET_BASE_REMOTE_SH_LOADED:-0}" == "1" ]]; then
  return 0
fi
readonly SECRET_BASE_REMOTE_SH_LOADED=1

SECRET_BASE_REMOTE_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "$SECRET_BASE_REMOTE_DIR/common.sh"

readonly SECRETCTL_DOCKER_BUILDER_IMAGE="golang:1.25.6-alpine3.22"

sb_resolve_go_bin() {
  if command -v go >/dev/null 2>&1; then
    command -v go
    return 0
  fi

  local candidate=""
  for candidate in /opt/homebrew/bin/go /usr/local/bin/go /usr/local/go/bin/go; do
    if [[ -x "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done

  return 1
}

sb_build_secretctl_linux_binary() {
  local output_path="${1:-}"
  local secretctl_dir
  local go_bin=""

  [[ -n "$output_path" ]] || sb_die "sb_build_secretctl_linux_binary 缺少输出路径"
  secretctl_dir="$(sb_secretctl_dir)"
  mkdir -p "$(dirname -- "$output_path")"

  if go_bin="$(sb_resolve_go_bin)"; then
    sb_log_info "使用本机 Go 构建 Linux secretctl"
    (
      cd "$secretctl_dir"
      CGO_ENABLED=0 GOOS=linux GOARCH=amd64 "$go_bin" build -mod=vendor -trimpath -o "$output_path" .
    )
    chmod +x "$output_path"
    return 0
  fi

  if command -v docker >/dev/null 2>&1; then
    local output_dir output_name
    output_dir="$(cd -- "$(dirname -- "$output_path")" && pwd)"
    output_name="$(basename -- "$output_path")"
    sb_log_info "本机未安装 Go，回退到 Docker 构建 Linux secretctl"
    docker run --rm \
      -v "$secretctl_dir:/src" \
      -v "$output_dir:/out" \
      -w /src \
      -e PATH=/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
      "$SECRETCTL_DOCKER_BUILDER_IMAGE" \
      sh -c "CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -trimpath -o /out/$output_name ."
    chmod +x "$output_path"
    return 0
  fi

  sb_die "构建 secretctl 需要 go 或 docker"
}

sb_create_remote_temp_dir() {
  [[ ${#SSH_CMD[@]} -gt 0 ]] || sb_die "SSH_CMD 未初始化，无法创建远端临时目录"
  "${SSH_CMD[@]}" "mktemp -d /tmp/workspace_secret_base.XXXXXX"
}

sb_upload_remote_artifacts() {
  local remote_dir="${1:-}"
  shift || true

  [[ -n "$remote_dir" ]] || sb_die "上传远端工件时缺少 remote_dir"
  [[ ${#SCP_CMD[@]} -gt 0 ]] || sb_die "SCP_CMD 未初始化，无法上传远端工件"
  [[ "$#" -gt 0 ]] || sb_die "上传远端工件时缺少本地文件"

  "${SCP_CMD[@]}" "$@" "${TARGET_HOST}:${remote_dir}/"
}
