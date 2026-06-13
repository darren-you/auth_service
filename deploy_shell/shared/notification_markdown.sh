#!/usr/bin/env bash
# shellcheck shell=bash

notification_normalize_status() {
  case "${1:-}" in
    success|SUCCESS|Success)
      printf '%s' "success"
      ;;
    *)
      printf '%s' "failed"
      ;;
  esac
}

notification_status_icon() {
  if [[ "$(notification_normalize_status "${1:-}")" == "success" ]]; then
    printf '%s' "✅"
  else
    printf '%s' "❌"
  fi
}

notification_status_label() {
  if [[ "$(notification_normalize_status "${1:-}")" == "success" ]]; then
    printf '%s' "成功"
  else
    printf '%s' "失败"
  fi
}

notification_status_color() {
  if [[ "$(notification_normalize_status "${1:-}")" == "success" ]]; then
    printf '%s' "info"
  else
    printf '%s' "warning"
  fi
}

notification_status_rich_label() {
  printf '<font color="%s">%s</font>' \
    "$(notification_status_color "${1:-}")" \
    "$(notification_status_label "${1:-}")"
}

notification_stage_label() {
  case "${1:-build}" in
    deploy)
      printf '%s' "部署"
      ;;
    *)
      printf '%s' "构建"
      ;;
  esac
}

notification_type_label() {
  case "${1:-generic}" in
    ssl)
      printf '%s' "SSL"
      ;;
    web)
      printf '%s' "Web"
      ;;
    nginx)
      printf '%s' "Nginx"
      ;;
    server)
      printf '%s' "Server"
      ;;
    ios)
      printf '%s' "iOS"
      ;;
    macos)
      printf '%s' "macOS"
      ;;
    android)
      printf '%s' "Android"
      ;;
    *)
      printf '%s' "${1:-构建}"
      ;;
  esac
}

notification_stage_type_label() {
  local build_type="${1:-generic}"
  local stage="${2:-build}"
  printf '%s%s' "$(notification_type_label "$build_type")" "$(notification_stage_label "$stage")"
}

notification_append_emoji_line() {
  local var_name="$1"
  local emoji="$2"
  local title="$3"
  local detail="${4:-}"
  local current_content="${!var_name:-}"

  [[ -n "$title" ]] || return 0
  [[ -n "$detail" ]] || return 0

  if [[ -n "$current_content" ]]; then
    printf -v "$var_name" '%s\n%s %s: %s' "$current_content" "$emoji" "$title" "$detail"
  else
    printf -v "$var_name" '%s %s: %s' "$emoji" "$title" "$detail"
  fi
}

notification_append_emoji_link_line() {
  local var_name="$1"
  local emoji="$2"
  local title="$3"
  local link_text="${4:-查看详情}"
  local url="${5:-}"

  [[ -n "$url" ]] || return 0
  notification_append_emoji_line "$var_name" "$emoji" "$title" "[${link_text}](${url})"
}

notification_init_content() {
  local var_name="$1"
  local notify_status="${2:-}"
  local build_type="${3:-generic}"
  local stage="${4:-build}"
  local project_name="${5:-unknown}"
  local title=""

  title="**$(notification_status_icon "$notify_status") ${project_name} $(notification_stage_label "$stage")$(notification_status_label "$notify_status")**"
  printf -v "$var_name" '%s' "$title"
  notification_append_emoji_line "$var_name" "🧩" "构建类型" "$(notification_type_label "$build_type")"
  notification_append_emoji_line "$var_name" "🛠️" "阶段" "$(notification_stage_label "$stage")"
  notification_append_emoji_line "$var_name" "📊" "状态" "$(notification_status_rich_label "$notify_status")"
  notification_append_emoji_line "$var_name" "📦" "项目" "$project_name"
}

notification_append_line() {
  local var_name="$1"
  local label="$2"
  local value="${3:-}"
  local current_content="${!var_name:-}"

  [[ -n "$value" ]] || return 0

  if [[ -n "$current_content" ]]; then
    printf -v "$var_name" '%s\n%s：%s' "$current_content" "$label" "$value"
  else
    printf -v "$var_name" '%s：%s' "$label" "$value"
  fi
}

notification_append_heading() {
  local var_name="$1"
  local heading="${2:-}"
  local current_content="${!var_name:-}"

  [[ -n "$heading" ]] || return 0

  if [[ -n "$current_content" ]]; then
    printf -v "$var_name" '%s\n\n### %s' "$current_content" "$heading"
  else
    printf -v "$var_name" '### %s' "$heading"
  fi
}

notification_append_text() {
  local var_name="$1"
  local value="${2:-}"
  local current_content="${!var_name:-}"

  [[ -n "$value" ]] || return 0

  if [[ -n "$current_content" ]]; then
    printf -v "$var_name" '%s\n%s' "$current_content" "$value"
  else
    printf -v "$var_name" '%s' "$value"
  fi
}

notification_append_link_line() {
  local var_name="$1"
  local label="$2"
  local link_text="${3:-查看详情}"
  local url="${4:-}"

  [[ -n "$url" ]] || return 0
  notification_append_line "$var_name" "$label" "[${link_text}](${url})"
}

notification_json_escape() {
  local value="${1:-}"
  value=${value//\\/\\\\}
  value=${value//\"/\\\"}
  value=${value//$'\n'/\\n}
  value=${value//$'\r'/}
  value=${value//$'\t'/\\t}
  printf '%s' "$value"
}

notification_send_markdown() {
  local webhook_url="${1:-}"
  local content="${2:-}"
  local payload_content=""

  payload_content="$(notification_json_escape "$content")"

  curl -fsS -X POST "$webhook_url" \
    -H "Content-Type: application/json" \
    -d "{\"msgtype\":\"markdown\",\"markdown\":{\"content\":\"$payload_content\"}}" >/dev/null
}
