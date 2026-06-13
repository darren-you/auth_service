# deploy_ssl 部署脚本说明

脚本目录：`deploy_shell/deploy_ssl/`

统一配置方式：通过 `--config` 显式传入 `deploy_config.sh`。

用途：

- 从业务仓库中的独立 `cert-renewal-hook.sh` 构建部署产物
- 将该 hook 文件同步到目标服务器 `/etc/letsencrypt/cert-renewal-hook.sh`
- 在远端执行 `bash -n` 校验，并发送企业微信通知

## 脚本清单

- `remote_deploy_pipeline.sh`：总控脚本（构建、部署、通知）
- `ssl_build_package.sh`：构建阶段（复制本地 `cert-renewal-hook.sh` 并做语法校验）
- `send_notification.sh`：企业微信通知
- `common.sh`：公共函数

## 使用方式

在壳工程根目录执行。示例中的 `PROJECT_ROOT` 请替换为当前业务仓库绝对路径：

```bash
bash deploy_shell/deploy_ssl/remote_deploy_pipeline.sh --config "$PROJECT_ROOT/remote/ssl/deploy_config.sh"
```

可选分步调试：

```bash
bash deploy_shell/deploy_ssl/ssl_build_package.sh --config "$PROJECT_ROOT/remote/ssl/deploy_config.sh"
bash deploy_shell/deploy_ssl/remote_deploy_pipeline.sh --config "$PROJECT_ROOT/remote/ssl/deploy_config.sh"
```

说明：直接执行 `remote_deploy_pipeline.sh` 时，会发送构建与部署通知（成功/失败）；若未配置 `WECHAT_WEBHOOK_URL`，则自动跳过通知。

## deploy_config.sh 示例

```bash
#!/usr/bin/env bash
# shellcheck shell=bash

PROJECT_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
SERVER_DIR="$(cd -- "$PROJECT_ROOT/../.." && pwd)"

PROJECT_NAME="example_ssl"
BUILD_ENV="production"

LOCAL_HOOK_SOURCE_FILE="$PROJECT_ROOT/certbot_renewal/scripts/cert-renewal-hook.sh"
ARTIFACT_LOCAL_PATH="/tmp/${PROJECT_NAME}-cert-renewal-hook.sh"

DEPLOY_HOST="127.0.0.1"
DEPLOY_PORT="22"
DEPLOY_USER="ubuntu"
DEPLOY_SSH_PASSWORD=""
DEPLOY_SSH_KEY_PATH=""
DEPLOY_SSH_OPTIONS="-o StrictHostKeyChecking=no"

REMOTE_HOOK_SCRIPT_PATH="/etc/letsencrypt/cert-renewal-hook.sh"
REMOTE_TMP_DIR="/tmp"
REMOTE_MODE="755"
REMOTE_USE_SUDO="true"
REMOTE_HOOK_CHECK_CMD="bash -n /etc/letsencrypt/cert-renewal-hook.sh"

WECHAT_WEBHOOK_URL="your_wechat_webhook"
```
