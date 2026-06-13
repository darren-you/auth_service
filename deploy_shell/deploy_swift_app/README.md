# deploy_swift_app 脚本说明

脚本目录：`deploy_shell/deploy_swift_app/`

统一配置方式：在移动端工程根目录维护 `deploy_config.sh`，并通过 `--config` 显式传入。

## 脚本清单

- `change_bundle_id.sh`：修改 Bundle Identifier
- `change_app_name.sh`：修改桌面显示名称
- `change_launch_screen_assets.sh`：替换启动页资源
- `change_app_icon.sh`：替换 AppIcon
- `change_project_name.sh`：重命名 iOS 原生工程（谨慎）
- `remote_deploy_pipeline.sh`：iOS 完整构建发布流水线（Jenkins 参数化入口）

## 使用方式

在移动端工程根目录执行（例如 `ios_app` 目录）：

```bash
bash ../deploy_shell/deploy_swift_app/change_bundle_id.sh --config "$(pwd)/deploy_config.sh"
bash ../deploy_shell/deploy_swift_app/change_app_name.sh --config "$(pwd)/deploy_config.sh"
bash ../deploy_shell/deploy_swift_app/change_launch_screen_assets.sh --config "$(pwd)/deploy_config.sh"
bash ../deploy_shell/deploy_swift_app/change_app_icon.sh --config "$(pwd)/deploy_config.sh"
```

资源路径规则：
- 图片路径支持绝对路径，或相对项目根目录（`deploy_config.sh` 所在目录）的路径。
- `deploy_shell/deploy_swift_app` 不再内置资源目录。
- 以下工程路径已改为脚本自动识别，无需在 `deploy_config.sh` 配置：
  - `PBXPROJ_PATH`
  - `ASSETS_XCASSETS_DIR`
  - `LAUNCH_SCREEN_STORYBOARD_PATH`
  - `APP_ICONSET_DIR`

Jenkins 参数化构建入口：

```bash
bash ../deploy_shell/deploy_swift_app/remote_deploy_pipeline.sh --config "$(pwd)/deploy_config.sh"
```

Jenkins 参数默认值：
- `BuildBranch=master`
- `BuildType=app-store`
- `BuildEnv=test`
- `BuildPodUpdate=false`
- `AppStoreAppleID`：`BuildType=app-store` 时必填
- `AppStoreProviderPublicID`：`BuildType=app-store` 时必填

Jenkins 签名收口：

- `remote_deploy_pipeline.sh` 现在会在 Jenkins / `BUILD_NUMBER` 环境下自动准备 `~/Library/Keychains/ci_codesign.keychain-db`，并把 `OTHER_CODE_SIGN_FLAGS=--keychain <ci_keychain>` 注入到 `archive/exportArchive`。
- 默认会优先从以下位置寻找开发证书与发布证书 `.p12`：
  - `~/Projects/tool/apple/DarrenYou-iOS-Dev-2025.p12`
  - `~/Projects/tool/apple/DarrenYou-iOS-Dis-2025.p12`
  - `~/darren_projects/agents_tools/local/apple/darrenyou_ios_development_2025.p12`
  - `~/darren_projects/agents_tools/local/apple/darrenyou_ios_distribution_2025.p12`
- 默认 `p12` 导入密码与 CI keychain 密码使用当前 Jenkins 统一值；如需覆盖，可在 `deploy_config.sh` 或 Jenkins 环境变量中设置：
  - `CiCodesignPrepare` / `CI_CODESIGN_PREPARE`
  - `CiCodesignKeychainPath` / `CI_CODESIGN_KEYCHAIN_PATH`
  - `CiCodesignKeychainPassword` / `CI_CODESIGN_KEYCHAIN_PASSWORD`
  - `CiCodesignP12Password` / `CI_CODESIGN_P12_PASSWORD`
  - `CiCodesignDevelopmentIdentity` / `CI_CODESIGN_DEVELOPMENT_IDENTITY`
  - `CiCodesignDistributionIdentity` / `CI_CODESIGN_DISTRIBUTION_IDENTITY`
  - `CiCodesignDevelopmentP12Path` / `CI_CODESIGN_DEVELOPMENT_P12_PATH`
  - `CiCodesignDistributionP12Path` / `CI_CODESIGN_DISTRIBUTION_P12_PATH`
- 不要再让 Jenkins 直接依赖 `System.keychain` 里的私钥；那条链路在无头构建下会重新触发管理员授权弹窗。

通知配置（在 `deploy_config.sh` 中）：
- `PROJECT_NAME`：通知标题中的项目名
- `WECHAT_WEBHOOK_URL`：企业微信机器人 webhook
- `JENKINS_BUILD_URL_BASE`：Jenkins 任务基础地址（可选），例如 `https://jenkins.example.com/job/ios-release`
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
