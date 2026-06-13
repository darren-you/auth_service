#!/usr/bin/env bash

# =============================================================================
# iOS 项目名称修改脚本
# 用途: 修改 iOS 原生工程的项目名称（包括文件夹、配置文件等）
# 依赖 deploy_config.sh 配置项：
#   - NEW_NAME（必填）：目标项目名（如 imagen）
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
  change_project_name.sh --config <deploy_config.sh绝对路径>
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
PROJECT_ROOT_FROM_ARG="${PROJECT_ROOT}"

# 清理可能来自环境变量的同名值，确保只以 deploy_config.sh 为准
unset NEW_NAME

# shellcheck disable=SC1090
source "${DEPLOY_CONFIG_FILE}"

# 避免被 deploy_config.sh 内同名变量覆盖
PROJECT_ROOT="${PROJECT_ROOT_FROM_ARG}"

if [[ -z "${NEW_NAME:-}" ]]; then
    echo "错误: deploy_config.sh 未配置 NEW_NAME（新项目名）"
    exit 1
fi

# 自动识别当前项目名：
# 1) 如果只有一个 *.xcodeproj，直接使用；
# 2) 如果有多个，优先选择同名源码目录（<name>/）也存在的那个；
# 3) 仍无法唯一确定则报错。
XCODEPROJ_CANDIDATES=()
while IFS= read -r proj; do
    XCODEPROJ_CANDIDATES+=("$proj")
done < <(find "${PROJECT_ROOT}" -maxdepth 1 -type d -name "*.xcodeproj" | sort)

if [[ ${#XCODEPROJ_CANDIDATES[@]} -eq 0 ]]; then
    echo -e "${RED}错误: 项目根目录未找到 .xcodeproj 文件，无法自动识别当前项目名${NC}"
    exit 1
fi

if [[ ${#XCODEPROJ_CANDIDATES[@]} -eq 1 ]]; then
    CURRENT_PROJECT_NAME="$(basename "${XCODEPROJ_CANDIDATES[0]}" .xcodeproj)"
else
    MATCHED_NAMES=()
    for proj in "${XCODEPROJ_CANDIDATES[@]}"; do
        name="$(basename "$proj" .xcodeproj)"
        if [[ -d "${PROJECT_ROOT}/${name}" ]]; then
            MATCHED_NAMES+=("$name")
        fi
    done

    if [[ ${#MATCHED_NAMES[@]} -eq 1 ]]; then
        CURRENT_PROJECT_NAME="${MATCHED_NAMES[0]}"
    else
        echo -e "${RED}错误: 项目根目录存在多个 .xcodeproj，无法自动识别当前项目名${NC}"
        echo "检测到："
        for proj in "${XCODEPROJ_CANDIDATES[@]}"; do
            echo "  - $(basename "$proj" .xcodeproj)"
        done
        echo "请清理无关工程后重试"
        exit 1
    fi
fi

if [[ "${CURRENT_PROJECT_NAME}" == "${NEW_NAME}" ]]; then
    echo -e "${YELLOW}提示: 当前项目名与新项目名一致，无需修改${NC}"
    exit 0
fi

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}  iOS 项目名称修改工具${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""

echo -e "项目根目录: ${GREEN}${PROJECT_ROOT}${NC}"
echo -e "当前项目名: ${YELLOW}${CURRENT_PROJECT_NAME}${NC}"
echo -e "新项目名: ${GREEN}${NEW_NAME}${NC}"
echo ""

# 检查当前项目是否存在
CURRENT_XCODEPROJ="${PROJECT_ROOT}/${CURRENT_PROJECT_NAME}.xcodeproj"
CURRENT_SOURCE_DIR="${PROJECT_ROOT}/${CURRENT_PROJECT_NAME}"
HAS_SOURCE_DIR_RENAME="false"

if [ ! -d "$CURRENT_XCODEPROJ" ]; then
    echo -e "${RED}错误: 找不到 ${CURRENT_PROJECT_NAME}.xcodeproj${NC}"
    echo "路径: $CURRENT_XCODEPROJ"
    exit 1
fi

if [ -d "$CURRENT_SOURCE_DIR" ]; then
    HAS_SOURCE_DIR_RENAME="true"
else
    echo -e "${YELLOW}提示: 未发现同名源码目录 ${CURRENT_PROJECT_NAME}/，按扁平目录结构处理（仅重命名工程与引用）${NC}"
fi

# 确认操作
echo -e "${YELLOW}警告: 此操作将修改项目结构，建议先提交或备份代码！${NC}"
echo -n "是否继续? (y/N): "
read -r CONFIRM
if [[ ! "$CONFIRM" =~ ^[Yy]$ ]]; then
    echo "操作已取消"
    exit 0
fi

echo ""
echo -e "${BLUE}开始修改项目名称...${NC}"
echo ""

PBXPROJ_PATH="${CURRENT_XCODEPROJ}/project.pbxproj"

# 1. 修改 project.pbxproj 中的引用
echo -e "${BLUE}[1/5] 修改 project.pbxproj 中的引用...${NC}"

# 替换项目名称相关的引用
sed -i '' "s/${CURRENT_PROJECT_NAME}\.app/${NEW_NAME}.app/g" "$PBXPROJ_PATH"
sed -i '' "s/path = ${CURRENT_PROJECT_NAME};/path = ${NEW_NAME};/g" "$PBXPROJ_PATH"
sed -i '' "s/name = ${CURRENT_PROJECT_NAME};/name = ${NEW_NAME};/g" "$PBXPROJ_PATH"
sed -i '' "s/productName = ${CURRENT_PROJECT_NAME};/productName = ${NEW_NAME};/g" "$PBXPROJ_PATH"
sed -i '' "s/\/\* ${CURRENT_PROJECT_NAME} \*\//\/* ${NEW_NAME} *\//g" "$PBXPROJ_PATH"
sed -i '' "s/PBXNativeTarget \"${CURRENT_PROJECT_NAME}\"/PBXNativeTarget \"${NEW_NAME}\"/g" "$PBXPROJ_PATH"
sed -i '' "s/PBXProject \"${CURRENT_PROJECT_NAME}\"/PBXProject \"${NEW_NAME}\"/g" "$PBXPROJ_PATH"
sed -i '' "s/path = ${CURRENT_PROJECT_NAME}\//path = ${NEW_NAME}\//g" "$PBXPROJ_PATH"

# 修复/同步 build settings 中的路径（避免重命名后仍指向旧目录导致构建报错）
# 典型报错：CODE_SIGN_ENTITLEMENTS 仍指向 ${CURRENT_PROJECT_NAME}/${CURRENT_PROJECT_NAME}.entitlements
sed -i '' "s/CODE_SIGN_ENTITLEMENTS = ${CURRENT_PROJECT_NAME}\//CODE_SIGN_ENTITLEMENTS = ${NEW_NAME}\//g" "$PBXPROJ_PATH"
sed -i '' "s/INFOPLIST_FILE = ${CURRENT_PROJECT_NAME}\//INFOPLIST_FILE = ${NEW_NAME}\//g" "$PBXPROJ_PATH"
sed -i '' "s/CODE_SIGN_ENTITLEMENTS = ${CURRENT_PROJECT_NAME}\.entitlements/CODE_SIGN_ENTITLEMENTS = ${NEW_NAME}.entitlements/g" "$PBXPROJ_PATH"
echo -e "  ✓ 已修改 project.pbxproj"

# 2. 重命名源代码目录
echo -e "${BLUE}[2/5] 重命名源代码目录...${NC}"
NEW_SOURCE_DIR="${PROJECT_ROOT}/${NEW_NAME}"
if [[ "${HAS_SOURCE_DIR_RENAME}" == "true" ]]; then
    if [ -d "$NEW_SOURCE_DIR" ]; then
        echo -e "${RED}错误: 目标目录已存在: ${NEW_SOURCE_DIR}${NC}"
        exit 1
    fi
    mv "$CURRENT_SOURCE_DIR" "$NEW_SOURCE_DIR"
    echo -e "  ✓ 已重命名 ${CURRENT_PROJECT_NAME}/ -> ${NEW_NAME}/"
else
    NEW_SOURCE_DIR="${PROJECT_ROOT}"
    echo -e "  - 跳过（未使用 ${CURRENT_PROJECT_NAME}/ 同名源码目录）"
fi

# 3. 重命名 .xcodeproj 目录
echo -e "${BLUE}[3/5] 重命名 .xcodeproj 目录...${NC}"
NEW_XCODEPROJ="${PROJECT_ROOT}/${NEW_NAME}.xcodeproj"
if [ -d "$NEW_XCODEPROJ" ]; then
    echo -e "${RED}错误: 目标目录已存在: ${NEW_XCODEPROJ}${NC}"
    exit 1
fi
mv "$CURRENT_XCODEPROJ" "$NEW_XCODEPROJ"
echo -e "  ✓ 已重命名 ${CURRENT_PROJECT_NAME}.xcodeproj -> ${NEW_NAME}.xcodeproj"

# 3.1 重命名 entitlements 文件（若存在旧命名）
OLD_ENTITLEMENTS="${PROJECT_ROOT}/${CURRENT_PROJECT_NAME}.entitlements"
NEW_ENTITLEMENTS="${PROJECT_ROOT}/${NEW_NAME}.entitlements"
if [ -f "$OLD_ENTITLEMENTS" ] && [ ! -f "$NEW_ENTITLEMENTS" ]; then
    mv "$OLD_ENTITLEMENTS" "$NEW_ENTITLEMENTS"
    echo -e "  ✓ 已重命名 ${CURRENT_PROJECT_NAME}.entitlements -> ${NEW_NAME}.entitlements"
fi

# 4. 更新 scheme 文件
echo -e "${BLUE}[4/5] 更新 scheme 相关文件...${NC}"
SCHEMES_DIR="${NEW_XCODEPROJ}/xcshareddata/xcschemes"
if [ -d "$SCHEMES_DIR" ]; then
    OLD_SCHEME="${SCHEMES_DIR}/${CURRENT_PROJECT_NAME}.xcscheme"
    NEW_SCHEME="${SCHEMES_DIR}/${NEW_NAME}.xcscheme"
    if [ -f "$OLD_SCHEME" ]; then
        # 修改 scheme 内容
        sed -i '' "s/${CURRENT_PROJECT_NAME}/${NEW_NAME}/g" "$OLD_SCHEME"
        mv "$OLD_SCHEME" "$NEW_SCHEME"
        echo -e "  ✓ 已重命名 scheme 文件"
    fi
fi

# 更新 xcschememanagement.plist
USERDATA_DIR="${NEW_XCODEPROJ}/xcuserdata"
if [ -d "$USERDATA_DIR" ]; then
    find "$USERDATA_DIR" -name "xcschememanagement.plist" -exec sed -i '' "s/${CURRENT_PROJECT_NAME}\.xcscheme/${NEW_NAME}.xcscheme/g" {} \;
    echo -e "  ✓ 已更新 xcschememanagement.plist"
fi

# 5. 更新 workspace 引用（如果有独立的 .xcworkspace）
echo -e "${BLUE}[5/5] 检查并更新 workspace 引用...${NC}"
OLD_WORKSPACE="${PROJECT_ROOT}/${CURRENT_PROJECT_NAME}.xcworkspace"
if [ -d "$OLD_WORKSPACE" ]; then
    NEW_WORKSPACE="${PROJECT_ROOT}/${NEW_NAME}.xcworkspace"
    mv "$OLD_WORKSPACE" "$NEW_WORKSPACE"
    # 更新 workspace 内容
    WORKSPACE_DATA="${NEW_WORKSPACE}/contents.xcworkspacedata"
    if [ -f "$WORKSPACE_DATA" ]; then
        sed -i '' "s/${CURRENT_PROJECT_NAME}\.xcodeproj/${NEW_NAME}.xcodeproj/g" "$WORKSPACE_DATA"
    fi
    echo -e "  ✓ 已重命名并更新 workspace"
else
    echo -e "  - 未发现独立的 workspace 文件"
fi

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  ✓ 项目名称修改完成！${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "新项目名称: ${GREEN}${NEW_NAME}${NC}"
echo -e "新项目路径: ${GREEN}${NEW_XCODEPROJ}${NC}"
echo -e "源代码目录: ${GREEN}${NEW_SOURCE_DIR}${NC}"
echo ""
echo -e "${YELLOW}提示:${NC}"
echo "  1. 请在 Xcode 中重新打开项目以确认更改"
echo ""
