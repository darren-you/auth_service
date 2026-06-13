# AGENTS

## General Rules

- 使用中文回复。
- 使用中文撰写文档。
- 生成 commit 时使用中文。
- 开始处理某个仓库前，优先阅读该仓库根目录 `README.md`；如果存在对应子工程 `README.md`，继续读取子工程说明后再动手。
- 当根 `README.md`、子工程 `README.md`、`deploy_config.sh`、实际目录结构之间出现不一致时，以当前仓库实际文件和配置为准，不要死记旧规则。
- 仅在故障排查、bug 修复、线上问题定位或明确需要复盘历史问题时，优先检索当前仓库的 `docs/issues` 或所属工作区根目录 `docs` 中是否已有类似记录及解决方案。
- 对于明确的代码修改、文档修改、重构、实现新功能、纯说明类问题，不要求默认先检查上述问题归档目录。
- 对 App、Web 与其他前端工程，dev/prod 运行时统一直连对应已部署 server 的 API 域名；不要把本地启动 `template_server`、本地 `go run`、本地容器或本地 API proxy 当成前端联调默认方案。
- 前端如需切换环境，应切换到对应测试或正式环境的已部署 API 域名，并同步补齐 CORS、网关与鉴权配置；不要把 `localhost`、`127.0.0.1`、容器内网地址或宿主机端口写成前端默认 API 地址。
- 对 Web 工程，只要 `vite.config.ts` 要求 `VITE_WEB_NAME`，统一将其视为同级 `deploy_config.sh` 中的 `PROJECT_NAME`；执行 `lint`、`build`、`preview` 等命令前先补齐该变量，不要再把缺少 `VITE_WEB_NAME` 当成额外问题单独汇报。
- 在 `darren_space` 工作区内，如用户要求“全部提交并推送”“批量 pull/push 整个工作区”这类针对全工作区的 Git 操作，优先直接使用工作区根目录 `darren_space_git.sh`，不要逐仓库手动执行；除非用户明确要求只处理单个仓库，或该脚本不适用。
- 在 `darren_space` 工作区内，如需修改子工程中的 `deploy_shell` submodule，标准流程是先在工作区根目录的 `deploy_shell` 源工程完成修改并 push，再进入对应子工程中的 `deploy_shell` 执行 pull 同步最新提交，并在该子工程提交更新后的 submodule 指针；不要长期直接在子工程内嵌的 `deploy_shell` 目录脱离源工程单独维护。

## Documentation Layout

- 对于单用途仓库、工具仓库、配置仓库或轻量前端仓库，以根 `README.md` 作为主入口，按需补充 `docs` 或子目录文档即可。
- 不强制要求补齐完整的 `docs/api`、`docs/features/*`、`docs/issues/*` 目录树。
- 细致的专题说明、操作手册、排障步骤、配置指南不要直接放在仓库根目录或 `docs` 根层级；顶层只保留 `README.md`、总览、索引类入口文档，具体内容应放入合适的主题子目录或对应模块目录。
- 如果当前仓库位于 `darren_space` 工作区内，且该项目已采用工作区集中归档规则，相关归档文档优先查看工作区根目录 `docs/projects/<repo>/` 中对应记录；如该仓库在归档内继续按 `app`、`server`、`web` 细分，再进入对应子目录。

## Documentation Naming

- 对新增的项目文档文件（如 `.md`、`.markdown`、`.txt`），优先使用英文小写单词加下划线命名。
- `README.md` 作为特殊文档保留默认命名，不纳入普通文档命名规则。
- 历史文档、外部导入资料、第三方文档、截图说明、工具资料可以保留原始命名；不要为了满足命名规则批量重命名现有文件。
- 如果确实需要重命名已有文档，必须同时检查并更新仓库内的引用链接。
- 文档命名示例：`project_analysis_report.md`、`font_subset_extraction_guide.md`。

## Commit Message Record

- 当前工作区内每个子工程根目录统一维护 `commit_message.txt`，用于记录该子工程当前这次修改对应的提交信息。
- 每次在某个子工程完成修改后，都要覆盖写入该子工程根目录的 `commit_message.txt`，不要在文件中累积历史提交信息。
- `commit_message.txt` 的内容必须可直接用于提交：使用中文；第一行写 commit 标题；如有补充说明，第二行留空后继续写正文。
- 当用户要求提交某个子工程时，优先直接使用该子工程根目录 `commit_message.txt` 中已记录的内容作为 commit 信息；除非用户明确指定新的提交文案，否则不要临时改写成其他内容。
- 如果一次任务同时修改多个子工程，需要分别覆盖各自的 `commit_message.txt`。

## Deployment And Verification

- 本工作区所说的 `CICD`，默认指正式线上版本的构建与部署；若用户只说“CICD”“发布”“部署”而未明确环境，按 production / prod / 线上环境处理。统一流程是先将本次涉及的代码全部提交并推送，再通过 HTTP 方式触发 Jenkins 中对应的生产构建任务。
- 当用户明确要求“部署、上线、修复线上配置并验证”时，Agent 可以直接在本地调用 `deploy_shell` 中的脚本；如果走 Jenkins 流程，也应先完成提交推送，再通过 HTTP 触发。
- 优先执行完整流水线；只有在明确知道只是线上 YAML 配置核对、容器现场问题或 nginx 转发问题时，才直接 SSH 登录服务器处理。
- 对 Web 工程执行线上 `CICD`、发布正式版本或构建 production 产物前，必须先关闭辅助定位色块、布局标记、调试角标等仅用于本地对齐的视觉辅助元素，并确认 production 构建结果中不再渲染这些元素；这类调试开关只能在本地 dev 调试下启用，不能依赖人工记忆，必要时应在代码或构建脚本中默认对 production 强制关闭。
- 执行部署后，必须补做线上验证，至少覆盖首页、健康检查和本次变更涉及的关键接口或页面。
- 对 `template_server` 做健康检查时，优先使用已上线域名对应的接口地址验证，例如 `https://<api-domain>/api/v1/health`；不要把公网 `IP:端口` 直接当成对外健康检查地址。
- 如果域名暂时不可达或需要排查宿主机内部连通性，可在 SSH 登录服务器后使用 `127.0.0.1:<host-port>`、容器端口或容器内地址做补充验证，但这类方式只作为排障手段，不能替代域名健康检查结论。
- `deploy_shell` 是部署标准的唯一来源，不要为历史子工程目录、旧配置路径或非标准结构继续在 `deploy_shell` 内追加兼容逻辑；发现业务仓库不符合规范时，应优先修改业务仓库本身对齐当前标准。
- 移动端工程目录统一使用 `template_swift_app` 与 `template_android_app`；不要继续沿用 `template_app` 之类的历史命名。
- `AGENTS.md` 中禁止写入某个具体项目专属的域名、服务器 IP、账号密码、固定容器名、固定部署目录等硬编码信息。
- 对 `deploy_shell` 的 Nexus Docker 凭证，优先复用 `deploy_shell/deploy_server/common.sh` 中已固化的默认配置；排查 Jenkins / Nexus 认证问题时，不要再把“重置密码”当成常规处理手段。
- 受上一条限制，`AGENTS.md` 只说明“默认凭证以脚本和 README 为准”，不在 AGENTS 中重复展开凭证明文字面量。
- 所有部署命令、验证地址、SSH 目标都必须优先从当前项目实际存在的 `deploy_config.sh`、项目目录结构和线上返回结果中动态读取，不要把某个项目的现场信息写成通用规则。
- `deploy_shell/shared/jenkins_profiles/mac_mini.sh` 中维护的是 Jenkins 打包机 Mac mini 的共享 SSH profile；公共加载入口位于 `deploy_shell/shared/load_jenkins_profile.sh`。
- 调用 Jenkins HTTP 触发 CICD 时，默认走共享域名 `https://jenkins.xdarren.com`；不要再把当前机器本机 `127.0.0.1:8080` 或本地 `~/.jenkins` 当成前提。
- 通过 HTTP 触发 Jenkins 后，默认不需要等待构建任务执行完成；只有在排查问题、分析构建结果、查看构建日志或同步 Jenkins 配置时，才通过共享 Jenkins SSH profile 访问远端 Jenkins Home，而不是读取当前机器本地 Jenkins 目录。

通用调用方式：

```bash
PROJECT_ROOT=/absolute/path/to/project

# Web
bash "$PROJECT_ROOT/deploy_shell/deploy_web/remote_deploy_pipeline.sh" \
  --config "$PROJECT_ROOT/<web_dir>/deploy_config.sh"

# Server
BuildBranch=origin/master BuildEnv=prod \
bash "$PROJECT_ROOT/deploy_shell/deploy_server/remote_deploy_pipeline.sh" \
  --config "$PROJECT_ROOT/<server_dir>/deploy_config.sh"
```

可选分步调用：

```bash
bash "$PROJECT_ROOT/deploy_shell/deploy_web/npm_build_package.sh" \
  --config "$PROJECT_ROOT/<web_dir>/deploy_config.sh"

BuildBranch=origin/master BuildEnv=prod \
bash "$PROJECT_ROOT/deploy_shell/deploy_server/docker_build_push.sh" \
  --config "$PROJECT_ROOT/<server_dir>/deploy_config.sh"
```

通用验证方式：

```bash
curl -I https://<web-domain>/
curl -i https://<api-domain>/api/v1/health
```

## Server SSH

- 当需要排查线上容器、端口映射、线上实际 YAML 配置、nginx 转发或日志问题时，Agent 可以直接 SSH 登录宿主机处理。
- SSH 目标必须优先从当前项目 `deploy_config.sh` 中读取，例如：
  - `DEPLOY_HOST`
  - `DEPLOY_PORT`
  - `DEPLOY_USER`
  - `DEPLOY_SSH_PASSWORD`
  - `DEPLOY_SSH_KEY_PATH`
- 若项目存在多个部署目标，也必须先识别当前用户要求操作的是哪一个环境，再连接对应机器。

登录示例：

```bash
sshpass -p '<password>' ssh -o StrictHostKeyChecking=no -p <port> <user>@<host>
```

排查时优先先读当前项目配置，再决定是否连接服务器，不要预设以下信息：

- 远端部署目录
- 容器名
- Docker 网络名
- 对外域名

重要注意事项：

- `template_server` 业务配置只认 `config/config.yaml`；环境差异通过 `config/config.dev.yaml` 与 `config/config.prod.yaml` 在构建阶段收口。
- 修改 YAML 后必须重新执行后端部署脚本，不能依赖 `docker restart`、远端 `.env` 或临时环境变量覆盖来生效。
- 不确定线上当前值时，直接 SSH 登录部署机，优先查看当前容器内生效的 `config/config.yaml` 与镜像标签。

## Sync Source

- 本文件由工作区根目录 `scripts/sync_agents.sh` 根据 `agents/manifest.sh` 渲染生成。
- 需要调整通用规则时，优先修改工作区根目录 `agents/fragments`、`agents/manifest.sh` 或对应 `agents/overrides/*.md`，不要长期直接手改各子仓库 `AGENTS.md`。

# AGENTS

## Jenkins Trigger Branch

- 分析 `deploy_shell/deploy_jenkins/trigger_jobs_http.sh` 触发规则时，必须先区分“Jenkins job 配置里的默认 `BuildBranch` 值”和“本次 HTTP 请求是否显式传入 `BuildBranch` 参数”；看到默认值是 `origin/master` 不代表本次请求也该传它。
- `jobs_shared.json`、`jobs_config.json`、`sync_jobs.py` 中的 `origin/master` 是 job 默认参数模板；只有在调用 `trigger_jobs_http.sh` 时显式传入 `--branch`，脚本才会把它带到 HTTP 请求里。
- `trigger_jobs_http.sh` 会把显式传入的 `origin/master`、`refs/heads/master` 归一化成 `master` 后再请求 Jenkins；如果某个 job 对显式 `BuildBranch=master` 校验失败，不要继续尝试把参数改写成 `origin/master`。
- 对依赖 Jenkins job 自身默认分支的任务，正确调用方式是省略 `--branch`，让 Jenkins 使用 job 默认值。
- 调用包装脚本时，必须先确认它是否会偷偷补上 `--branch`；若包装脚本会自动透传当前 Git 分支，而目标 job 不接受显式 `BuildBranch`，应改用 `trigger_jobs_http.sh` 直接触发。
