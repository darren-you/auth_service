#!/usr/bin/env bash
# shellcheck shell=bash

# =========================================
# 基础配置
# =========================================
PROJECT_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_NAME="auth_service_server"
DEPLOY_SERVER_PROFILE="host_124_221_158_155"

# =========================================
# 共享部署默认值
# =========================================
# shellcheck disable=SC1091
source "$PROJECT_ROOT/../deploy_shell/shared/load_deploy_profile.sh"
apply_deploy_profile_defaults "$DEPLOY_SERVER_PROFILE" "server"

# =========================================
# 项目特定
# =========================================
REMOTE_CONTAINER_PORT="8080"
SECRET_BASE_ENABLED="true"
SECRET_BASE_REPO="auth_service"
SECRET_BASE_SUBPROJECT="template_server"
DOCKERFILE_PATH="$PROJECT_ROOT/Dockerfile"
DOCKER_BUILD_CONTEXT="$PROJECT_ROOT/.."
BASE_IMAGE_REGISTRY_CANDIDATES="docker.m.daocloud.io,docker.io"

# 生产环境
PROD_HOST_PORT="8095"
PROD_CONTAINER_IP=""
PROD_LOG_PATH=""

# 测试环境
TEST_HOST_PORT="8105"
TEST_CONTAINER_IP=""
TEST_LOG_PATH=""

# =========================================
# 通知配置
# =========================================
WECHAT_WEBHOOK_URL="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=e2856bcc-f9a7-4352-95e5-1c6bdc5c6f75"
JENKINS_BUILD_URL_BASE="https://jenkins.xdarren.com/job/${PROJECT_NAME}"
