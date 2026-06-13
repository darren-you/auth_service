#!/usr/bin/env bash

# =============================================================================
# macOS AppIcon 替换脚本
# 用途:
#   从一张 1024x1024 源图自动生成并覆盖 Assets.xcassets/AppIcon.appiconset 全套图标。
#
# 当前项目配置：
#   - build settings: ASSETCATALOG_COMPILER_APPICON_NAME = AppIcon
#   - 资源目录: 自动识别 AppIcon.appiconset
#
# 配置来自：`项目根目录/deploy_config.sh`（通过 `--config` 传入）
# 依赖 deploy_config.sh 配置项：
#   - APP_ICON_IMAGE_1024（必填）：1024x1024 源图路径（相对项目根目录或绝对路径）
#
# 提示：
#   - 建议源图为 1024x1024、PNG、无透明通道（App Store 要求 marketing icon 无 alpha）。
# =============================================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

usage() {
  cat <<'EOF'
用法:
  change_app_icon.sh --config <deploy_config.sh绝对路径>

说明:
  - 必须通过 --config 指定外部项目配置文件
  - 请在 `deploy_config.sh` 中配置：
      APP_ICON_IMAGE_1024="deploy_assets/macos/icon/icon_1024.png"      # 相对项目根目录或绝对路径
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

# 清理可能来自环境变量的同名值，确保只以 deploy_config.sh 为准
unset APP_ICON_IMAGE_1024

# shellcheck disable=SC1090
source "${DEPLOY_CONFIG_FILE}"

if [ -z "${APP_ICON_IMAGE_1024:-}" ]; then
  echo "错误: deploy_config.sh 未配置 APP_ICON_IMAGE_1024"
  exit 1
fi

infer_app_iconset_dir() {
  local exact="${PROJECT_ROOT}/Assets.xcassets/AppIcon.appiconset"
  if [ -d "$exact" ]; then
    echo "$exact"
    return 0
  fi

  local candidates=()
  while IFS= read -r p; do
    candidates+=("$p")
  done < <(find "${PROJECT_ROOT}" -maxdepth 4 -type d -name "AppIcon.appiconset" | sort)

  if [[ ${#candidates[@]} -eq 1 ]]; then
    echo "${candidates[0]}"
    return 0
  fi

  if [[ ${#candidates[@]} -gt 1 ]]; then
    return 2
  fi

  return 1
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

# 解析路径
SOURCE_ICON="$(resolve_asset_path "$APP_ICON_IMAGE_1024")"
require_file "$SOURCE_ICON" "APP_ICON_IMAGE_1024"

if APPICONSET="$(infer_app_iconset_dir)"; then
  echo "提示: 已自动识别 AppIcon 目录 ${APPICONSET}"
else
  rc=$?
  if [[ $rc -eq 1 ]]; then
    echo "错误: 未找到 AppIcon.appiconset，无法自动识别图标目录"
  else
    echo "错误: 检测到多个 AppIcon.appiconset，无法自动识别图标目录"
    find "${PROJECT_ROOT}" -maxdepth 4 -type d -name "AppIcon.appiconset" -print | sed 's/^/  - /'
  fi
  exit 1
fi
CONTENTS_JSON="${APPICONSET}/Contents.json"

if [ ! -d "$APPICONSET" ]; then
  echo -e "${RED}错误: 找不到 AppIcon.appiconset: ${APPICONSET}${NC}"
  exit 1
fi
require_file "$CONTENTS_JSON" "Contents.json"

if ! command -v sips >/dev/null 2>&1; then
  echo -e "${RED}错误: 未找到 sips，无法生成图标尺寸${NC}"
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo -e "${RED}错误: 未找到 python3，无法解析 Contents.json${NC}"
  exit 1
fi

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}  AppIcon 替换工具${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""
echo -e "项目根目录: ${GREEN}${PROJECT_ROOT}${NC}"
echo -e "源图:       ${GREEN}${SOURCE_ICON}${NC}"
echo -e "AppIconSet: ${GREEN}${APPICONSET}${NC}"
echo ""

# --------------------- 生成/覆盖 ---------------------
echo -e "${BLUE}开始生成 AppIcon...${NC}"

python3 - "$CONTENTS_JSON" <<'PY' | while IFS=$'\t' read -r filename w h; do
import json, sys, math
p=sys.argv[1]
d=json.load(open(p,'r',encoding='utf-8'))
for img in d.get("images", []):
    fn=img.get("filename")
    size=img.get("size")
    scale=img.get("scale")
    if not fn or not size or not scale:
        continue
    try:
        w_pt, h_pt = size.split("x")
        w_pt=float(w_pt); h_pt=float(h_pt)
        s=float(scale[:-1])
        w_px=int(round(w_pt*s))
        h_px=int(round(h_pt*s))
    except Exception:
        continue
    print(f"{fn}\t{w_px}\t{h_px}")
PY
  # shell loop body
  target="${APPICONSET}/${filename}"
  echo -e "  - ${GREEN}${filename}${NC} (${w}x${h})"
  # 生成 png（覆盖）
  sips -s format png -z "$h" "$w" "$SOURCE_ICON" --out "$target" >/dev/null
done

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  ✓ AppIcon 替换完成！${NC}"
echo -e "${GREEN}========================================${NC}"
echo -e "${YELLOW}提示: 如 Xcode 未刷新，可 Clean Build Folder 或删除 DerivedData。${NC}"
