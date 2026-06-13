# deploy_android_app 脚本说明

脚本目录：`deploy_shell/deploy_android_app/`

用途：

- 统一构建原生 Android 模板工程
- 自动维护 `version.properties` 中的 `VERSION_CODE`
- 支持按渠道输出 `apk` 或 `aab`
- 支持构建结果企业微信通知

## 配置约定

统一读取 `template_android_app/deploy_config.sh`，关键字段：

- `PROJECT_NAME_ANDROID_APP`：Jenkins 工程名，必须以 `_android_app` 结尾。
- `PROJECT_ROOT`：可选，项目根目录；默认取 `deploy_config.sh` 所在目录。
- `APP_NAME`：可选，产物目录前缀；默认取 `PROJECT_NAME_ANDROID_APP`。
- `ANDROID_APP_MODULE_NAME`：可选，默认 `app`。
- `ANDROID_GRADLEW_PATH`：可选，默认 `./gradlew`。
- `ANDROID_VERSION_FILE`：可选，默认 `app/version.properties`。
- `ANDROID_BUILD_CHANNELS`：可选，逗号分隔的渠道列表，默认 `official`。
- `ANDROID_DEFAULT_CHANNEL`：可选，默认 `official`。
- `ANDROID_DEFAULT_ARTIFACT`：可选，默认 `apk`，可选 `apk` / `aab`。
- `WECHAT_WEBHOOK_URL`：可选，构建通知地址。
- `JENKINS_BUILD_URL_BASE`：可选，通知中 Jenkins 链接前缀。
  - 企业微信通知统一使用 markdown 消息，正文结构与 SSL 巡检通知保持一致。
  - 当能解析出本次构建地址时，消息末尾会附“查看 Jenkins 构建”链接。

## Jenkins / 环境变量参数

- `BuildBranch`：构建分支，默认 `origin/master`
- `BuildEnv`：业务环境，默认 `test`
- `BuildChannel`：构建渠道，默认读取 `ANDROID_DEFAULT_CHANNEL`，传 `all` 时全渠道输出
- `BuildArtifact`：构建产物，默认读取 `ANDROID_DEFAULT_ARTIFACT`

## 使用示例

```bash
PROJECT_ROOT=/absolute/path/to/project

bash "$PROJECT_ROOT/deploy_shell/deploy_android_app/remote_deploy_pipeline.sh" \
  --config "$PROJECT_ROOT/template_android_app/deploy_config.sh"
```

指定环境与产物：

```bash
BuildBranch=origin/master BuildEnv=prod BuildChannel=official BuildArtifact=aab \
bash "$PROJECT_ROOT/deploy_shell/deploy_android_app/remote_deploy_pipeline.sh" \
  --config "$PROJECT_ROOT/template_android_app/deploy_config.sh"
```
