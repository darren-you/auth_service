# deploy_swift_mac 脚本说明

脚本目录：`deploy_shell/deploy_swift_mac/`

统一配置方式：在 macOS 工程根目录维护 `deploy_config.sh`，并通过 `--config` 显式传入。

## 脚本清单

- `change_bundle_id.sh`：修改 Bundle Identifier
- `change_app_name.sh`：修改桌面显示名称
- `change_launch_screen_assets.sh`：替换启动页资源（可选，仅在工程使用 LaunchScreen 时需要）
- `change_app_icon.sh`：替换 AppIcon
- `change_project_name.sh`：重命名 macOS 原生工程（谨慎）
- `remote_deploy_pipeline.sh`：macOS 完整构建发布流水线（Jenkins 参数化入口）

## 使用方式

在 macOS 工程根目录执行（例如 `mac_app` 目录）：

```bash
bash ../deploy_shell/deploy_swift_mac/change_bundle_id.sh --config "$(pwd)/deploy_config.sh"
bash ../deploy_shell/deploy_swift_mac/change_app_name.sh --config "$(pwd)/deploy_config.sh"
bash ../deploy_shell/deploy_swift_mac/change_launch_screen_assets.sh --config "$(pwd)/deploy_config.sh"
bash ../deploy_shell/deploy_swift_mac/change_app_icon.sh --config "$(pwd)/deploy_config.sh"
```

资源路径规则：
- 图片路径支持绝对路径，或相对项目根目录（`deploy_config.sh` 所在目录）的路径。
- `deploy_shell/deploy_swift_mac` 不再内置资源目录。
- 以下工程路径已改为脚本自动识别，无需在 `deploy_config.sh` 配置：
  - `PBXPROJ_PATH`
  - `ASSETS_XCASSETS_DIR`
  - `LAUNCH_SCREEN_STORYBOARD_PATH`
  - `APP_ICONSET_DIR`

Jenkins 参数化构建入口：

```bash
bash ../deploy_shell/deploy_swift_mac/remote_deploy_pipeline.sh --config "$(pwd)/deploy_config.sh"
```

Jenkins 参数默认值：
- `BuildBranch=master`
- `BuildType=app-store`
- `BuildEnv=test`
- `BuildPodUpdate=false`
- `AppStoreAppleID`：`BuildType=app-store` 时必填
- `AppStoreProviderPublicID`：`BuildType=app-store` 时必填

通知配置（在 `deploy_config.sh` 中）：
- `PROJECT_NAME`：通知标题中的项目名
- `WECHAT_WEBHOOK_URL`：企业微信机器人 webhook
- `JENKINS_BUILD_URL_BASE`：Jenkins 任务基础地址（可选），例如 `https://jenkins.example.com/job/macos-release`
  - 企业微信通知统一使用 markdown 消息，正文结构与 SSL 巡检通知保持一致
  - 当能解析出本次构建地址时，消息末尾会附“查看 Jenkins 构建”链接
- `APP_STORE_APPLE_ID`：App Store Connect 的应用 Apple ID
- `APP_STORE_PROVIDER_PUBLIC_ID`：App Store Connect Provider Public ID

## App Store 参数查询

查询 `APP_STORE_PROVIDER_PUBLIC_ID`：

```bash
xcrun altool --list-providers \
  -u "<App Store Connect 账号邮箱>" \
  -p "<App 专用密码>" \
  --output-format json
```

在返回结果中查看 provider 的 `id`；或使用普通输出查看 `Public ID` 字段。
