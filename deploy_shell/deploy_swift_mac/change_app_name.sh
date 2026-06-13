#!/usr/bin/env bash

# =============================================================================
# macOS App 显示名称修改脚本
# 用途: 修改桌面图标下显示的 App 名称（CFBundleDisplayName）
#
# 默认会修改工程 build settings 中的：
#   INFOPLIST_KEY_CFBundleDisplayName
# 适用于本项目当前使用的 “Generate Info.plist” 方式。
# 配置来自：`项目根目录/deploy_config.sh`（通过 `--config` 传入）
# 依赖 deploy_config.sh 配置项：
#   - APP_DISPLAY_NAME（必填）：目标 App 显示名称
# =============================================================================

usage() {
    cat <<'EOF'
用法:
  change_app_name.sh --config <deploy_config.sh绝对路径>
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
unset APP_DISPLAY_NAME

# shellcheck disable=SC1090
source "${DEPLOY_CONFIG_FILE}"

if [ -z "${APP_DISPLAY_NAME:-}" ]; then
    echo "错误: deploy_config.sh 未配置 APP_DISPLAY_NAME"
    exit 1
fi

infer_pbxproj_path() {
    local candidates=()
    while IFS= read -r proj; do
        candidates+=("$proj")
    done < <(find "${PROJECT_ROOT}" -maxdepth 1 -type d -name "*.xcodeproj" | sort)

    if [[ ${#candidates[@]} -eq 0 ]]; then
        return 1
    fi

    if [[ ${#candidates[@]} -eq 1 ]]; then
        echo "${candidates[0]}/project.pbxproj"
        return 0
    fi

    local matched=()
    for proj in "${candidates[@]}"; do
        local name
        name="$(basename "$proj" .xcodeproj)"
        if [[ -d "${PROJECT_ROOT}/${name}" ]]; then
            matched+=("$proj")
        fi
    done

    if [[ ${#matched[@]} -eq 1 ]]; then
        echo "${matched[0]}/project.pbxproj"
        return 0
    fi

    return 2
}

if PBXPROJ_PATH="$(infer_pbxproj_path)"; then
    echo "提示: 已自动识别工程文件 ${PBXPROJ_PATH}"
else
    rc=$?
    if [[ $rc -eq 1 ]]; then
        echo "错误: 项目根目录未找到 .xcodeproj，无法自动识别 project.pbxproj"
    else
        echo "错误: 项目根目录存在多个 .xcodeproj，无法自动识别 project.pbxproj"
        find "${PROJECT_ROOT}" -maxdepth 1 -type d -name "*.xcodeproj" -print | sed 's#^.*/##; s#\.xcodeproj$##' | sed 's/^/  - /'
    fi
    exit 1
fi

NEW_APP_NAME="$APP_DISPLAY_NAME"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}  macOS App 显示名称修改工具${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""

# 解析 INFOPLIST_FILE 配置为绝对路径
resolve_infoplist_path() {
    local raw="$1"
    local p="$raw"
    p="$(echo "$p" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//; s/^"//; s/"$//')"

    p="${p//\$\{SRCROOT\}\//}"
    p="${p//\$\(SRCROOT\)\//}"
    p="${p//\$\{PROJECT_DIR\}\//}"
    p="${p//\$\(PROJECT_DIR\)\//}"

    if [[ "$p" = /* ]]; then
        echo "$p"
        return 0
    fi
    echo "${PROJECT_ROOT%/}/$p"
}

# 检查文件是否存在
if [ -z "$PBXPROJ_PATH" ] || [ ! -f "$PBXPROJ_PATH" ]; then
    echo -e "${RED}错误: 找不到 project.pbxproj 文件${NC}"
    echo "路径: ${PBXPROJ_PATH:-<未找到>}"
    exit 1
fi

# 优先：修改 pbxproj 中的 INFOPLIST_KEY_CFBundleDisplayName（Generate Info.plist 模式）
CURRENT_APP_NAME_RAW=$(grep -o 'INFOPLIST_KEY_CFBundleDisplayName = [^;]*' "$PBXPROJ_PATH" | head -1 | sed 's/INFOPLIST_KEY_CFBundleDisplayName = //')
if [ -n "$CURRENT_APP_NAME_RAW" ]; then
    CURRENT_APP_NAME=$(echo "$CURRENT_APP_NAME_RAW" | sed 's/^"//; s/"$//')

    echo -e "项目路径: ${GREEN}${PROJECT_ROOT}${NC}"
    echo -e "工程文件: ${GREEN}${PBXPROJ_PATH}${NC}"
    echo -e "修改方式: ${GREEN}project.pbxproj (INFOPLIST_KEY_CFBundleDisplayName)${NC}"
    echo -e "当前 App 名称: ${YELLOW}${CURRENT_APP_NAME}${NC}"
    echo -e "目标 App 名称: ${GREEN}${NEW_APP_NAME}${NC}"
    echo ""

    if [ "$CURRENT_APP_NAME" == "$NEW_APP_NAME" ]; then
        echo -e "${GREEN}✓ App 名称已经是目标值，无需修改${NC}"
        exit 0
    fi

    ESCAPED_NEW_APP_NAME=$(printf '%s' "$NEW_APP_NAME" | sed -e 's/[\\/&|]/\\&/g')
    sed -i '' -E "s|(INFOPLIST_KEY_CFBundleDisplayName = )[^;]*;|\\1\"${ESCAPED_NEW_APP_NAME}\";|g" "$PBXPROJ_PATH"

    UPDATED_APP_NAME_RAW=$(grep -o 'INFOPLIST_KEY_CFBundleDisplayName = [^;]*' "$PBXPROJ_PATH" | head -1 | sed 's/INFOPLIST_KEY_CFBundleDisplayName = //')
    UPDATED_APP_NAME=$(echo "$UPDATED_APP_NAME_RAW" | sed 's/^"//; s/"$//')

    if [ "$UPDATED_APP_NAME" == "$NEW_APP_NAME" ]; then
        echo ""
        echo -e "${GREEN}========================================${NC}"
        echo -e "${GREEN}  ✓ App 名称修改成功！${NC}"
        echo -e "${GREEN}========================================${NC}"
        echo -e "新的 App 名称: ${GREEN}${NEW_APP_NAME}${NC}"
        echo ""
        echo -e "${YELLOW}提示: 请 Clean + 重新编译，并在真机/模拟器上确认桌面名称更新（可能需要卸载重装才完全刷新）${NC}"
        exit 0
    fi

    echo -e "${RED}✗ 修改失败，请检查 project.pbxproj 后重试${NC}"
    exit 1
fi

# 兜底：修改 Info.plist 中的 CFBundleDisplayName（静态 Info.plist 模式）
INFOPLIST_FILE_RAW=$(awk -F' = ' '
    /^[[:space:]]*INFOPLIST_FILE = / {
        v=$2
        sub(/;.*/, "", v)
        gsub(/^[[:space:]]+|[[:space:]]+$/, "", v)
        print v
        exit
    }
' "$PBXPROJ_PATH")
if [ -z "$INFOPLIST_FILE_RAW" ]; then
    echo -e "${RED}错误: 未在 project.pbxproj 中找到 INFOPLIST_KEY_CFBundleDisplayName 或 INFOPLIST_FILE${NC}"
    echo "文件: $PBXPROJ_PATH"
    exit 1
fi

INFO_PLIST_PATH="$(resolve_infoplist_path "$INFOPLIST_FILE_RAW")"
if [ ! -f "$INFO_PLIST_PATH" ]; then
    echo -e "${RED}错误: 找不到 Info.plist 文件${NC}"
    echo "路径: $INFO_PLIST_PATH"
    exit 1
fi

if ! command -v /usr/libexec/PlistBuddy >/dev/null 2>&1; then
    echo -e "${RED}错误: 未找到 /usr/libexec/PlistBuddy，无法修改 Info.plist${NC}"
    exit 1
fi

CURRENT_APP_NAME="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleDisplayName' "$INFO_PLIST_PATH" 2>/dev/null || true)"
if [ -z "$CURRENT_APP_NAME" ]; then
    CURRENT_APP_NAME="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleName' "$INFO_PLIST_PATH" 2>/dev/null || true)"
fi
[ -n "$CURRENT_APP_NAME" ] || CURRENT_APP_NAME="<未设置>"

echo -e "项目路径: ${GREEN}${PROJECT_ROOT}${NC}"
echo -e "工程文件: ${GREEN}${PBXPROJ_PATH}${NC}"
echo -e "Info.plist: ${GREEN}${INFO_PLIST_PATH}${NC}"
echo -e "修改方式: ${GREEN}Info.plist (CFBundleDisplayName)${NC}"
echo -e "当前 App 名称: ${YELLOW}${CURRENT_APP_NAME}${NC}"
echo -e "目标 App 名称: ${GREEN}${NEW_APP_NAME}${NC}"
echo ""

if [ "$CURRENT_APP_NAME" == "$NEW_APP_NAME" ]; then
    echo -e "${GREEN}✓ App 名称已经是目标值，无需修改${NC}"
    exit 0
fi

if /usr/libexec/PlistBuddy -c 'Print :CFBundleDisplayName' "$INFO_PLIST_PATH" >/dev/null 2>&1; then
    /usr/libexec/PlistBuddy -c "Set :CFBundleDisplayName ${NEW_APP_NAME}" "$INFO_PLIST_PATH"
else
    /usr/libexec/PlistBuddy -c "Add :CFBundleDisplayName string ${NEW_APP_NAME}" "$INFO_PLIST_PATH"
fi

UPDATED_APP_NAME="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleDisplayName' "$INFO_PLIST_PATH" 2>/dev/null || true)"
if [ "$UPDATED_APP_NAME" == "$NEW_APP_NAME" ]; then
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  ✓ App 名称修改成功！${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo -e "新的 App 名称: ${GREEN}${NEW_APP_NAME}${NC}"
    echo ""
    echo -e "${YELLOW}提示: 请 Clean + 重新编译，并在真机/模拟器上确认桌面名称更新（可能需要卸载重装才完全刷新）${NC}"
else
    echo -e "${RED}✗ 修改失败，请检查 Info.plist 后重试${NC}"
    exit 1
fi
