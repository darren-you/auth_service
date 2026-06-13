# deploy_web 部署脚本说明

脚本目录：`deploy_shell/deploy_web/`

统一配置方式：所有脚本均不再内置默认配置路径，必须通过 `--config` 显式传入 `deploy_config.sh`。

## 脚本清单

- `change_web_icons.sh`：从一张 1024x1024 源图生成 web 图标
- `remote_deploy_pipeline.sh`：总控脚本（构建、部署、通知）
- `npm_build_package.sh`：构建前端并打包 `dist.zip`
- `remote_deploy_core.sh`：内部远程部署实现（由 `remote_deploy_pipeline.sh` 调用）
- `send_notification.sh`：企业微信通知
- `common.sh`：公共函数

## 使用方式

在前端工程根目录执行。示例中的 `PROJECT_ROOT` 请替换为当前业务仓库绝对路径：

```bash
bash ../deploy_shell/deploy_web/change_web_icons.sh --config "$PROJECT_ROOT/template_web/deploy_config.sh"
bash ../deploy_shell/deploy_web/remote_deploy_pipeline.sh --config "$PROJECT_ROOT/template_web/deploy_config.sh"
```

可选分步调试：

```bash
bash ../deploy_shell/deploy_web/npm_build_package.sh --config "$PROJECT_ROOT/template_web/deploy_config.sh"
META_FILE=/tmp/web.meta bash ../deploy_shell/deploy_web/remote_deploy_core.sh --config "$PROJECT_ROOT/template_web/deploy_config.sh"
```

图标生成配置（在 `deploy_config.sh` 中）：

- `WEB_ICON_IMAGE_1024`：1024x1024 源图路径，支持绝对路径或相对项目根目录路径

图标生成目标目录固定为：

```bash
<web_dir>/public/assets/icons
```

脚本会覆盖以下 PNG 文件：

- `apple-touch-icon.png`
- `favicon-16x16.png`
- `favicon-32x32.png`
- `favicon-48x48.png`
- `icon-28x28.png`
- `icon-108x108.png`
- `icon-192x192.png`
- `icon-512x512.png`
- `icon-1024x1024.png`

说明：

- `favicon.svg` 与 `safari-pinned-tab.svg` 不在该脚本覆盖范围内。

## 部署后验证

建议至少执行以下检查：

```bash
curl -I https://your-web-domain.example.com/
```

若本次改动涉及前端登录回调、支付回跳或单页应用路由，还应补充验证对应路径，例如：

```bash
curl -I https://your-web-domain.example.com/some/callback/path
```

说明：

- `remote_deploy_core.sh` 会先清理远端目标目录，再上传新的 `dist.zip` 并解压。
- 因此部署完成后，应立即验证首页与关键路由是否可访问。
