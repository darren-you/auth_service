# deploy_nginx 部署脚本说明

脚本目录：`deploy_shell/deploy_nginx/`

统一配置方式：通过 `--config` 显式传入 `deploy_config.sh`。

## 脚本清单

- `remote_deploy_pipeline.sh`：总控脚本（构建、部署、通知）
- `nginx_build_package.sh`：构建阶段（配置存在性检查，可选本地语法校验）
- `send_notification.sh`：企业微信通知
- `common.sh`：公共函数

## 使用方式

在壳工程根目录执行。示例中的 `PROJECT_ROOT` 请替换为当前业务仓库绝对路径：

```bash
bash deploy_shell/deploy_nginx/remote_deploy_pipeline.sh --config "$PROJECT_ROOT/template_nginx/deploy_config.sh"
```

可选分步调试：

```bash
bash deploy_shell/deploy_nginx/nginx_build_package.sh --config "$PROJECT_ROOT/template_nginx/deploy_config.sh"
bash deploy_shell/deploy_nginx/remote_deploy_pipeline.sh --config "$PROJECT_ROOT/template_nginx/deploy_config.sh"
```

说明：直接执行 `remote_deploy_pipeline.sh` 时，会发送构建与部署通知（成功/失败）；若未配置 `WECHAT_WEBHOOK_URL`，则自动跳过通知。

## deploy_config.sh 示例

```bash
#!/usr/bin/env bash
# shellcheck shell=bash

PROJECT_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
SERVER_DIR="$(cd -- "$PROJECT_ROOT/.." && pwd)"

# 必须满足命名规范：xxx_server / xxx_app / xxx_web / xxx_nginx
PROJECT_NAME="example_nginx"
BUILD_ENV="production"

LOCAL_NGINX_CONF="$PROJECT_ROOT/nginx.conf"

DEPLOY_HOST="127.0.0.1"
DEPLOY_PORT="22"
DEPLOY_USER="ubuntu"
DEPLOY_SSH_PASSWORD=""
DEPLOY_SSH_KEY_PATH=""
DEPLOY_SSH_OPTIONS="-o StrictHostKeyChecking=no"

REMOTE_NGINX_CONF="/etc/nginx/nginx.conf"
REMOTE_TMP_DIR="/tmp"
REMOTE_MODE="644"
REMOTE_USE_SUDO="true"
REMOTE_NGINX_TEST_CMD="nginx -t"
REMOTE_NGINX_RELOAD_CMD="nginx -s reload"

NGINX_LOCAL_TEST_ENABLED="false"
# NGINX_LOCAL_TEST_CMD="nginx -t -c $LOCAL_NGINX_CONF"

WECHAT_WEBHOOK_URL="your_wechat_webhook"
```
