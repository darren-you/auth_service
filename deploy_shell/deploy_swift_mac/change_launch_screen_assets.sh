#!/usr/bin/env bash

# =============================================================================
# macOS 启动页（LaunchScreen）资源替换脚本
# 用途:
#   - 替换启动屏中的背景图/Logo/底部 Slogan（对应 Assets.xcassets 的 imageset）
#   - 背景图可选：不设置背景图时，可在脚本中设置纯色背景，并自动从 storyboard 移除背景图引用
#
# 关联关系（当前项目）：
#   LaunchScreen.storyboard 引用图片名：
#     - LaunchBackground  -> Assets.xcassets/LaunchBackground.imageset
#     - LaunchLogoAlpha   -> Assets.xcassets/LaunchLogoAlpha.imageset
#     - LaunchBranding    -> Assets.xcassets/LaunchBranding.imageset
# 配置来自：`项目根目录/deploy_config.sh`
# 依赖 deploy_config.sh 配置项：
#   - 自动识别：ASSETS_XCASSETS_DIR、LAUNCH_SCREEN_STORYBOARD_PATH（无需配置）
#   - 可选（背景）：LAUNCH_BACKGROUND_IMAGE、LAUNCH_BACKGROUND_IMAGE_1X/2X/3X、LAUNCH_BACKGROUND_COLOR
#   - 可选（Logo）：LAUNCH_LOGO_IMAGE、LAUNCH_LOGO_IMAGE_1X/2X/3X
#   - 可选（Slogan）：LAUNCH_SLOGAN_IMAGE、LAUNCH_SLOGAN_IMAGE_1X/2X/3X
# =============================================================================

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

usage() {
  cat <<'EOF'
用法:
  change_launch_screen_assets.sh --config <deploy_config.sh绝对路径>
EOF
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
unset ASSETS_XCASSETS_DIR LAUNCH_SCREEN_STORYBOARD_PATH
unset LAUNCH_BACKGROUND_IMAGE LAUNCH_BACKGROUND_COLOR
unset LAUNCH_LOGO_IMAGE LAUNCH_SLOGAN_IMAGE
unset LAUNCH_LOGO_IMAGE_1X LAUNCH_LOGO_IMAGE_2X LAUNCH_LOGO_IMAGE_3X
unset LAUNCH_SLOGAN_IMAGE_1X LAUNCH_SLOGAN_IMAGE_2X LAUNCH_SLOGAN_IMAGE_3X
unset LAUNCH_BACKGROUND_IMAGE_1X LAUNCH_BACKGROUND_IMAGE_2X LAUNCH_BACKGROUND_IMAGE_3X

# shellcheck disable=SC1090
source "${DEPLOY_CONFIG_FILE}"

infer_launch_storyboard_path() {
  local exact="${PROJECT_ROOT}/LaunchScreen.storyboard"
  if [ -f "$exact" ]; then
    echo "$exact"
    return 0
  fi

  local candidates=()
  while IFS= read -r p; do
    candidates+=("$p")
  done < <(find "${PROJECT_ROOT}" -maxdepth 3 -type f -name "LaunchScreen.storyboard" | sort)

  if [[ ${#candidates[@]} -eq 1 ]]; then
    echo "${candidates[0]}"
    return 0
  fi
  if [[ ${#candidates[@]} -gt 1 ]]; then
    return 2
  fi

  while IFS= read -r p; do
    candidates+=("$p")
  done < <(find "${PROJECT_ROOT}" -maxdepth 3 -type f -name "*Launch*Screen*.storyboard" | sort)
  if [[ ${#candidates[@]} -eq 1 ]]; then
    echo "${candidates[0]}"
    return 0
  fi
  if [[ ${#candidates[@]} -gt 1 ]]; then
    return 2
  fi

  return 1
}

infer_assets_xcassets_dir() {
  local exact="${PROJECT_ROOT}/Assets.xcassets"
  if [ -d "$exact" ]; then
    echo "$exact"
    return 0
  fi

  local launch_related=()
  local all=()
  while IFS= read -r p; do
    all+=("$p")
    if [ -d "${p}/LaunchBackground.imageset" ] && [ -d "${p}/LaunchLogoAlpha.imageset" ] && [ -d "${p}/LaunchBranding.imageset" ]; then
      launch_related+=("$p")
    fi
  done < <(find "${PROJECT_ROOT}" -maxdepth 3 -type d -name "*.xcassets" | sort)

  if [[ ${#launch_related[@]} -eq 1 ]]; then
    echo "${launch_related[0]}"
    return 0
  fi
  if [[ ${#launch_related[@]} -gt 1 ]]; then
    return 2
  fi

  if [[ ${#all[@]} -eq 1 ]]; then
    echo "${all[0]}"
    return 0
  fi
  if [[ ${#all[@]} -gt 1 ]]; then
    return 2
  fi

  return 1
}

if LAUNCH_SCREEN_STORYBOARD_PATH="$(infer_launch_storyboard_path)"; then
  echo "提示: 已自动识别 LaunchScreen.storyboard ${LAUNCH_SCREEN_STORYBOARD_PATH}"
else
  rc=$?
  if [[ $rc -eq 1 ]]; then
    echo "错误: 未找到 LaunchScreen.storyboard，无法自动识别启动页 storyboard"
  else
    echo "错误: 检测到多个 LaunchScreen storyboard，无法自动识别启动页 storyboard"
    find "${PROJECT_ROOT}" -maxdepth 3 -type f -name "*Launch*Screen*.storyboard" -print | sed 's/^/  - /'
  fi
  exit 1
fi

if ASSETS_XCASSETS_DIR="$(infer_assets_xcassets_dir)"; then
  echo "提示: 已自动识别 Assets.xcassets ${ASSETS_XCASSETS_DIR}"
else
  rc=$?
  if [[ $rc -eq 1 ]]; then
    echo "错误: 未找到 .xcassets 目录，无法自动识别 Assets.xcassets"
  else
    echo "错误: 检测到多个 .xcassets 目录，无法自动识别 Assets.xcassets"
    find "${PROJECT_ROOT}" -maxdepth 3 -type d -name "*.xcassets" -print | sed 's/^/  - /'
  fi
  exit 1
fi

require_file() {
  local p="$1"
  local desc="$2"
  if [ -z "$p" ] || [ ! -f "$p" ]; then
    echo -e "${RED}错误: 找不到 ${desc} 文件: ${p}${NC}"
    exit 1
  fi
}

# 解析资源图片路径
# - 绝对路径：原样返回
# - 相对路径：相对项目根目录
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

# 解析 config 中相对项目根目录的路径（如 storyboard / Assets.xcassets）
resolve_shell_path() {
  local p="$1"
  if [ -z "$p" ]; then
    echo ""
    return 0
  fi
  if [[ "$p" = /* ]]; then
    echo "$p"
    return 0
  fi
  echo "$(cd "$PROJECT_ROOT" && cd "$(dirname "$p")" && pwd)/$(basename "$p")"
}

copy_or_convert() {
  local src="$1"
  local dst="$2"

  local ext="${dst##*.}"
  local ext_lower
  ext_lower="$(echo "$ext" | tr '[:upper:]' '[:lower:]')"
  local fmt=""
  case "$ext_lower" in
    png) fmt="png" ;;
    jpg|jpeg) fmt="jpeg" ;;
    *)
      echo -e "${RED}错误: 不支持的目标文件格式: $dst${NC}"
      return 1
      ;;
  esac

  # 使用 sips 转换格式并写入（不做尺寸缩放；imageset 本身不带 size 信息）
  sips -s format "$fmt" "$src" --out "$dst" >/dev/null
}

get_imageset_scale_map() {
  local contents_json="$1"
  python3 - "$contents_json" <<'PY'
import json, sys
p=sys.argv[1]
with open(p,'r',encoding='utf-8') as f:
    d=json.load(f)
for img in d.get("images", []):
    scale = img.get("scale")
    fn = img.get("filename")
    if scale and fn:
        print(f"{scale}\t{fn}")
PY
}

apply_imageset() {
  local imageset_dir="$1"
  local base_src="$2"
  local src_1x="$3"
  local src_2x="$4"
  local src_3x="$5"

  local contents_json="${imageset_dir}/Contents.json"
  require_file "$contents_json" "imageset Contents.json"

  # 兼容 macOS 自带 bash 3.2：不使用 associative array（declare -A）
  local fn_1x="" fn_2x="" fn_3x=""
  while IFS=$'\t' read -r scale fn; do
    case "$scale" in
      1x) fn_1x="$fn" ;;
      2x) fn_2x="$fn" ;;
      3x) fn_3x="$fn" ;;
    esac
  done < <(get_imageset_scale_map "$contents_json")

  for scale in 1x 2x 3x; do
    local target_fn=""
    case "$scale" in
      1x) target_fn="$fn_1x" ;;
      2x) target_fn="$fn_2x" ;;
      3x) target_fn="$fn_3x" ;;
    esac
    [ -n "$target_fn" ] || continue

    local chosen=""
    case "$scale" in
      1x) chosen="${src_1x:-${base_src:-}}" ;;
      2x) chosen="${src_2x:-${base_src:-}}" ;;
      3x) chosen="${src_3x:-${base_src:-}}" ;;
    esac

    if [ -z "$chosen" ]; then
      echo -e "${YELLOW}- 跳过 ${imageset_dir##*/} ${scale}：未提供源图${NC}"
      continue
    fi

    require_file "$chosen" "源图"
    local dst="${imageset_dir}/${target_fn}"
    echo -e "  - 写入 ${GREEN}${imageset_dir##*/}${NC} ${scale} -> ${GREEN}${dst}${NC}"
    copy_or_convert "$chosen" "$dst"
  done
}

set_background_image_reference() {
  local storyboard="$1"
  local mode="$2" # on|off

  python3 - "$storyboard" "$mode" <<'PY'
import sys
path=sys.argv[1]
mode=sys.argv[2]

with open(path,'r',encoding='utf-8') as f:
    lines=f.readlines()

out=[]
changed=False
for line in lines:
    if 'id="uNh-iN-P7v"' in line and "<imageView" in line:
        if mode == "off":
            if ' image="LaunchBackground"' in line:
                line=line.replace(' image="LaunchBackground"', '')
                changed=True
        elif mode == "on":
            if ' image="LaunchBackground"' not in line:
                # 插入到 translatesAutoresizingMaskIntoConstraints 之前
                if ' translatesAutoresizingMaskIntoConstraints="NO"' in line:
                    line=line.replace(' translatesAutoresizingMaskIntoConstraints="NO"',
                                      ' image="LaunchBackground" translatesAutoresizingMaskIntoConstraints="NO"', 1)
                    changed=True
                else:
                    # 兜底：直接追加
                    line=line.replace(' id="uNh-iN-P7v"', ' image="LaunchBackground" id="uNh-iN-P7v"', 1)
                    changed=True
        else:
            raise SystemExit("mode must be on|off")
    out.append(line)

if changed:
    with open(path,'w',encoding='utf-8') as f:
        f.writelines(out)
print("Storyboard background image reference:", "updated" if changed else "no change")
PY
}

set_storyboard_bg_color() {
  local storyboard="$1"
  local hex="$2"

  python3 - "$storyboard" "$hex" <<'PY'
import re, sys
path=sys.argv[1]
hex_=sys.argv[2].strip()
if hex_.startswith("#"):
    hex_=hex_[1:]
if len(hex_)!=6 or any(c not in "0123456789abcdefABCDEF" for c in hex_):
    raise SystemExit(f"Invalid bg color: {sys.argv[2]} (expect #RRGGBB)")

r=int(hex_[0:2],16)/255
g=int(hex_[2:4],16)/255
b=int(hex_[4:6],16)/255

txt=open(path,'r',encoding='utf-8').read()
pattern=r'(<color key="backgroundColor" red=")([^"]+)(" green=")([^"]+)(" blue=")([^"]+)(" alpha=")([^"]+)(" colorSpace="custom" customColorSpace="sRGB"\s*/>)'
m=re.search(pattern, txt)
if not m:
    raise SystemExit("Could not find backgroundColor in storyboard")

new=f'{m.group(1)}{r:.6f}{m.group(3)}{g:.6f}{m.group(5)}{b:.6f}{m.group(7)}{m.group(8)}{m.group(9)}'
txt2=re.sub(pattern, new, txt, count=1)
open(path,'w',encoding='utf-8').write(txt2)
print(f"Storyboard backgroundColor set to #{hex_.upper()} ({r:.6f},{g:.6f},{b:.6f})")
PY
}

# --------------------- 读取 config 配置 ---------------------
# 图片资源（相对项目根目录或绝对路径）
BG="$(resolve_asset_path "${LAUNCH_BACKGROUND_IMAGE:-}")"
BG_1X="$(resolve_asset_path "${LAUNCH_BACKGROUND_IMAGE_1X:-}")"
BG_2X="$(resolve_asset_path "${LAUNCH_BACKGROUND_IMAGE_2X:-}")"
BG_3X="$(resolve_asset_path "${LAUNCH_BACKGROUND_IMAGE_3X:-}")"

LOGO="$(resolve_asset_path "${LAUNCH_LOGO_IMAGE:-}")"
LOGO_1X="$(resolve_asset_path "${LAUNCH_LOGO_IMAGE_1X:-}")"
LOGO_2X="$(resolve_asset_path "${LAUNCH_LOGO_IMAGE_2X:-}")"
LOGO_3X="$(resolve_asset_path "${LAUNCH_LOGO_IMAGE_3X:-}")"

SLOGAN="$(resolve_asset_path "${LAUNCH_SLOGAN_IMAGE:-}")"
SLOGAN_1X="$(resolve_asset_path "${LAUNCH_SLOGAN_IMAGE_1X:-}")"
SLOGAN_2X="$(resolve_asset_path "${LAUNCH_SLOGAN_IMAGE_2X:-}")"
SLOGAN_3X="$(resolve_asset_path "${LAUNCH_SLOGAN_IMAGE_3X:-}")"

# 背景色（当不使用背景图时生效）
BG_COLOR="${LAUNCH_BACKGROUND_COLOR:-}"

# 背景可选：若未配置背景图，则必须配置背景色（#RRGGBB）
if [ -z "$BG" ] && [ -z "$BG_1X" ] && [ -z "$BG_2X" ] && [ -z "$BG_3X" ]; then
  if [ -z "$BG_COLOR" ]; then
    echo -e "${RED}错误: 未配置 LAUNCH_BACKGROUND_COLOR（不使用背景图时必须提供）${NC}"
    exit 1
  fi
fi

# storyboard / assets 路径（相对项目根目录）
STORYBOARD_PATH="$(resolve_shell_path "$LAUNCH_SCREEN_STORYBOARD_PATH")"
ASSETS_ROOT="$(resolve_shell_path "$ASSETS_XCASSETS_DIR")"

if [ -z "$STORYBOARD_PATH" ] || [ ! -f "$STORYBOARD_PATH" ]; then
  echo -e "${RED}错误: 找不到 LaunchScreen.storyboard: ${STORYBOARD_PATH}${NC}"
  exit 1
fi

if [ -z "$ASSETS_ROOT" ] || [ ! -d "$ASSETS_ROOT" ]; then
  echo -e "${RED}错误: 找不到 Assets.xcassets: ${ASSETS_ROOT}${NC}"
  exit 1
fi

LAUNCH_BG_SET="${ASSETS_ROOT}/LaunchBackground.imageset"
LAUNCH_LOGO_SET="${ASSETS_ROOT}/LaunchLogoAlpha.imageset"
LAUNCH_SLOGAN_SET="${ASSETS_ROOT}/LaunchBranding.imageset"

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}  LaunchScreen 启动页资源替换工具${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""
echo -e "项目根目录: ${GREEN}${PROJECT_ROOT}${NC}"
echo -e "Storyboard : ${GREEN}${STORYBOARD_PATH}${NC}"
echo -e "AssetsRoot : ${GREEN}${ASSETS_ROOT}${NC}"
echo ""

# 检查 sips
if ! command -v sips >/dev/null 2>&1; then
  echo -e "${RED}错误: 未找到 sips，无法进行图片格式转换${NC}"
  exit 1
fi

# --------------------- 执行修改 ---------------------
USE_BG_IMAGE=0
if [ -n "$BG" ] || [ -n "$BG_1X" ] || [ -n "$BG_2X" ] || [ -n "$BG_3X" ]; then
  USE_BG_IMAGE=1
fi

echo -e "${BLUE}[1/3] 处理启动页背景...${NC}"
if [ $USE_BG_IMAGE -eq 1 ]; then
  echo -e "- 背景模式: ${GREEN}使用背景图${NC}"
  if [ ! -d "$LAUNCH_BG_SET" ]; then
    echo -e "${RED}错误: 找不到 ${LAUNCH_BG_SET}${NC}"
    exit 1
  fi
  set_background_image_reference "$STORYBOARD_PATH" "on"
  apply_imageset "$LAUNCH_BG_SET" "$BG" "$BG_1X" "$BG_2X" "$BG_3X"
else
  echo -e "- 背景模式: ${YELLOW}纯色背景（无背景图）${NC}"
  echo -e "  背景色: ${GREEN}${BG_COLOR}${NC}"
  set_background_image_reference "$STORYBOARD_PATH" "off"
  set_storyboard_bg_color "$STORYBOARD_PATH" "$BG_COLOR"
fi
echo ""

echo -e "${BLUE}[2/3] 处理 Logo...${NC}"
if [ -n "$LOGO" ] || [ -n "$LOGO_1X" ] || [ -n "$LOGO_2X" ] || [ -n "$LOGO_3X" ]; then
  if [ ! -d "$LAUNCH_LOGO_SET" ]; then
    echo -e "${RED}错误: 找不到 ${LAUNCH_LOGO_SET}${NC}"
    exit 1
  fi
  apply_imageset "$LAUNCH_LOGO_SET" "$LOGO" "$LOGO_1X" "$LOGO_2X" "$LOGO_3X"
else
  echo -e "${YELLOW}- 未提供 Logo 源图，跳过${NC}"
fi
echo ""

echo -e "${BLUE}[3/3] 处理底部 Slogan...${NC}"
if [ -n "$SLOGAN" ] || [ -n "$SLOGAN_1X" ] || [ -n "$SLOGAN_2X" ] || [ -n "$SLOGAN_3X" ]; then
  if [ ! -d "$LAUNCH_SLOGAN_SET" ]; then
    echo -e "${RED}错误: 找不到 ${LAUNCH_SLOGAN_SET}${NC}"
    exit 1
  fi
  apply_imageset "$LAUNCH_SLOGAN_SET" "$SLOGAN" "$SLOGAN_1X" "$SLOGAN_2X" "$SLOGAN_3X"
else
  echo -e "${YELLOW}- 未提供 Slogan 源图，跳过${NC}"
fi

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  ✓ 启动页资源处理完成！${NC}"
echo -e "${GREEN}========================================${NC}"
echo -e "${YELLOW}提示: 如未立即生效，请 Clean Build Folder；必要时卸载重装以清缓存。${NC}"
