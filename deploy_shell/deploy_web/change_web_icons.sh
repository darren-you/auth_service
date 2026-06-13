#!/usr/bin/env bash
# shellcheck shell=bash

# =============================================================================
# Web 图标生成脚本
# 用途:
#   从一张 1024x1024 源图自动生成并覆盖 web 项目 public/assets/icons 下的常用图标。
#
# 配置来自：`项目根目录/deploy_config.sh`（通过 `--config` 传入）
# 依赖 deploy_config.sh 配置项：
#   - WEB_ICON_IMAGE_1024（必填）：1024x1024 源图路径（相对项目根目录或绝对路径）
#
# 生成文件：
#   - apple-touch-icon.png   (180x180)
#   - favicon-16x16.png      (16x16)
#   - favicon-32x32.png      (32x32)
#   - favicon-48x48.png      (48x48)
#   - icon-28x28.png         (28x28)
#   - icon-108x108.png       (108x108)
#   - icon-192x192.png       (192x192)
#   - icon-512x512.png       (512x512)
#   - icon-1024x1024.png     (1024x1024)
#
# 说明：
#   - 仅生成 PNG 图标，不会改动 favicon.svg / safari-pinned-tab.svg。
# =============================================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

usage() {
  cat <<'EOF'
用法:
  change_web_icons.sh --config <deploy_config.sh绝对路径>

说明:
  - 必须通过 --config 指定外部项目配置文件
  - 请在 `deploy_config.sh` 中配置：
      WEB_ICON_IMAGE_1024="deploy_assets/web/icon/icon_1024.png"      # 相对项目根目录或绝对路径
EOF
}

require_file() {
  local p="$1"
  local desc="$2"
  if [ -z "$p" ] || [ ! -f "$p" ]; then
    echo -e "${RED}错误: 找不到 ${desc}: ${p}${NC}"
    exit 1
  fi
}

resolve_asset_path() {
  local p="$1"
  if [ -z "$p" ]; then
    echo ""
    return 0
  fi
  if [[ "$p" = /* ]]; then
    echo "$p"
    return 0
  fi
  echo "${PROJECT_ROOT%/}/$p"
}

read_image_size() {
  local image_path="$1"
  local field="$2"
  sips -g "$field" "$image_path" 2>/dev/null | awk -v key="$field" '$1 == key ":" { print $2; exit }'
}

generate_icon() {
  local filename="$1"
  local size="$2"
  local target="${ICONS_DIR}/${filename}"

  echo -e "  - ${GREEN}${filename}${NC} (${size}x${size})"
  if ! sips -s format png -z "$size" "$size" "$SOURCE_ICON" --out "$target" >/dev/null 2>&1; then
    echo -e "${RED}错误: 生成图标失败: ${filename}${NC}"
    exit 1
  fi
}

DEPLOY_CONFIG_FILE="${DEPLOY_CONFIG_FILE:-}"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --config)
      [[ $# -ge 2 ]] || { echo "错误: --config 缺少路径参数"; exit 1; }
      DEPLOY_CONFIG_FILE="$2"
      shift 2
      ;;
    *)
      echo "错误: 未知参数 $1"
      usage
      exit 1
      ;;
  esac
done

if [[ -z "${DEPLOY_CONFIG_FILE}" ]]; then
  echo "错误: 必须传入 --config <deploy_config.sh绝对路径>"
  usage
  exit 1
fi

if [[ ! -f "${DEPLOY_CONFIG_FILE}" ]]; then
  echo "错误: 找不到配置文件 deploy_config.sh"
  echo "路径: ${DEPLOY_CONFIG_FILE}"
  exit 1
fi

PROJECT_ROOT="$(cd -- "$(dirname -- "${DEPLOY_CONFIG_FILE}")" && pwd)"
ICONS_DIR="${PROJECT_ROOT}/public/assets/icons"

unset WEB_ICON_IMAGE_1024

# shellcheck disable=SC1090
source "${DEPLOY_CONFIG_FILE}"

if [ -z "${WEB_ICON_IMAGE_1024:-}" ]; then
  echo "错误: deploy_config.sh 未配置 WEB_ICON_IMAGE_1024"
  exit 1
fi

if ! command -v sips >/dev/null 2>&1; then
  echo -e "${RED}错误: 未找到 sips，无法生成图标尺寸${NC}"
  exit 1
fi

SOURCE_ICON="$(resolve_asset_path "$WEB_ICON_IMAGE_1024")"
require_file "$SOURCE_ICON" "WEB_ICON_IMAGE_1024"

SOURCE_WIDTH="$(read_image_size "$SOURCE_ICON" "pixelWidth")"
SOURCE_HEIGHT="$(read_image_size "$SOURCE_ICON" "pixelHeight")"

if [[ "$SOURCE_WIDTH" != "1024" || "$SOURCE_HEIGHT" != "1024" ]]; then
  echo -e "${RED}错误: 源图尺寸必须是 1024x1024，当前为 ${SOURCE_WIDTH}x${SOURCE_HEIGHT}${NC}"
  exit 1
fi

mkdir -p "$ICONS_DIR"

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}  Web 图标生成工具${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""
echo -e "项目根目录: ${GREEN}${PROJECT_ROOT}${NC}"
echo -e "源图:       ${GREEN}${SOURCE_ICON}${NC}"
echo -e "输出目录:   ${GREEN}${ICONS_DIR}${NC}"
echo ""

echo -e "${BLUE}开始生成 web 图标...${NC}"

while IFS=':' read -r filename size; do
  generate_icon "$filename" "$size"
done <<'EOF'
apple-touch-icon.png:180
favicon-16x16.png:16
favicon-32x32.png:32
favicon-48x48.png:48
icon-28x28.png:28
icon-108x108.png:108
icon-192x192.png:192
icon-512x512.png:512
icon-1024x1024.png:1024
EOF

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  ✓ Web 图标生成完成！${NC}"
echo -e "${GREEN}========================================${NC}"
echo -e "${YELLOW}提示: favicon.svg / safari-pinned-tab.svg 未被修改，如需替换请单独处理。${NC}"
