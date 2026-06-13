#!/usr/bin/env bash

# =============================================================================
# macOS Bundle Identifier 修改脚本
# 用途: 将macOS原生工程的包名修改为指定的 Bundle Identifier
# 依赖 deploy_config.sh 配置项：
#   - APP_BUNDLE_ID（必填）：目标 Bundle Identifier
# =============================================================================

usage() {
    cat <<'EOF'
用法:
  change_bundle_id.sh --config <deploy_config.sh绝对路径>
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
unset APP_BUNDLE_ID

# shellcheck disable=SC1090
source "${DEPLOY_CONFIG_FILE}"

if [ -z "${APP_BUNDLE_ID:-}" ]; then
    echo "错误: deploy_config.sh 未配置 APP_BUNDLE_ID"
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

NEW_BUNDLE_ID="$APP_BUNDLE_ID"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}  macOS Bundle Identifier 修改工具${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""

# 检查文件是否存在
if [ -z "$PBXPROJ_PATH" ] || [ ! -f "$PBXPROJ_PATH" ]; then
    echo -e "${RED}错误: 找不到 project.pbxproj 文件${NC}"
    echo "路径: ${PBXPROJ_PATH:-<未找到>}"
    exit 1
fi

# 获取当前 Bundle ID
CURRENT_BUNDLE_ID=$(grep -o 'PRODUCT_BUNDLE_IDENTIFIER = [^;]*' "$PBXPROJ_PATH" | head -1 | sed 's/PRODUCT_BUNDLE_IDENTIFIER = //')

echo -e "项目路径: ${GREEN}${PROJECT_ROOT}${NC}"
echo -e "当前 Bundle ID: ${YELLOW}${CURRENT_BUNDLE_ID}${NC}"
echo -e "目标 Bundle ID: ${GREEN}${NEW_BUNDLE_ID}${NC}"
echo ""

# 检查是否需要修改
if [ "$CURRENT_BUNDLE_ID" == "$NEW_BUNDLE_ID" ]; then
    echo -e "${GREEN}✓ Bundle ID 已经是目标值，无需修改${NC}"
    exit 0
fi

# 执行替换
sed -i '' "s/PRODUCT_BUNDLE_IDENTIFIER = ${CURRENT_BUNDLE_ID}/PRODUCT_BUNDLE_IDENTIFIER = ${NEW_BUNDLE_ID}/g" "$PBXPROJ_PATH"

# 验证修改
NEW_VALUE=$(grep -o 'PRODUCT_BUNDLE_IDENTIFIER = [^;]*' "$PBXPROJ_PATH" | head -1 | sed 's/PRODUCT_BUNDLE_IDENTIFIER = //')

if [ "$NEW_VALUE" == "$NEW_BUNDLE_ID" ]; then
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  ✓ Bundle ID 修改成功！${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo -e "新的 Bundle ID: ${GREEN}${NEW_BUNDLE_ID}${NC}"
    echo ""
    echo -e "${YELLOW}提示: 请在 Xcode 中重新打开项目以确认更改${NC}"
else
    echo -e "${RED}✗ 修改失败，请检查 project.pbxproj 后重试${NC}"
    exit 1
fi
