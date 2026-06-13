# deploy_jenkins 说明

脚本目录：`deploy_shell/deploy_jenkins/`

用途：

- 统一维护 Jenkins 上 `swift_app`、`android_app`、`web`、`server`、`nginx`、`ssl` 六类 Freestyle 工程
- `jobs_config.json` 由人工维护，`jobs_shared.json` 收口共享模板
- Jenkins Shell 模板固定走 `template_swift_app`、`template_android_app`、`template_web`、`template_server`、`template_nginx`、`remote/ssl`
- 托管 job 默认启用 HTTP 远程触发 token，可配合 `Build Authorization Token Root Plugin` 直接通过 URL 触发构建

## 目录约定

所有使用 `deploy_shell` 的业务仓库都要遵循下面的目录规则：

- Swift App：`template_swift_app/deploy_config.sh`
- Android App：`template_android_app/deploy_config.sh`
- Web：`template_web/deploy_config.sh`
- Server：`template_server/deploy_config.sh`
- Nginx：`template_nginx/deploy_config.sh`
- SSL：`remote/ssl/deploy_config.sh`

说明：

- `deploy_shell` 只接受标准目录；历史仓库如仍使用 `template_app/deploy_config.sh`，必须先迁移到 `template_swift_app/deploy_config.sh`
- `jobs_config.json` 中的 `name`、`view`、`description` 由人工决定，不再根据工作区自动生成

## 配置结构

日常维护只需要打开 `jobs_config.json`；共享模板统一放在同目录 `jobs_shared.json`。

`jobs_config.json` 固定使用 `version = 2`：

```json
{
  "version": 2,
  "shared_file": "jobs_shared.json",
  "jobs": [],
  "commented_jobs": []
}
```

其中：

- `jobs_shared.json`
  - 保存所有共享 `shell_command_profiles` 与 `parameter_profiles`
  - `pull` 时会按当前模板自动更新这个文件
- `jobs_shared.json.shell_command_profiles`
  - `swift_app` -> `$WORKSPACE/template_swift_app/deploy_config.sh`
  - `android_app` -> `$WORKSPACE/template_android_app/deploy_config.sh`
  - `web` -> `$WORKSPACE/template_web/deploy_config.sh`
  - `server` -> `$WORKSPACE/template_server/deploy_config.sh`
  - `nginx` -> `$WORKSPACE/template_nginx/deploy_config.sh`
  - `ssl` -> `$WORKSPACE/remote/ssl/deploy_config.sh`
- `jobs_shared.json.parameter_profiles`
  - `branch_only`
  - `server`
  - `swift_app`
  - `android_app`
- `jobs`
  - 只维护 job 本身信息
  - 常规情况下只需要 `name`、`type`、`repo_url`、`view` 以及 profile 字段
  - `view` 是 Jenkins 视图名，例如 `📱App`、`💻Server`、`🧭Web`
  - 若某个 job 未配置 `view`，`push` 时不会把它加入任何显式 List View
- `commented_jobs`
  - 暂时不下发到 Jenkins 的 job 清单
  - `pull` 时会继续保留这些条目，避免被重新写回 `jobs`

兼容性说明：

- 旧版把 `shared` 内联写在 `jobs_config.json` 的格式仍然可读
- 新版 `pull` 会自动拆成 `jobs_config.json + jobs_shared.json`

实际模板内容以 [jobs_config.json](/Users/darrenyou/darren_space/deploy_shell/deploy_jenkins/jobs_config.json) 和 [jobs_shared.json](/Users/darrenyou/darren_space/deploy_shell/deploy_jenkins/jobs_shared.json) 为准。

## 使用方式

前置依赖：

- Jenkins 已安装 `Build Authorization Token Root Plugin`
- 推荐使用插件提供的 `/buildByToken/buildWithParameters` 接口触发参数化 job
- server 构建若要推送 Nexus，需让 Jenkins 运行环境可读取：
  - `NEXUS_DOCKER_USERNAME`
  - `NEXUS_DOCKER_PASSWORD` 或 `NEXUS_DOCKER_PASSWORD_FILE`
- 当前推荐做法是在 Jenkins 主配置 `globalNodeProperties` 中设置：
  - `NEXUS_DOCKER_USERNAME=admin`
  - `NEXUS_DOCKER_PASSWORD=admin123`
- `deploy_shell` 当前默认也内置同一组 Nexus 凭证：`admin/admin123`
- 后续不要再把“重置 Nexus 密码”作为 Jenkins/Nexus 构建失败时的常规修复手段；优先复用当前固定默认值排查

### 1. 手工维护本地配置

```bash
vim deploy_shell/deploy_jenkins/jobs_config.json
```

行为：

- `jobs` 只能手工添加、修改、删除
- 模板项目如 `template_*` 默认不要加入 `jobs`
- 共享模板通常不需要手改；如需调整，改 `jobs_shared.json`

### 2. 从 Jenkins 反向拉取配置

```bash
bash deploy_shell/deploy_jenkins/sync_jobs.sh pull
```

行为：

- 从远端 Jenkins 读取 `config.xml`
- 识别标准 deploy_shell Freestyle job
- 同时读取 Jenkins 根 `config.xml` 中的视图配置
- 回写为 `version = 2` 配置结构，并同步拆分成 `jobs_config.json + jobs_shared.json`
- 以远端 Jenkins 实际 job 列表覆盖本地 `jobs_config.json`

说明：

- `pull` 主要用于反查远端现状
- 日常可先手工维护；如执行 `pull`，则以 Jenkins 远端结果为准覆盖本地
- `pull` 会保留 Jenkins 原始 job 名，不再按仓库名二次规范化

### 3. 推送到 Jenkins

```bash
bash deploy_shell/deploy_jenkins/sync_jobs.sh push
```

默认行为：

- 渲染 Jenkins `config.xml`
- 通过 SSH 写入远端 `~/.jenkins/jobs/<job_name>/config.xml`
- 写入 `.deploy_shell_managed_job.json`
- 删除远端未出现在本地配置中的旧标准 job
- 按 `jobs[].view` 回写 Jenkins 根视图配置
- 自动重载 Jenkins

### 4. dry-run

```bash
bash deploy_shell/deploy_jenkins/sync_jobs.sh push --dry-run
```

说明：

- `push` 默认会删除两类旧 job：
  - 带 `.deploy_shell_managed_job.json` 标记的托管 job
  - 能识别为标准 `deploy_shell` 构建链路、但已不在本地配置中的历史 job
- 不会删除与 `deploy_shell` 无关的其他 Jenkins 工程

### 5. 查看 HTTP 触发清单

```bash
bash deploy_shell/deploy_jenkins/trigger_jobs_http.sh list
bash deploy_shell/deploy_jenkins/trigger_jobs_http.sh list --type server
```

输出字段依次为：

- `job.name`
- `job.type`
- 默认 `BuildBranch`（仅展示 job 默认值）
- 远程触发 token
- 可直接访问的 HTTP 触发地址

### 6. 通过 HTTP 触发构建

触发单个 server job：

```bash
bash deploy_shell/deploy_jenkins/trigger_jobs_http.sh trigger \
  --type server \
  --job stellar_server \
  --build-env test \
  --jenkins-url https://jenkins.xdarren.com
```

触发全部 server job：

```bash
bash deploy_shell/deploy_jenkins/trigger_jobs_http.sh trigger \
  --type server \
  --build-env test \
  --jenkins-url https://jenkins.xdarren.com
```

如需手工覆盖分支：

```bash
bash deploy_shell/deploy_jenkins/trigger_jobs_http.sh trigger \
  --type server \
  --job stellar_server \
  --branch origin/master \
  --build-env test \
  --jenkins-url https://jenkins.xdarren.com
```

说明：

- 脚本默认访问共享 Jenkins 域名：`https://jenkins.xdarren.com`
- 默认不显式传 `BuildBranch`，而是沿用 Jenkins job 自己配置的默认分支
- 若传入 `--branch`，脚本会把 `origin/master`、`refs/heads/master` 这类值归一化成纯分支名后再请求 Jenkins
- 因此 `jobs_shared.json`、`jobs_config.json` 或 Jenkins UI 中看到默认值是 `origin/master`，只说明 job 默认参数如此配置；不代表 HTTP 触发时要显式传 `--branch origin/master`
- 若某个 job 已确认对显式 `BuildBranch=master` 校验失败，正确做法是省略 `--branch`，让 Jenkins 落回 job 默认值；把参数改写成 `origin/master` 也无效，因为脚本仍会先归一化成 `master`
- 若传 `--wait true`，脚本会自动加载共享 Jenkins SSH profile，读取远端 Jenkins Home 中的 `build.xml` 与 `log` 判断结果；不再依赖当前机器本地 `~/.jenkins`
- 如需额外参数，可重复传入 `--param KEY=VALUE`

## 维护规则

后续新增项目时，流程固定为：

1. 手工编辑 `jobs_config.json`，新增目标 job
2. 根据需要填写 `type`、`repo_url`、`config_path`、`view`、profile 字段
3. 执行 `push`

## 注意事项

- `push` 会先备份原始 `config.xml` 到 `config.xml.bak.deploy_shell`
- Jenkins 根 `config.xml` 也会在同步视图前备份到 `config.xml.bak.deploy_shell`
- Jenkins UI 需要重载后才会显示磁盘配置变更
- 模板仓库 `template_project` 默认作为脚手架母版，不建议直接加入 Jenkins 打包队列
- 托管 job 若未显式关闭 `remote_trigger_enabled`，会自动写入稳定生成的 `authToken`
- 如需为单个 job 自定义 token，可在 `jobs_config.json` 中设置 `remote_trigger_token`
