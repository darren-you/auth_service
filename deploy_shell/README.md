# deploy_shell

通用部署脚本仓库（与具体业务工程解耦），按能力分类：

- `deploy_swift_app/`：iOS 原生项目配置与构建脚本
- `deploy_swift_mac/`：macOS 原生项目配置与构建脚本
- `deploy_android_app/`：Android 原生项目构建脚本
- `deploy_server/`：后端 Docker 构建、推送 Nexus 私仓、远程部署、通知脚本
- `deploy_web/`：前端构建、远程部署、通知脚本
- `deploy_nginx/`：Nginx 配置构建、远程部署、通知脚本
- `deploy_ssl/`：SSL renew hook 构建、远程部署、通知脚本
- `deploy_jenkins/`：Jenkins Freestyle 工程的配置化同步脚本，支持 HTTP 触发 token 管理与批量触发

## 引入方式

在壳工程根目录将本仓库作为一个 submodule 引入，例如：`<shell_root>/deploy_shell`。

引入命令（首次）：

```bash
# 在壳工程根目录执行
git submodule add git@github.com:darren-you/deploy_shell.git deploy_shell
git submodule update --init --recursive
```

更新命令（拉取 submodule 最新代码）：

```bash
# 在壳工程根目录执行
git submodule update --remote --merge deploy_shell

# 更新 submodule 指针并提交
git add deploy_shell
git commit -m "chore: update deploy_shell submodule"
```

## 配置约定

共享 Jenkins 打包机 SSH 配置：

- `deploy_shell/shared/load_jenkins_profile.sh`：Jenkins SSH profile 加载入口
- `deploy_shell/shared/jenkins_profiles/mac_mini.sh`：Mac mini Jenkins 机器的共享 SSH profile
- Jenkins 相关脚本统一读取以下共享变量：
  - `JENKINS_SSH_HOST`
  - `JENKINS_SSH_PORT`
  - `JENKINS_SSH_USER`
  - `JENKINS_SSH_PASSWORD`
  - `JENKINS_SSH_KEY_PATH`
  - `JENKINS_SSH_OPTIONS`
  - `JENKINS_SSH_TARGET`
- 默认 profile 为 `mac_mini`，后续新增 Jenkins 机器时，直接在 `shared/jenkins_profiles` 中增加 profile 文件即可。
- 不需要在 `deploy_swift_app`、`deploy_swift_mac`、`deploy_android_app`、`deploy_web`、`deploy_server`、`deploy_nginx`、`deploy_ssl` 入口脚本中重复透传这组配置。
- 这组值应优先在 `deploy_shell` 内统一维护，不要在各业务项目中重复散落定义。

业务配置文件路径：

- `deploy_swift_app` 的 `change_*` 脚本与 `remote_deploy_pipeline.sh` -> `template_swift_app/deploy_config.sh`
- `deploy_swift_mac` 的 `change_*` 脚本与 `remote_deploy_pipeline.sh` -> `template_swift_mac/deploy_config.sh`
- `deploy_android_app` -> `template_android_app/deploy_config.sh`
- `deploy_server` -> `template_server/deploy_config.sh`
- `deploy_web` -> `template_web/deploy_config.sh`
- `deploy_nginx` -> `template_nginx/deploy_config.sh`
- `deploy_ssl` -> `remote/ssl/deploy_config.sh`
- `deploy_jenkins` -> `deploy_shell/deploy_jenkins/jobs_config.json`

## 使用方法

以下命令均在壳工程根目录执行。示例中的 `PROJECT_ROOT` 请替换为当前业务仓库绝对路径。

说明：

- 这些脚本支持在本地直接调用，不依赖 Jenkins UI。
- 当用户明确要求“部署、上线、修复线上配置并验证”时，Agent 可以直接在本地执行这些脚本，再补做线上验证。
- `deploy_server` 若涉及远端 `.env` 变更，不能只执行 `docker restart`；需要重新创建容器，或直接重跑 `remote_deploy_core.sh` / `remote_deploy_pipeline.sh`。

### 本地直接调用

#### AGENTS 规范同步

如当前仓库位于 `darren_space` 工作区内，统一使用工作区根目录同步器：

```bash
WORKSPACE_ROOT=/absolute/path/to/darren_space
bash "$WORKSPACE_ROOT/scripts/sync_agents.sh" --all --write
```
#### Web 完整闭环（构建 + 部署 + 通知）

```bash
./deploy_shell/deploy_web/change_web_icons.sh --config "$PROJECT_ROOT/template_web/deploy_config.sh"
./deploy_shell/deploy_web/remote_deploy_pipeline.sh --config "$PROJECT_ROOT/template_web/deploy_config.sh"
```

#### Server 完整闭环（构建 + 部署 + 通知）

```bash
BuildBranch=origin/master BuildEnv=prod \
./deploy_shell/deploy_server/remote_deploy_pipeline.sh --config "$PROJECT_ROOT/template_server/deploy_config.sh"
# 或
BuildBranch=origin/develop BuildEnv=test \
./deploy_shell/deploy_server/remote_deploy_pipeline.sh --config "$PROJECT_ROOT/template_server/deploy_config.sh"
```

说明：

- `deploy_server` 已切换为 Nexus Docker 私仓链路，不再使用 Gitee 制品仓库、`git clone` 或 `docker load`。
- 默认推送到 `docker-hosted.xdarren.com`，默认从同一地址拉取；若识别到运行在 Jenkins 且未显式覆盖推送地址，则构建阶段会优先走本机 `127.0.0.1:8082` 上传镜像。
- 本地 macOS 构建默认跟随当前 `docker` CLI 指向的 context / endpoint；当前标准用法是 Docker CLI 指向 Colima。
- 当当前 context 指向 Colima 且 daemon 未就绪时，`deploy_server/docker_build_push.sh` 会尝试启动对应的 Colima profile；如需覆盖 profile，可设置 `DOCKER_COLIMA_PROFILE`。
- 当前默认凭证已固定为 `admin/admin123`；如未显式覆盖，`deploy_server` 会直接使用这组值。
- 认证信息仍支持通过 `NEXUS_DOCKER_USERNAME`、`NEXUS_DOCKER_PASSWORD` 或 `NEXUS_DOCKER_PASSWORD_FILE` 覆盖；详细约定见 `deploy_shell/deploy_server/README.md`。
- 当前共享 Jenkins 构建机也统一在主配置中写死同一组 `NEXUS_DOCKER_USERNAME=admin`、`NEXUS_DOCKER_PASSWORD=admin123`。

#### Nginx 完整闭环（构建 + 部署 + 通知）

```bash
./deploy_shell/deploy_nginx/remote_deploy_pipeline.sh --config "$PROJECT_ROOT/template_nginx/deploy_config.sh"
```

#### SSL Hook 完整闭环（构建 + 部署 + 通知）

```bash
./deploy_shell/deploy_ssl/remote_deploy_pipeline.sh --config "$PROJECT_ROOT/remote/ssl/deploy_config.sh"
```

#### Jenkins 工程同步（配置 -> Jenkins）

```bash
./deploy_shell/deploy_jenkins/sync_jobs.sh push
```

#### Jenkins 工程反向拉取（Jenkins -> 配置）

```bash
./deploy_shell/deploy_jenkins/sync_jobs.sh pull
```

#### Jenkins HTTP 触发（共享 Jenkins 域名）

```bash
./deploy_shell/deploy_jenkins/trigger_jobs_http.sh list --type server --jenkins-url https://jenkins.xdarren.com
./deploy_shell/deploy_jenkins/trigger_jobs_http.sh trigger --type server --build-env test --jenkins-url https://jenkins.xdarren.com
```

说明：

- 依赖 Jenkins 已安装 `Build Authorization Token Root Plugin`
- 默认通过共享 Jenkins 域名 `https://jenkins.xdarren.com` 触发 job，不再假设当前机器存在本地 Jenkins
- `trigger` 默认沿用 job 自己配置的默认分支；只有显式传入 `--branch` 时才会覆盖
- 当传 `--wait true` 时，脚本会自动通过共享 Jenkins SSH profile 读取远端 Jenkins Home 的构建状态，不读取当前机器本地 `~/.jenkins`
- 若传入 `--branch origin/master` 或 `--branch refs/heads/master`，脚本会自动归一化成纯分支名
- `jobs_shared.json` 或 Jenkins UI 中显示默认值是 `origin/master`，只表示 job 默认参数如此配置；不代表每次 HTTP 触发都要显式传 `--branch origin/master`
- 如果目标 job 对显式 `BuildBranch=master` 校验失败，正确做法是省略 `--branch`，让 Jenkins 落回 job 默认值，而不是继续尝试改传 `origin/master`
- Server / iOS / macOS / Android / Web / Nginx / SSL 通知统一使用企业微信 markdown 消息
- Markdown 正文结构与 SSL 巡检通知保持一致：标题、概览、详情块、异常信息与可选链接分段展示
- 当能够解析出 Jenkins 构建地址时，消息末尾会附“查看 Jenkins 构建”链接；没有可用地址时则只保留正文内容

#### Swift App 配置脚本（iOS 原生，仅本地调用）

```bash
./deploy_shell/deploy_swift_app/change_bundle_id.sh --config "$PROJECT_ROOT/template_swift_app/deploy_config.sh"
./deploy_shell/deploy_swift_app/change_app_name.sh --config "$PROJECT_ROOT/template_swift_app/deploy_config.sh"
./deploy_shell/deploy_swift_app/change_launch_screen_assets.sh --config "$PROJECT_ROOT/template_swift_app/deploy_config.sh"
./deploy_shell/deploy_swift_app/change_app_icon.sh --config "$PROJECT_ROOT/template_swift_app/deploy_config.sh"
```

说明：

- `deploy_shell` 只认标准目录 `template_swift_app/deploy_config.sh`。
- 历史业务仓库若仍保留 `template_app/`，应先改造仓库目录与文档，再接入当前部署链路。
- Jenkins iOS 构建现在会自动准备 `ci_codesign.keychain-db` 并注入 `OTHER_CODE_SIGN_FLAGS=--keychain ...`，避免 `codesign wants to use the "System" keychain` 弹窗。

#### Swift Mac 配置脚本（macOS 原生，仅本地调用）

```bash
./deploy_shell/deploy_swift_mac/change_bundle_id.sh --config "$PROJECT_ROOT/template_swift_mac/deploy_config.sh"
./deploy_shell/deploy_swift_mac/change_app_name.sh --config "$PROJECT_ROOT/template_swift_mac/deploy_config.sh"
./deploy_shell/deploy_swift_mac/change_launch_screen_assets.sh --config "$PROJECT_ROOT/template_swift_mac/deploy_config.sh"
./deploy_shell/deploy_swift_mac/change_app_icon.sh --config "$PROJECT_ROOT/template_swift_mac/deploy_config.sh"
```

说明：

- `deploy_shell` 只认标准目录 `template_swift_mac/deploy_config.sh`。

#### Android App 构建

```bash
./deploy_shell/deploy_android_app/remote_deploy_pipeline.sh --config "$PROJECT_ROOT/template_android_app/deploy_config.sh"
```

### 部署后建议验证

- Web：
  - `curl -I https://your-web-domain.example.com/`
- Server：
  - `curl -i https://your-api-domain.example.com/api/v1/health`
  - 必要时再验证关键业务接口
- Nginx：
  - `ssh <user>@<host> 'sudo nginx -t'`
  - 验证目标域名是否已返回新配置

### Jenkins 使用示例

说明：
- 以下示例适用于 Jenkins `Free Style` 工程。
- 以下命令请放在 `Build Steps` 的 `Execute shell` 中执行。

#### 通用前置（拉取 submodule 最新代码）

```bash
set -euo pipefail

cd "$WORKSPACE"
git submodule sync --recursive
git submodule update --init --remote --recursive deploy_shell
```

### Jenkins 工程模板同步

说明：
- `deploy_shell/deploy_jenkins/jobs_config.json` 是 Jenkins 工程清单。
- `deploy_shell/deploy_jenkins/jobs_shared.json` 保存共享 shell/参数 profile；日常维护通常只需要改 `jobs_config.json` 里的 `jobs`。
- 后续新增项目时，先保证仓库目录固定为 `template_swift_app`、`template_swift_mac`、`template_android_app`、`template_web`、`template_server`、`template_nginx`、`remote/ssl`，并在对应 `deploy_config.sh` 中设置唯一工程名。
- 需要新增 Jenkins job 时，直接手工编辑 `jobs_config.json`，再执行同步脚本。
- 如执行 `pull`，本地 `jobs_config.json` 会被 Jenkins 远端实际 job 列表覆盖。
- 详细配置字段与同步方式见：
  - `deploy_shell/deploy_jenkins/README.md`

#### Nginx 示例

```bash
set -euo pipefail

cd "$WORKSPACE"
git submodule sync --recursive
git submodule update --init --remote --recursive deploy_shell

bash "$WORKSPACE/deploy_shell/deploy_nginx/remote_deploy_pipeline.sh" --config "$WORKSPACE/template_nginx/deploy_config.sh"
```

#### SSL 示例

```bash
set -euo pipefail

cd "$WORKSPACE"
git submodule sync --recursive
git submodule update --init --remote --recursive deploy_shell

bash "$WORKSPACE/deploy_shell/deploy_ssl/remote_deploy_pipeline.sh" --config "$WORKSPACE/remote/ssl/deploy_config.sh"
```

#### Web 示例

```bash
set -euo pipefail

cd "$WORKSPACE"
git submodule sync --recursive
git submodule update --init --remote --recursive deploy_shell

bash "$WORKSPACE/deploy_shell/deploy_web/remote_deploy_pipeline.sh" --config "$WORKSPACE/template_web/deploy_config.sh"
```

#### Server 示例

```bash
set -euo pipefail

cd "$WORKSPACE"
git submodule sync --recursive
git submodule update --init --remote --recursive deploy_shell

bash "$WORKSPACE/deploy_shell/deploy_server/remote_deploy_pipeline.sh" --config "$WORKSPACE/template_server/deploy_config.sh"
```

Server 构建参数说明：

- Jenkins 参数 `BuildBranch` 会注入环境变量 `BuildBranch`（默认 `master`）
  - 支持 `origin/master`、`refs/heads/master`、`master`，脚本会自动归一化为 `master`
- Jenkins 参数 `BuildEnv` 会注入环境变量 `BuildEnv`（仅支持 `test` / `prod`，默认 `test`）
  - server 链路内部也统一使用 `test` / `prod`
- `deploy_config.sh` 中可配置 `JENKINS_BUILD_URL_BASE`，用于在通知中生成 `🏗️Jenkins` 构建任务链接

#### Swift App 示例

```bash
set -euo pipefail

cd "$WORKSPACE"
git submodule sync --recursive
git submodule update --init --remote --recursive deploy_shell

bash "$WORKSPACE/deploy_shell/deploy_swift_app/remote_deploy_pipeline.sh" --config "$WORKSPACE/template_swift_app/deploy_config.sh"
```

Swift App 构建参数说明：

- `--config` 必传，建议使用：`$WORKSPACE/template_swift_app/deploy_config.sh`
- 业务参数由 Jenkins `This project is parameterized` 注入到环境变量，不需要在脚本中再次赋值。

- `BuildBranch`：构建分支（默认 `master`）
- `BuildType`：打包类型（`ad-hoc` / `app-store`，默认 `app-store`）
- `BuildEnv`：后端环境（`test` / `prod`，默认 `test`）
- `BuildPodUpdate`：是否执行 `pod update`（`true` / `false`，默认 `false`）
- `AppStoreAppleID`：App Store Connect 的应用 Apple ID（`BuildType=app-store` 时必填）
- `AppStoreProviderPublicID`：App Store Connect Provider Public ID（`BuildType=app-store` 时必填）

#### Swift Mac 示例

```bash
set -euo pipefail

cd "$WORKSPACE"
git submodule sync --recursive
git submodule update --init --remote --recursive deploy_shell

bash "$WORKSPACE/deploy_shell/deploy_swift_mac/remote_deploy_pipeline.sh" --config "$WORKSPACE/template_swift_mac/deploy_config.sh"
```

Swift Mac 构建参数说明：

- `--config` 必传，建议使用：`$WORKSPACE/template_swift_mac/deploy_config.sh`
- 业务参数由 Jenkins `This project is parameterized` 注入到环境变量，不需要在脚本中再次赋值。

- `BuildBranch`：构建分支（默认 `master`）
- `BuildType`：打包类型（`ad-hoc` / `app-store`，默认 `app-store`）
- `BuildEnv`：后端环境（`test` / `prod`，默认 `test`）
- `BuildPodUpdate`：是否执行 `pod update`（`true` / `false`，默认 `false`）
- `AppStoreAppleID`：App Store Connect 的应用 Apple ID（`BuildType=app-store` 时必填）
- `AppStoreProviderPublicID`：App Store Connect Provider Public ID（`BuildType=app-store` 时必填）

#### Android App 示例

```bash
set -euo pipefail

cd "$WORKSPACE"
git submodule sync --recursive
git submodule update --init --remote --recursive deploy_shell

bash "$WORKSPACE/deploy_shell/deploy_android_app/remote_deploy_pipeline.sh" --config "$WORKSPACE/template_android_app/deploy_config.sh"
```
