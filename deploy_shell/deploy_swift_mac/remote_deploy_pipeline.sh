#!/bin/sh

if [ -z "${BASH_VERSION:-}" ]; then
    exec bash "$0" "$@"
fi

# ============================================================================
# macOS 打包流水线脚本
# 依赖 deploy_config.sh 配置项：
#   - 必填：PROJECT_NAME
#   - 可选：PROJECT_ROOT（未配置则使用 deploy_config.sh 所在目录）
#   - 可选：PROJECT_FILE、SCHEME_NAME、WORKSPACE_NAME、USE_WORKSPACE、PODS_TARGET_NAME、APP_NAME（未配置则自动识别）
#   - 可选：BuildBranch、BuildType、BuildEnv、BuildPodUpdate
#   - 可选：DevelopmentTeam、CodeSignStyle
#   - 可选：APP_BUNDLE_ID（兼容旧字段 AppBundleID）
#   - 可选：AppStoreUsername、AppStorePassword、AppStoreAppleID、AppStoreProviderPublicID
#   - 可选兼容字段：APP_STORE_USERNAME、APP_STORE_PASSWORD、APP_STORE_APPLE_ID、APP_STORE_PROVIDER_PUBLIC_ID
# ============================================================================

# ============================================================================
# 环境变量配置
# ============================================================================

# 设置UTF-8编码（修复CocoaPods编码问题）
export LANG=en_US.UTF-8
export LC_ALL=en_US.UTF-8

# Xcode工具路径（确保xcodebuild可用）
export PATH="/Applications/Xcode.app/Contents/Developer/usr/bin:/opt/homebrew/bin:/usr/local/bin:$PATH"

validate_project_name() {
    local value="${1:-}"
    if [[ -z "$value" ]]; then
        echo "[ERROR] PROJECT_NAME 不能为空，请在 deploy_config.sh 中定义。命名规范：xxx_app" >&2
        exit 1
    fi
    if [[ ! "$value" =~ ^[a-z0-9]+(_[a-z0-9]+)*_app$ ]]; then
        echo "[ERROR] PROJECT_NAME 不符合命名规范: ${value}。仅支持下划线命名，示例：xxx_app" >&2
        exit 1
    fi
}

# ============================================================================
# 参数解析与配置加载
# ============================================================================

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_CONFIG_FILE=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --config)
            [[ $# -ge 2 ]] || {
                echo "[ERROR] --config 缺少路径参数" >&2
                exit 1
            }
            DEPLOY_CONFIG_FILE="$2"
            shift 2
            ;;
        *)
            echo "[ERROR] 未知参数: $1" >&2
            echo "[ERROR] 用法: bash deploy_shell/deploy_swift_mac/remote_deploy_pipeline.sh --config <deploy_config.sh绝对路径>" >&2
            exit 1
            ;;
    esac
done

if [[ -z "${DEPLOY_CONFIG_FILE}" ]]; then
    echo "[ERROR] 必须传入 --config <deploy_config.sh绝对路径>" >&2
    exit 1
fi

if [[ ! -f "${DEPLOY_CONFIG_FILE}" ]]; then
    echo "[ERROR] 找不到 deploy_config.sh: ${DEPLOY_CONFIG_FILE}" >&2
    exit 1
fi
export DEPLOY_CONFIG_FILE

CONFIG_DIR="$(cd -- "$(dirname -- "${DEPLOY_CONFIG_FILE}")" && pwd)"
DEPLOY_CONFIG_FILE="${CONFIG_DIR}/$(basename -- "${DEPLOY_CONFIG_FILE}")"
# shellcheck disable=SC1090
source "${DEPLOY_CONFIG_FILE}"
validate_project_name "${PROJECT_NAME:-}"

if [[ -n "${PROJECT_ROOT:-}" ]]; then
    case "${PROJECT_ROOT}" in
        /*) ;;
        *) PROJECT_ROOT="${CONFIG_DIR}/${PROJECT_ROOT}" ;;
    esac
else
    PROJECT_ROOT="${CONFIG_DIR}"
fi

if [[ ! -d "${PROJECT_ROOT}" ]]; then
    echo "[ERROR] PROJECT_ROOT 目录不存在: ${PROJECT_ROOT}" >&2
    exit 1
fi

PROJECT_ROOT="$(cd -- "${PROJECT_ROOT}" && pwd)"
cd "${PROJECT_ROOT}" || exit 1
echo "📁📁📁使用项目目录: ${PROJECT_ROOT}"

# ============================================================================
# 配置参数定义区域
# ============================================================================

# 应用基本信息
app_name="${APP_NAME:-""}"
scheme_name="${SCHEME_NAME:-""}"  # Xcode Scheme名称（未配置则自动识别）
workspace_name="${WORKSPACE_NAME:-""}"  # 如果使用CocoaPods则用workspace（未配置则自动识别）
project_name="${PROJECT_FILE:-""}"  # 如果不使用CocoaPods则用project（未配置则自动识别）
use_workspace="${USE_WORKSPACE:-""}"  # true=使用workspace，false=使用project（未配置则自动识别）
pods_target_name="${PODS_TARGET_NAME:-""}"

version=""
version_code=""

# 本地构建产物目录
local_build_dir="/Users/darrenyou/BuildPackages/app"

# Git分支相关
remote=${BuildBranch:-"master"}

# 打包类型：ad-hoc（测试包）、app-store（市场包）
BuildType=${BuildType:-"app-store"}

# 编译配置（Debug/Release）
configuration="Release"

# 开发团队ID（留空则自动从项目读取）
development_team=${DevelopmentTeam:-""}

# 代码签名方式（Automatic/Manual）
code_sign_style=${CodeSignStyle:-"Automatic"}

# 从项目读取的Team ID（延迟读取）
project_team_id=""

# 导出配置
export_options_plist="ExportOptions.plist"

# 产物路径
build_path="$(pwd)/build"
export_path="${build_path}/package"
symbols_path="${build_path}/symbols"
archive_path=""
build_output_path=""
artifact_timestamp=""

# Jenkins相关传入配置
build_id=${BUILD_NUMBER}
environment=${BuildEnv:-"test"}
pod_update=${BuildPodUpdate:-"false"}

# App Store配置
app_store_username=${AppStoreUsername:-${APP_STORE_USERNAME:-"darren-you@outlook.com"}}
app_store_password=${AppStorePassword:-${APP_STORE_PASSWORD:-"fxiy-tyqj-evsp-uhfv"}}
app_store_apple_id=${AppStoreAppleID:-${APP_STORE_APPLE_ID:-""}}
app_store_provider_public_id=${AppStoreProviderPublicID:-${APP_STORE_PROVIDER_PUBLIC_ID:-""}}
app_bundle_id=${APP_BUNDLE_ID:-${AppBundleID:-""}}
build_error_msg=""

detect_macos_project_context() {
    local candidates=()
    local proj=""
    local name=""

    if [[ -n "${project_name}" ]]; then
        if [[ ! -d "${project_name}" ]]; then
            echo "❌❌❌指定的 PROJECT_FILE 不存在: ${project_name}" >&2
            exit 1
        fi
    else
        while IFS= read -r proj; do
            proj="${proj#./}"
            candidates+=("$proj")
        done < <(find . -maxdepth 1 -type d -name "*.xcodeproj" ! -name "Pods.xcodeproj" | sort)

        if [[ ${#candidates[@]} -eq 0 ]]; then
            echo "❌❌❌未找到 .xcodeproj，无法继续构建" >&2
            exit 1
        fi

        if [[ ${#candidates[@]} -eq 1 ]]; then
            project_name="${candidates[0]}"
        else
            local matched=()
            for proj in "${candidates[@]}"; do
                name="$(basename "$proj" .xcodeproj)"
                if [[ -d "${name}" ]]; then
                    matched+=("$proj")
                fi
            done

            if [[ ${#matched[@]} -eq 1 ]]; then
                project_name="${matched[0]}"
            else
                echo "❌❌❌检测到多个 .xcodeproj，无法自动识别项目：" >&2
                for proj in "${candidates[@]}"; do
                    echo "  - $(basename "$proj" .xcodeproj)" >&2
                done
                echo "请在 deploy_config.sh 中配置 PROJECT_FILE，例如 PROJECT_FILE=\"xxx.xcodeproj\"" >&2
                exit 1
            fi
        fi
    fi

    local project_base
    project_base="$(basename "${project_name}" .xcodeproj)"

    if [[ -z "${workspace_name}" ]]; then
        if [[ -d "${project_base}.xcworkspace" ]]; then
            workspace_name="${project_base}.xcworkspace"
        else
            local ws_candidates=()
            while IFS= read -r proj; do
                proj="${proj#./}"
                ws_candidates+=("$proj")
            done < <(find . -maxdepth 1 -type d -name "*.xcworkspace" ! -name "Pods.xcworkspace" | sort)

            if [[ ${#ws_candidates[@]} -eq 1 ]]; then
                workspace_name="${ws_candidates[0]}"
            fi
        fi
    fi

    if [[ -z "${use_workspace}" ]]; then
        if [[ -n "${workspace_name}" && -d "${workspace_name}" ]]; then
            use_workspace=true
        else
            use_workspace=false
        fi
    fi

    if [[ -z "${scheme_name}" ]]; then
        if [[ -f "${project_name}/xcshareddata/xcschemes/${project_base}.xcscheme" ]]; then
            scheme_name="${project_base}"
        else
            local scheme_candidates=()
            while IFS= read -r proj; do
                proj="${proj#./}"
                scheme_candidates+=("$(basename "$proj" .xcscheme)")
            done < <(find "${project_name}/xcshareddata/xcschemes" -maxdepth 1 -type f -name "*.xcscheme" 2>/dev/null | sort)

            if [[ ${#scheme_candidates[@]} -eq 1 ]]; then
                scheme_name="${scheme_candidates[0]}"
            else
                scheme_name="${project_base}"
            fi
        fi
    fi

    if [[ -z "${app_name}" ]]; then
        app_name="${project_base}"
    fi

    if [[ -z "${pods_target_name}" ]]; then
        pods_target_name="${scheme_name}"
    fi

    archive_path="${build_path}/${app_name}.xcarchive"
    build_output_path="${export_path}/${app_name}.pkg"

    echo "📱📱📱项目识别结果: project=${project_name}, workspace=${workspace_name:-<无>}, scheme=${scheme_name}, use_workspace=${use_workspace}, pods_target=${pods_target_name}"
}

detect_macos_project_context

# ============================================================================
# 功能函数定义区域
# ============================================================================

# 从Xcode项目配置读取版本号和Team ID
get_version_from_project() {
    echo "📱 从Xcode项目读取版本信息..."

    # 读取版本号优先使用 xcodeproj，避免 workspace 在 pod install 之前解析失败
    if [ -d "${project_name}" ]; then
        build_settings_target="-project ${project_name}"
        echo "📱 版本读取目标: ${project_name}"
    elif [ "${use_workspace}" = "true" ]; then
        build_settings_target="-workspace ${workspace_name}"
        echo "📱 版本读取目标: ${workspace_name}"
    else
        build_settings_target="-project ${project_name}"
        echo "📱 版本读取目标: ${project_name}"
    fi

    # 获取完整的 Build Settings，并保留错误输出用于排查
    xcodebuild_err_file=$(mktemp /tmp/macos_version_read.XXXXXX)
    build_settings=$(xcodebuild -showBuildSettings \
        ${build_settings_target} \
        -scheme "${scheme_name}" \
        -configuration "${configuration}" 2>"${xcodebuild_err_file}")
    xcodebuild_status=$?
    xcodebuild_err=$(sed -n '1,40p' "${xcodebuild_err_file}")
    rm -f "${xcodebuild_err_file}"

    if [ $xcodebuild_status -ne 0 ]; then
        echo "⚠️⚠️⚠️xcodebuild读取版本失败（退出码: ${xcodebuild_status}）"
        [ -n "${xcodebuild_err}" ] && echo "${xcodebuild_err}"
    fi
    
    # 读取MARKETING_VERSION (版本号)
    version=$(echo "${build_settings}" | grep -m1 "MARKETING_VERSION" | sed 's/.*= //')
    
    # 读取CURRENT_PROJECT_VERSION (构建号)
    version_code=$(echo "${build_settings}" | grep -m1 "CURRENT_PROJECT_VERSION" | sed 's/.*= //')
    
    # 读取DEVELOPMENT_TEAM (如果未通过环境变量指定)
    if [ -z "${development_team}" ]; then
        project_team_id=$(echo "${build_settings}" | grep -m1 "DEVELOPMENT_TEAM" | sed 's/.*= //')
        project_team_id=$(echo "${project_team_id}" | xargs)
        
        if [ -n "${project_team_id}" ]; then
            development_team="${project_team_id}"
            echo "📱 从项目读取 Team ID: ${development_team}"
        fi
    fi
    
    # 去除可能的空白字符
    version=$(echo "${version}" | xargs)
    version_code=$(echo "${version_code}" | xargs)

    # 兜底：如果 xcodebuild 未读到版本，则直接从 pbxproj 读取
    if [ -z "${version}" ] || [ -z "${version_code}" ]; then
        pbxproj_path="${project_name}/project.pbxproj"
        if [ -f "${pbxproj_path}" ]; then
            echo "⚠️⚠️⚠️Build Settings未读到完整版本，改为从 ${pbxproj_path} 兜底读取"
            if [ -z "${version}" ]; then
                version=$(grep -m1 "MARKETING_VERSION = " "${pbxproj_path}" | sed 's/.*= //; s/;//')
                version=$(echo "${version}" | xargs)
            fi
            if [ -z "${version_code}" ]; then
                version_code=$(grep -m1 "CURRENT_PROJECT_VERSION = " "${pbxproj_path}" | sed 's/.*= //; s/;//')
                version_code=$(echo "${version_code}" | xargs)
            fi
            if [ -z "${development_team}" ]; then
                project_team_id=$(grep -m1 "DEVELOPMENT_TEAM = " "${pbxproj_path}" | sed 's/.*= //; s/;//')
                project_team_id=$(echo "${project_team_id}" | xargs)
                if [ -n "${project_team_id}" ]; then
                    development_team="${project_team_id}"
                    echo "📱 从pbxproj读取 Team ID: ${development_team}"
                fi
            fi
        fi
    fi
    
    # 如果读取失败，设置默认值
    if [ -z "${version}" ]; then
        echo "⚠️⚠️⚠️未能从项目读取MARKETING_VERSION，使用默认值"
        version="1.0.0"
    fi
    
    if [ -z "${version_code}" ]; then
        echo "⚠️⚠️⚠️未能从项目读取CURRENT_PROJECT_VERSION，使用默认值"
        version_code="1"
    fi
    
    echo "📱 当前版本号: ${version}"
    echo "📱 当前构建号: ${version_code}"
}

# 更新项目配置中的构建号
update_build_version_in_project() {
    # 自动递增构建号
    new_version_code=$((version_code + 1))
    
    echo "⚙️⚙️⚙️更新构建号: ${version_code} -> ${new_version_code}"
    
    # 使用agvtool更新构建号（需要在项目中启用）
    # 注意：agvtool在有多个项目的目录中不工作，因此默认在构建时指定版本号
    project_count=$(find . -maxdepth 1 -type d -name "*.xcodeproj" ! -name "Pods.xcodeproj" | wc -l | tr -d ' ')
    if command -v agvtool &> /dev/null && [ "${project_count}" -eq 1 ]; then
        # 只在没有多个项目时使用agvtool
        agvtool new-version -all ${new_version_code} 2>/dev/null
        if [ $? -eq 0 ]; then
            echo "✅ 使用agvtool更新构建号成功"
        else
            echo "⚠️⚠️⚠️agvtool执行失败，将在构建时指定版本号"
        fi
    else
        if [ "${project_count}" -gt 1 ]; then
            echo "⚠️⚠️⚠️检测到多个xcodeproj(${project_count}个)，跳过agvtool"
        else
            echo "⚠️⚠️⚠️agvtool不可用，将在构建时指定版本号"
        fi
        echo "提示: 构建时会通过命令行参数 CURRENT_PROJECT_VERSION=${new_version_code} 来设置版本号"
    fi
    
    version_code=${new_version_code}
}

# 确保 Git 提交身份可用（统一规则：必须有 user.email）
ensure_git_commit_identity() {
    local default_name=""
    local default_email=""

    default_name="$(git config --global --get user.name 2>/dev/null || true)"
    default_email="$(git config --global --get user.email 2>/dev/null || true)"

    if [[ -z "${default_name}" ]]; then
        default_name="${USER:-jenkins}"
    fi

    if [[ -z "${default_email}" ]]; then
        notify_failure_and_exit "未检测到 Git user.email，请在 Jenkins 机器上执行: git config --global user.email \"your-email@example.com\""
    fi

    git config user.name "${default_name}"
    git config user.email "${default_email}"
}

# 修改Build号
update_build_version() {
    branch=${remote##*/}
    value=$(git branch --list ${branch})
    if [ "${value}" != "" ]; then
        git checkout ${branch}
    else
        git checkout -b ${branch} ${remote}
    fi
    git pull
    git push --set-upstream origin ${branch}

    # 从Xcode项目读取版本信息
    get_version_from_project
    
    # 更新构建号
    update_build_version_in_project
    
    echo "⚙️⚙️⚙️修改版本号: ${version}"
    echo "⚙️⚙️⚙️构建号: ${version_code}"
    
    # 暂存所有更改
    git add .
    
    # 检查是否有实际的更改需要提交
    if git diff --cached --quiet; then
        echo "⏭️⏭️⏭️没有需要提交的更改（版本号已是最新）"
    else
        ensure_git_commit_identity
        git commit -m "修改Build号为: ${version_code}"
        git push
        echo "✅ 修改已推送至git服务器"
    fi
}

# 企业微信机器人通知
send_notification() {
    local build_status=$1
    local branch_name=${remote}
    local notification_cmd=()
    
    # 产物绝对路径（用于通知显示）- macOS所有包都上传到版本号目录
    artifact_path="${build_output_path:-${local_build_dir}/${app_name}/macos/${version}/${app_name}_${BuildType}_V${version}_${version_code}_${environment}_${artifact_timestamp}.pkg}"

    # 检查通知脚本是否存在
    notification_script="${SCRIPT_DIR}/notification_mac.sh"
    if [ ! -f "${notification_script}" ]; then
        echo "⚠️⚠️⚠️通知脚本不存在，跳过发送通知"
        return
    fi

    notification_cmd=(
        bash "${notification_script}"
        --config "${DEPLOY_CONFIG_FILE}" \
        --version "${version}" \
        --version_code "${version_code}" \
        --branch_name "${branch_name}" \
        --build_id "${build_id}" \
        --artifact_path "${artifact_path}" \
        --channel "${BuildType}" \
        --build_status "${build_status}" \
        --environment "${environment}"
    )

    if [[ "${build_status}" != "success" && -n "${build_error_msg}" ]]; then
        notification_cmd+=(--error "${build_error_msg}")
    fi

    "${notification_cmd[@]}"
}

extract_error_from_log() {
    local log_file="$1"
    local fallback_message="$2"
    local error_line=""
    local raw_error_line=""
    local shell_error_line=""
    local summary=""

    if [[ -s "$log_file" ]]; then
        raw_error_line="$(grep -E '(^error:|^Error:|^ERROR:|BUILD FAILED|FAILED|Validation failed|UPLOAD FAILED|Failed to upload package|This bundle is invalid|unable to|not found|No such file|No profiles for|Code signing is required|Provisioning profile|CocoaPods could not find compatible versions)' "$log_file" | tail -n 1 || true)"
        if [[ -n "$raw_error_line" ]]; then
            error_line="$(printf '%s' "$raw_error_line")"
        fi

        if [[ -z "$error_line" ]]; then
            shell_error_line="$(grep -E '❌|\[ERROR\]|error:' "$log_file" | tail -n 1 || true)"
            if [[ -n "$shell_error_line" ]]; then
                error_line="$(printf '%s' "$shell_error_line" | sed 's/^.*\[ERROR\] //')"
            fi
        fi

        if [[ -n "$error_line" ]]; then
            printf '%s' "$error_line"
            return 0
        fi

        summary="$(tail -n 20 "$log_file" | tr '\n' ' ' | sed 's/[[:space:]]\+/ /g; s/^ //; s/ $//')"
        if [[ -n "$summary" ]]; then
            printf '%s' "$summary"
            return 0
        fi
    fi

    printf '%s' "$fallback_message"
}

resolve_exported_artifact() {
    local artifact=""

    artifact=$(find "${export_path}" -type f -name "*.pkg" | head -n 1)
    if [[ -n "${artifact}" ]]; then
        echo "${artifact}"
        return 0
    fi

    artifact=$(find "${export_path}" -type f -name "*.ipa" | head -n 1)
    if [[ -n "${artifact}" ]]; then
        echo "${artifact}"
        return 0
    fi

    artifact=$(find "${export_path}" -type d -name "*.app" | head -n 1)
    if [[ -n "${artifact}" ]]; then
        echo "${artifact}"
        return 0
    fi

    return 1
}

set_failure_reason() {
    build_error_msg="$1"
}

notify_failure_and_exit() {
    local message="$1"
    set_failure_reason "$message"
    send_notification "failure"
    exit 1
}

resolve_pods_xcconfig() {
    local preferred="Pods/Target Support Files/Pods-${pods_target_name}/Pods-${pods_target_name}.release.xcconfig"
    if [ -f "${preferred}" ]; then
        echo "${preferred}"
        return 0
    fi

    local candidates=()
    local item=""
    while IFS= read -r item; do
        candidates+=("${item}")
    done < <(find "Pods/Target Support Files" -maxdepth 2 -type f -name "Pods-*.release.xcconfig" 2>/dev/null | sort)

    if [[ ${#candidates[@]} -eq 1 ]]; then
        echo "${candidates[0]}"
        return 0
    fi

    if [[ ${#candidates[@]} -gt 1 ]]; then
        local scheme_hit=""
        for item in "${candidates[@]}"; do
            if [[ "${item}" == *"Pods-${scheme_name}.release.xcconfig" ]]; then
                scheme_hit="${item}"
                break
            fi
        done
        if [[ -n "${scheme_hit}" ]]; then
            echo "${scheme_hit}"
            return 0
        fi
        return 2
    fi

    return 1
}

# 清理构建目录
clean_build() {
    echo "🧹🧹🧹清理构建目录"
    rm -rf "${build_path}"
    mkdir -p "${build_path}"
    mkdir -p "${export_path}"
    mkdir -p "${symbols_path}"
}

# Pod安装/更新
install_pods() {
    echo "📦📦📦检查CocoaPods环境..."
    local pod_log=""
    local pod_failure_reason=""
    
    # 确保UTF-8编码已设置
    if [ -z "$LANG" ] || [ -z "$LC_ALL" ]; then
        echo "⚠️⚠️⚠️环境变量未设置，强制设置UTF-8编码"
        export LANG=en_US.UTF-8
        export LC_ALL=en_US.UTF-8
    fi
    
    echo "当前编码: LANG=$LANG, LC_ALL=$LC_ALL"
    
    # 在Jenkins环境中，如果Pods目录存在但配置文件不完整，先清理
    if [ -d "Pods" ]; then
        pods_xcconfig="$(resolve_pods_xcconfig || true)"
        if [ -z "${pods_xcconfig}" ] || [ ! -f "${pods_xcconfig}" ]; then
            echo "⚠️⚠️⚠️检测到不完整的Pods目录，先清理..."
            rm -rf Pods
            rm -rf Podfile.lock
            echo "✅ Pods目录清理完成"
        fi
    fi
    
    pod_log="${build_path}/cocoapods_$(date +%Y%m%d_%H%M%S).log"

    if [ "${pod_update}" = "true" ]; then
        echo "📦📦📦开始执行pod update"
        pod update --verbose 2>&1 | tee "${pod_log}"
        pod_result=${PIPESTATUS[0]}
    else
        echo "📦📦📦开始执行pod install"
        pod install --verbose 2>&1 | tee "${pod_log}"
        pod_result=${PIPESTATUS[0]}
    fi
    
    if [ $pod_result -ne 0 ]; then
        echo "❌❌❌CocoaPods安装失败！退出码: ${pod_result}"
        echo "尝试清理后重新安装..."
        rm -rf Pods
        rm -rf Podfile.lock
        pod install --verbose --repo-update 2>&1 | tee -a "${pod_log}"
        pod_result=${PIPESTATUS[0]}
        
        if [ $pod_result -ne 0 ]; then
            echo "❌❌❌CocoaPods重试仍然失败！"
            pod_failure_reason="$(extract_error_from_log "${pod_log}" "CocoaPods重试仍然失败，最后退出码: ${pod_result}")"
            notify_failure_and_exit "${pod_failure_reason}"
        fi
    fi
    
    # 验证关键文件是否存在
    pods_xcconfig="$(resolve_pods_xcconfig || true)"
    if [ -z "${pods_xcconfig}" ] || [ ! -f "${pods_xcconfig}" ]; then
        echo "❌❌❌CocoaPods配置文件不存在或无法唯一识别"
        echo "可能原因："
        echo "  1. Pod安装失败但未正确报错"
        echo "  2. Target名称不匹配"
        echo "  3. Podfile配置错误"
        echo ""
        echo "Pods目录结构："
        ls -la Pods/Target\ Support\ Files/ 2>/dev/null || echo "Pods/Target Support Files 目录不存在"
        echo ""
        echo "检测到的 release xcconfig："
        find "Pods/Target Support Files" -maxdepth 2 -type f -name "Pods-*.release.xcconfig" 2>/dev/null | sed 's/^/  - /' || true
        echo ""
        echo "当前目录内容："
        ls -la
        notify_failure_and_exit "CocoaPods配置文件不存在或无法唯一识别，请检查 Pods/Target Support Files 与 Target 名称配置"
    fi
    
    echo "✅✅✅CocoaPods安装完成，配置文件验证通过"
    echo "配置文件路径: ${pods_xcconfig}"
}

# 编译
build() {
    echo "🚀🚀🚀开始编译 ${BuildType} 包！！！"
    echo "Server环境：${environment}"
    local archive_log=""
    local export_log=""
    local archive_failure_reason=""
    local export_failure_reason=""
    
    # 清理构建目录
    clean_build
    
    # 安装依赖
    if [ -f "Podfile" ]; then
        install_pods
    else
        echo "⏭️⏭️⏭️未检测到Podfile，跳过pod安装"
    fi
    
    # 读取版本信息（如果还没读取）
    if [ -z "${version}" ]; then
        get_version_from_project
    fi
    
    # 确定使用workspace还是project
    if [ "${use_workspace}" = "true" ]; then
        build_target="-workspace ${workspace_name}"
    else
        build_target="-project ${project_name}"
    fi
    
    # 构建Archive
    echo "📦📦📦开始Archive..."
    echo "Scheme: ${scheme_name}"
    echo "Configuration: ${configuration}"
    echo "Code Sign Style: ${code_sign_style}"
    if [ -n "${development_team}" ]; then
        echo "Development Team: ${development_team}"
    fi
    
    # 构建xcodebuild参数
    xcode_build_params="${build_target} \
        -scheme ${scheme_name} \
        -configuration ${configuration} \
        -archivePath ${archive_path} \
        -destination generic/platform=macOS \
        -allowProvisioningUpdates \
        MARKETING_VERSION=${version} \
        CURRENT_PROJECT_VERSION=${version_code} \
        ENABLE_BITCODE=NO \
        CODE_SIGN_STYLE=${code_sign_style}"
    
    # 如果指定了DEVELOPMENT_TEAM，添加到参数中
    if [ -n "${development_team}" ]; then
        xcode_build_params="${xcode_build_params} DEVELOPMENT_TEAM=${development_team}"
    fi
    
    # 检查xcpretty是否可用
    archive_log="${build_path}/xcodebuild_archive_$(date +%Y%m%d_%H%M%S).log"
    if command -v xcpretty &> /dev/null; then
        echo "使用xcpretty美化输出"
        xcodebuild archive ${xcode_build_params} 2>&1 | tee "${archive_log}" | xcpretty
        archive_result=${PIPESTATUS[0]}
    else
        echo "xcpretty未安装，使用原始输出"
        xcodebuild archive ${xcode_build_params} 2>&1 | tee "${archive_log}"
        archive_result=${PIPESTATUS[0]}
    fi
    
    if [ $archive_result -ne 0 ]; then
        echo "❌❌❌Archive失败！！！退出码: ${archive_result}"
        archive_failure_reason="$(extract_error_from_log "${archive_log}" "Archive失败，退出码: ${archive_result}")"
        notify_failure_and_exit "${archive_failure_reason}"
    fi
    
    # 验证archive文件是否存在
    if [ ! -d "${archive_path}" ]; then
        echo "❌❌❌Archive文件不存在: ${archive_path}"
        notify_failure_and_exit "Archive文件不存在: ${archive_path}"
    fi
    
    echo "✅✅✅Archive完成: ${archive_path}"
    
    # 创建ExportOptions.plist（如果不存在）
    create_export_options
    
    # 导出构建包
    echo "📤📤📤开始导出构建包..."
    
    # 检查xcpretty是否可用
    export_log="${build_path}/xcodebuild_export_$(date +%Y%m%d_%H%M%S).log"
    if command -v xcpretty &> /dev/null; then
        xcodebuild -exportArchive \
            -archivePath "${archive_path}" \
            -exportPath "${export_path}" \
            -exportOptionsPlist "${export_options_plist}" \
            -allowProvisioningUpdates 2>&1 | tee "${export_log}" | xcpretty
        export_result=${PIPESTATUS[0]}
    else
        xcodebuild -exportArchive \
            -archivePath "${archive_path}" \
            -exportPath "${export_path}" \
            -exportOptionsPlist "${export_options_plist}" \
            -allowProvisioningUpdates 2>&1 | tee "${export_log}"
        export_result=${PIPESTATUS[0]}
    fi
    
    if [ $export_result -eq 0 ]; then
        # 编译完成后生成时间戳
        artifact_timestamp=$(date +%y%m%d%H%M)
        echo "✅✅✅${BuildType}渠道${environment}环境包(V${version}_${version_code})编译完成！！！"
    else
        echo "❌❌❌${BuildType}渠道${environment}环境包(V${version}_${version_code})导出失败！！！"
        export_failure_reason="$(extract_error_from_log "${export_log}" "${BuildType}渠道${environment}环境包导出失败，退出码: ${export_result}")"
        notify_failure_and_exit "${export_failure_reason}"
    fi
}

# 创建ExportOptions.plist
create_export_options() {
    # 每次都重新生成，确保使用最新配置
    if [ -f "${export_options_plist}" ]; then
        echo "⚠️⚠️⚠️检测到旧的ExportOptions.plist，将重新生成"
        rm -f "${export_options_plist}"
    fi
    
    echo "📝📝📝创建ExportOptions.plist"
    
    # 根据BuildType确定导出方法
    if [ "${BuildType}" = "ad-hoc" ]; then
        export_method="ad-hoc"
    elif [ "${BuildType}" = "app-store" ]; then
        # Xcode 26+ 中 app-store 已废弃，改用 app-store-connect
        export_method="app-store-connect"
    else
        export_method="development"
    fi
    
    # 确保有Team ID（从项目配置读取或环境变量）
    if [ -z "${development_team}" ]; then
        echo "⚠️⚠️⚠️未设置Team ID，尝试从项目配置读取..."
        if [ -n "${project_team_id}" ]; then
            development_team="${project_team_id}"
        else
            # 最后一次尝试读取
            if [ "${use_workspace}" = "true" ] && [ -n "${workspace_name}" ] && [ -d "${workspace_name}" ]; then
                temp_team=$(xcodebuild -showBuildSettings \
                    -workspace "${workspace_name}" \
                    -scheme "${scheme_name}" \
                    -configuration "${configuration}" 2>/dev/null | \
                    grep -m1 "DEVELOPMENT_TEAM" | sed 's/.*= //' | xargs)
            else
                temp_team=$(xcodebuild -showBuildSettings \
                    -project "${project_name}" \
                    -scheme "${scheme_name}" \
                    -configuration "${configuration}" 2>/dev/null | \
                    grep -m1 "DEVELOPMENT_TEAM" | sed 's/.*= //' | xargs)
            fi
            
            if [ -n "${temp_team}" ]; then
                development_team="${temp_team}"
                echo "✅ 成功从项目读取 Team ID: ${development_team}"
            fi
        fi
    fi
    
    # 生成ExportOptions.plist
    if [ -n "${development_team}" ] && [ "${development_team}" != "YOUR_TEAM_ID" ]; then
        echo "使用 Team ID: ${development_team}"
        cat > "${export_options_plist}" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>method</key>
    <string>${export_method}</string>
    <key>teamID</key>
    <string>${development_team}</string>
    <key>uploadBitcode</key>
    <false/>
    <key>uploadSymbols</key>
    <true/>
    <key>compileBitcode</key>
    <false/>
    <key>signingStyle</key>
    <string>automatic</string>
</dict>
</plist>
EOF
    else
        echo "⚠️⚠️⚠️未能获取Team ID，使用基础配置"
        cat > "${export_options_plist}" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>method</key>
    <string>${export_method}</string>
    <key>uploadBitcode</key>
    <false/>
    <key>uploadSymbols</key>
    <true/>
    <key>compileBitcode</key>
    <false/>
    <key>signingStyle</key>
    <string>automatic</string>
</dict>
</plist>
EOF
    fi
    
    echo "✅✅✅ExportOptions.plist创建完成"
    echo "内容："
    cat "${export_options_plist}"
}

# 上传至本地目录
upload_to_server() {
    # 确保本地构建目录存在
    app_dir="${local_build_dir}/${app_name}/macos"
    mkdir -p "${app_dir}"

    # macOS所有包都上传到版本号目录
    version_dir="${app_dir}/${version}"
    mkdir -p "${version_dir}"

    # 查找导出的包文件（优先 pkg，其次 ipa，再次 app）
    exported_artifact="$(resolve_exported_artifact || true)"
    if [ -z "${exported_artifact}" ]; then
        echo "❌❌❌未找到导出的构建包"
        notify_failure_and_exit "未找到导出的构建包，导出目录: ${export_path}"
    fi

    echo "找到导出文件: ${exported_artifact}"

    artifact_extension="${exported_artifact##*.}"
    if [[ "${artifact_extension}" == "${exported_artifact}" ]]; then
        artifact_extension="pkg"
    fi
    file_name="${app_name}_${BuildType}_V${version}_${version_code}_${environment}_${artifact_timestamp}.${artifact_extension}"

    # 删除同版本同类型旧包（兼容文件与目录）
    find "${version_dir}" -maxdepth 1 -mindepth 1 -name "${app_name}_${BuildType}_V${version}_*_${environment}.*" -exec rm -rf {} +

    if [ -d "${exported_artifact}" ]; then
        cp -R "${exported_artifact}" "${version_dir}/${file_name}"
    else
        cp "${exported_artifact}" "${version_dir}/${file_name}"
    fi
    echo "✅✅✅${environment}环境：${BuildType}包已上传至${version}目录"
    echo "文件路径: ${version_dir}/${file_name}"

    # 更新build_output_path以便后续使用
    build_output_path="${version_dir}/${file_name}"

    send_notification "success"
    
    echo "✅✅✅${BuildType}包上传完成！！！"
}

# 上传到应用商店
upload_app_store() {
    echo "⏫⏫⏫开始上传构建包至AppStore"

    # App Store 上传优先使用 pkg，兼容旧链路 ipa
    exported_package=$(find "${export_path}" -type f \( -name "*.pkg" -o -name "*.ipa" \) | head -n 1)
    if [ -z "${exported_package}" ]; then
        echo "❌❌❌未找到可上传的构建包（pkg/ipa）"
        notify_failure_and_exit "未找到可上传的构建包（pkg/ipa），导出目录: ${export_path}"
    fi

    if [ -z "${app_store_apple_id}" ]; then
        echo "❌❌❌未配置 APP_STORE_APPLE_ID（或 Jenkins 参数 AppStoreAppleID）"
        notify_failure_and_exit "未配置 APP_STORE_APPLE_ID（或 Jenkins 参数 AppStoreAppleID）"
    fi
    if [ -z "${app_store_provider_public_id}" ]; then
        echo "❌❌❌未配置 APP_STORE_PROVIDER_PUBLIC_ID（或 Jenkins 参数 AppStoreProviderPublicID）"
        notify_failure_and_exit "未配置 APP_STORE_PROVIDER_PUBLIC_ID（或 Jenkins 参数 AppStoreProviderPublicID）"
    fi
    if [ -z "${app_bundle_id}" ]; then
        echo "❌❌❌未配置 APP_BUNDLE_ID（或 Jenkins 参数 AppBundleID）"
        notify_failure_and_exit "未配置 APP_BUNDLE_ID（或 Jenkins 参数 AppBundleID）"
    fi

    # 使用显式参数上传，避免 altool 自动根据 bundleID 推断 appleID 失败
    upload_log="${build_path}/altool_upload_$(date +%Y%m%d_%H%M%S).log"
    echo "📝 altool日志: ${upload_log}"

    # altool存在“命令退出码为0但返回Validation failed文本”的情况，必须同时校验日志内容
    xcrun altool --upload-package "${exported_package}" \
        -t macos \
        --provider-public-id "${app_store_provider_public_id}" \
        --apple-id "${app_store_apple_id}" \
        --bundle-version "${version_code}" \
        --bundle-short-version-string "${version}" \
        --bundle-id "${app_bundle_id}" \
        -u "${app_store_username}" \
        -p "${app_store_password}" \
        --verbose 2>&1 | tee "${upload_log}"

    build_result=${PIPESTATUS[0]}

    if [ $build_result -ne 0 ]; then
        echo "❌❌❌上传构建包至AppStore失败（altool退出码: ${build_result}）"
        notify_failure_and_exit "$(extract_error_from_log "${upload_log}" "上传构建包至AppStore失败（altool退出码: ${build_result}）")"
    fi

    if grep -Eqi "Validation failed|UPLOAD FAILED|Failed to upload package|Invalid Pre-Release Train|This bundle is invalid" "${upload_log}"; then
        local validation_error=""
        validation_error="$(grep -Eim1 "Validation failed|UPLOAD FAILED|Failed to upload package|Invalid Pre-Release Train|This bundle is invalid" "${upload_log}" || true)"
        echo "❌❌❌上传构建包至AppStore失败（检测到App Store校验失败）"
        echo "请查看日志: ${upload_log}"
        if [[ -n "${validation_error}" ]]; then
            notify_failure_and_exit "${validation_error}"
        fi
        notify_failure_and_exit "上传构建包至AppStore失败（检测到App Store校验失败），日志: ${upload_log}"
    fi

    build_output_path="${exported_package}"
    echo "✅✅✅上传构建包至AppStore完成！！！"
    send_notification "success"
}

# ============================================================================
# 主函数
# ============================================================================

main() {
    update_build_version

    build

    if [ "${BuildType}" == "ad-hoc" ]; then
        upload_to_server
    fi

    if [ "${BuildType}" == "app-store" ]; then
        upload_app_store
    fi
}

main

# ============================================================================
# 测试命令示例
# ============================================================================
# 本地测试打包ad-hoc包：
# BuildBranch=develop BuildType=ad-hoc BuildEnv=test BuildPodUpdate=false \
# bash ./remote_deploy_pipeline.sh --config "$(pwd)/deploy_config.sh"
#
# 本地测试打包app-store包：
# BuildBranch=master BuildType=app-store BuildEnv=prod BuildPodUpdate=true \
# bash ./remote_deploy_pipeline.sh --config "$(pwd)/deploy_config.sh"
#
# Jenkins参数说明（parameterized）：
# - BuildBranch: Git分支，默认 master
# - BuildType: ad-hoc 或 app-store，默认 app-store
# - BuildEnv: test 或 prod，默认 test
# - BuildPodUpdate: true 或 false，是否执行 pod update，默认 false
# - AppStoreAppleID: App Store Connect 的应用 Apple ID（BuildType=app-store 时必填）
# - AppStoreProviderPublicID: App Store Connect Provider Public ID（BuildType=app-store 时必填）
# - BUILD_NUMBER: Jenkins构建号
# ============================================================================
