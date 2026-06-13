#!/usr/bin/env bash
set -euo pipefail

export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:$PATH"

DEFAULT_KEYCHAIN_PATH="$HOME/Library/Keychains/ci_codesign.keychain-db"
DEFAULT_TIMEOUT_SECONDS=21600
DEFAULT_P12_PASSWORD="15882582967@yxd"
SYSTEM_KEYCHAIN_PATH="/Library/Keychains/System.keychain"

COMMAND="prepare"
IDENTITY=""
KEYCHAIN_PATH="$DEFAULT_KEYCHAIN_PATH"
KEYCHAIN_PASSWORD="${KEYCHAIN_PASSWORD:-}"
P12_PATH=""
P12_PASSWORD="${P12_PASSWORD:-$DEFAULT_P12_PASSWORD}"
TIMEOUT_SECONDS="$DEFAULT_TIMEOUT_SECONDS"
SET_DEFAULT_KEYCHAIN=true
INCLUDE_LOGIN_KEYCHAIN=true

log() {
  echo "[INFO] $*"
}

warn() {
  echo "[WARN] $*" >&2
}

err() {
  echo "[ERROR] $*" >&2
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1
}

require_cmd() {
  if ! need_cmd "$1"; then
    err "缺少命令: $1"
    exit 1
  fi
}

ensure_macos() {
  if [ "$(uname -s)" != "Darwin" ]; then
    err "该脚本仅支持 macOS"
    exit 1
  fi
}

expand_path() {
  local path="$1"

  case "$path" in
    "~")
      printf '%s\n' "$HOME"
      ;;
    "~/"*)
      printf '%s\n' "$HOME/${path#~/}"
      ;;
    /*)
      printf '%s\n' "$path"
      ;;
    *)
      printf '%s\n' "$PWD/$path"
      ;;
  esac
}

trim_quotes_and_space() {
  local value="$1"

  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  value="${value#\"}"
  value="${value%\"}"
  printf '%s\n' "$value"
}

append_unique_keychain() {
  local candidate="$1"
  local existing

  if [ -z "$candidate" ] || [ ! -e "$candidate" ]; then
    return 0
  fi

  for existing in "${SEARCH_KEYCHAINS[@]:-}"; do
    if [ "$existing" = "$candidate" ]; then
      return 0
    fi
  done

  SEARCH_KEYCHAINS+=("$candidate")
}

build_user_search_list() {
  local login_keychain="$HOME/Library/Keychains/login.keychain-db"
  local raw_path=""
  local normalized=""

  SEARCH_KEYCHAINS=()
  append_unique_keychain "$KEYCHAIN_PATH"

  if [ "$INCLUDE_LOGIN_KEYCHAIN" = true ] && [ "$login_keychain" != "$KEYCHAIN_PATH" ]; then
    append_unique_keychain "$login_keychain"
  fi

  while IFS= read -r raw_path; do
    normalized="$(trim_quotes_and_space "$raw_path")"
    if [ "$normalized" = "$SYSTEM_KEYCHAIN_PATH" ]; then
      continue
    fi
    append_unique_keychain "$normalized"
  done <<EOF
$(security list-keychains -d user 2>/dev/null || true)
EOF
}

apply_user_search_list() {
  if [ "${#SEARCH_KEYCHAINS[@]}" -eq 0 ]; then
    err "未生成可用的 user keychain search list"
    exit 1
  fi

  security list-keychains -d user -s "${SEARCH_KEYCHAINS[@]}"
}

ensure_keychain_exists() {
  local parent_dir

  if [ -e "$KEYCHAIN_PATH" ]; then
    return 0
  fi

  parent_dir="$(dirname "$KEYCHAIN_PATH")"
  mkdir -p "$parent_dir"
  log "创建专用 keychain: $KEYCHAIN_PATH"
  security create-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN_PATH"
}

unlock_target_keychain() {
  log "解锁 keychain: $KEYCHAIN_PATH"
  security unlock-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN_PATH"
  security set-keychain-settings -lut "$TIMEOUT_SECONDS" "$KEYCHAIN_PATH"
}

maybe_set_default_keychain() {
  if [ "$SET_DEFAULT_KEYCHAIN" = true ]; then
    security default-keychain -s "$KEYCHAIN_PATH"
  fi
}

identity_filter_sha1() {
  if printf '%s' "$IDENTITY" | grep -Eq '^[A-Fa-f0-9]{40}$'; then
    printf '%s' "$IDENTITY" | tr '[:lower:]' '[:upper:]'
  fi
}

list_matching_identities() {
  local keychain="$1"
  local filter_sha=""
  local line=""
  local sha=""
  local label=""

  filter_sha="$(identity_filter_sha1 || true)"

  while IFS= read -r line; do
    case "$line" in
      *"valid identities found"*)
        continue
        ;;
    esac

    sha="$(printf '%s\n' "$line" | awk '{print $2}')"
    label="$(printf '%s\n' "$line" | sed -n 's/^[[:space:]]*[0-9][0-9]*) [A-Fa-f0-9]\{40\} "\(.*\)".*/\1/p')"

    if [ -z "$sha" ] || [ -z "$label" ]; then
      continue
    fi

    if [ -n "$filter_sha" ]; then
      if [ "$sha" = "$filter_sha" ]; then
        printf '%s|%s\n' "$sha" "$label"
      fi
      continue
    fi

    if [ "$label" = "$IDENTITY" ] || printf '%s\n' "$label" | grep -F -- "$IDENTITY" >/dev/null 2>&1; then
      printf '%s|%s\n' "$sha" "$label"
    fi
  done <<EOF
$(security find-identity -v -p codesigning "$keychain" 2>/dev/null || true)
EOF
}

resolve_identity_record() {
  local keychain="$1"
  local matches=""
  local count="0"

  matches="$(list_matching_identities "$keychain" || true)"
  if [ -z "$matches" ]; then
    return 1
  fi

  count="$(printf '%s\n' "$matches" | awk 'NF { count += 1 } END { print count + 0 }')"
  if [ "$count" -ne 1 ]; then
    err "identity 匹配结果不唯一，请传入完整名称或 40 位 SHA1"
    printf '%s\n' "$matches" >&2
    return 2
  fi

  printf '%s\n' "$matches"
}

identity_exists_in_keychain() {
  local keychain="$1"

  if resolve_identity_record "$keychain" >/dev/null 2>&1; then
    return 0
  fi

  return 1
}

import_p12_if_needed() {
  local target_record=""

  if [ -z "$P12_PATH" ]; then
    return 0
  fi

  if [ ! -f "$P12_PATH" ]; then
    err "未找到 p12 文件: $P12_PATH"
    exit 1
  fi

  if [ -z "$P12_PASSWORD" ]; then
    err "传入了 --p12，但缺少 --p12-password 或环境变量 P12_PASSWORD"
    exit 1
  fi

  target_record="$(resolve_identity_record "$KEYCHAIN_PATH" 2>/dev/null || true)"
  if [ -n "$target_record" ]; then
    log "目标 keychain 已存在匹配 identity，跳过 p12 导入"
    return 0
  fi

  log "导入 p12 到目标 keychain"
  security import "$P12_PATH" \
    -k "$KEYCHAIN_PATH" \
    -P "$P12_PASSWORD" \
    -f pkcs12 \
    -T /usr/bin/codesign \
    -T /usr/bin/security \
    -T /usr/bin/productbuild \
    -T /usr/bin/productsign \
    -T /usr/bin/xcodebuild
}

ensure_target_identity_present() {
  local system_record=""

  if identity_exists_in_keychain "$KEYCHAIN_PATH"; then
    return 0
  fi

  system_record="$(resolve_identity_record "$SYSTEM_KEYCHAIN_PATH" 2>/dev/null || true)"
  if [ -n "$system_record" ]; then
    err "目标 identity 当前只在 System.keychain 中可见，Jenkins 无头构建会继续弹管理员授权框"
    err "请先导出对应的 .p12，并通过 --p12 / --p12-password 导入专用用户 keychain 后重试"
    printf '%s\n' "$system_record" >&2
    exit 1
  fi

  err "目标 keychain 中未找到 identity: $IDENTITY"
  exit 1
}

refresh_key_partition_list() {
  log "刷新 key partition list"
  security set-key-partition-list \
    -S apple-tool:,apple:,codesign: \
    -s \
    -k "$KEYCHAIN_PASSWORD" \
    "$KEYCHAIN_PATH" >/dev/null
}

warn_if_identity_exists_in_system_keychain() {
  local system_record=""

  system_record="$(resolve_identity_record "$SYSTEM_KEYCHAIN_PATH" 2>/dev/null || true)"
  if [ -n "$system_record" ]; then
    warn "检测到目标 identity 在 System.keychain 中也存在。当前脚本已把签名链路切到用户 keychain，但旧流水线如果绕过该脚本，仍可能再次命中 System.keychain。"
    printf '%s\n' "$system_record" >&2
  fi
}

verify_noninteractive_codesign() {
  local resolved=""
  local identity_sha=""
  local identity_name=""
  local probe_dir=""
  local probe_binary=""

  resolved="$(resolve_identity_record "$KEYCHAIN_PATH")"
  identity_sha="${resolved%%|*}"
  identity_name="${resolved#*|}"

  probe_dir="$(mktemp -d /tmp/codesign_probe.XXXXXX)"
  probe_binary="$probe_dir/true_probe"

  trap 'rm -rf "$probe_dir"' EXIT
  cp /usr/bin/true "$probe_binary"
  chmod u+w "$probe_binary"

  log "验证无头 codesign: $identity_name"
  codesign --force --keychain "$KEYCHAIN_PATH" --sign "$identity_sha" "$probe_binary" >/dev/null
  codesign --verify --verbose=2 "$probe_binary" >/dev/null

  rm -rf "$probe_dir"
  trap - EXIT
}

print_status() {
  local target_matches=""
  local system_matches=""

  echo "DEFAULT_KEYCHAIN=$(security default-keychain | sed 's/[[:space:]]*\"//g; s/\"//g')"
  echo "USER_KEYCHAINS="
  security list-keychains -d user | sed 's/^/  /'
  echo "TARGET_KEYCHAIN=$KEYCHAIN_PATH"

  target_matches="$(list_matching_identities "$KEYCHAIN_PATH" || true)"
  system_matches="$(list_matching_identities "$SYSTEM_KEYCHAIN_PATH" || true)"

  echo "TARGET_MATCHES="
  if [ -n "$target_matches" ]; then
    printf '%s\n' "$target_matches" | sed 's/^/  /'
  else
    echo "  <none>"
  fi

  echo "SYSTEM_MATCHES="
  if [ -n "$system_matches" ]; then
    printf '%s\n' "$system_matches" | sed 's/^/  /'
  else
    echo "  <none>"
  fi
}

validate_required_flags() {
  case "$COMMAND" in
    prepare|verify)
      if [ -z "$IDENTITY" ]; then
        err "缺少 --identity"
        exit 1
      fi
      if [ -z "$KEYCHAIN_PASSWORD" ]; then
        err "缺少 --keychain-password 或环境变量 KEYCHAIN_PASSWORD"
        exit 1
      fi
      ;;
  esac
}

usage() {
  cat <<'EOF'
用法：
  bash deploy_shell/deploy_swift_app/prepare_codesign_keychain.sh [command] [flags]

命令：
  prepare    创建或修复专用签名 keychain，并验证 codesign 可无头工作（默认）
  verify     仅验证现有 keychain / identity 的无头签名能力
  status     打印当前 default keychain、user search list 与 identity 分布
  help       显示帮助

核心 flags：
  --identity <name|sha1>             代码签名 identity 名称或 40 位 SHA1
  --keychain <path>                  目标 keychain 路径，默认 ~/Library/Keychains/ci_codesign.keychain-db
  --keychain-password <password>     目标 keychain 密码，也可用环境变量 KEYCHAIN_PASSWORD
  --p12 <path>                       可选，待导入的 p12 文件
  --p12-password <password>          可选，p12 密码；省略时默认使用当前 Jenkins 统一导入密码
  --timeout-seconds <seconds>        keychain 自动锁定超时秒数，默认 21600
  --no-set-default-keychain          不修改 default keychain
  --no-include-login-keychain        user search list 中不保留 login keychain
  -h, --help                         显示帮助
EOF
}

parse_args() {
  if [ "$#" -gt 0 ]; then
    case "$1" in
      prepare|verify|status|help)
        COMMAND="$1"
        shift
        ;;
    esac
  fi

  while [ "$#" -gt 0 ]; do
    case "$1" in
      --identity)
        IDENTITY="${2:-}"
        shift 2
        ;;
      --keychain)
        KEYCHAIN_PATH="${2:-}"
        shift 2
        ;;
      --keychain-password)
        KEYCHAIN_PASSWORD="${2:-}"
        shift 2
        ;;
      --p12)
        P12_PATH="${2:-}"
        shift 2
        ;;
      --p12-password)
        P12_PASSWORD="${2:-}"
        shift 2
        ;;
      --timeout-seconds)
        TIMEOUT_SECONDS="${2:-}"
        shift 2
        ;;
      --no-set-default-keychain)
        SET_DEFAULT_KEYCHAIN=false
        shift
        ;;
      --no-include-login-keychain)
        INCLUDE_LOGIN_KEYCHAIN=false
        shift
        ;;
      -h|--help)
        COMMAND="help"
        shift
        ;;
      *)
        err "未知参数: $1"
        usage
        exit 1
        ;;
    esac
  done
}

prepare_runtime() {
  ensure_macos
  require_cmd security
  require_cmd codesign
  require_cmd xcodebuild
  require_cmd awk
  require_cmd sed
  require_cmd grep
  require_cmd mktemp
  require_cmd cp

  KEYCHAIN_PATH="$(expand_path "$KEYCHAIN_PATH")"
  if [ -n "$P12_PATH" ]; then
    P12_PATH="$(expand_path "$P12_PATH")"
  fi

  validate_required_flags
}

run_prepare() {
  ensure_keychain_exists
  unlock_target_keychain
  build_user_search_list
  apply_user_search_list
  maybe_set_default_keychain
  import_p12_if_needed
  ensure_target_identity_present
  refresh_key_partition_list
  warn_if_identity_exists_in_system_keychain
  verify_noninteractive_codesign

  log "codesign keychain 预处理完成"
}

run_verify() {
  unlock_target_keychain
  ensure_target_identity_present
  refresh_key_partition_list
  warn_if_identity_exists_in_system_keychain
  verify_noninteractive_codesign
  log "无头 codesign 校验通过"
}

main() {
  parse_args "$@"

  case "$COMMAND" in
    prepare)
      prepare_runtime
      run_prepare
      ;;
    verify)
      prepare_runtime
      run_verify
      ;;
    status)
      ensure_macos
      KEYCHAIN_PATH="$(expand_path "$KEYCHAIN_PATH")"
      print_status
      ;;
    help)
      usage
      ;;
    *)
      err "未知命令: $COMMAND"
      usage
      exit 1
      ;;
  esac
}

main "$@"
