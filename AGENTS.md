# AGENTS

## General Rules

- 使用中文回复。
- 使用中文撰写文档。
- 生成 commit 时使用中文。
- 开始处理某个仓库前，优先阅读该仓库根目录 `README.md`；如果存在对应子工程 `README.md`，继续读取子工程说明后再动手。
- 当根 `README.md`、子工程 `README.md`、`deploy_config.sh`、实际目录结构之间出现不一致时，以当前仓库实际文件和配置为准，不要死记旧规则。
- 仅在故障排查、bug 修复、线上问题定位或明确需要复盘历史问题时，优先检索当前仓库的 `docs/issues` 或所属工作区根目录 `harness/docs` 中是否已有类似记录及解决方案。
- 对于明确的代码修改、文档修改、重构、实现新功能、纯说明类问题，不要求默认先检查上述问题归档目录。
- 在 `darren_space` 工作区内，如用户要求“全部提交并推送”“批量 pull/push 整个工作区”这类针对全工作区的 Git 操作，优先直接使用工作区根目录 `darren_space_git.sh`，不要逐仓库手动执行；除非用户明确要求只处理单个仓库，或该脚本不适用。
- 在 `darren_space` 工作区内，如需修改子工程中的 `deploy_shell` submodule，标准流程是先在工作区根目录的 `deploy_shell` 源工程完成修改并 push，再进入对应子工程中的 `deploy_shell` 执行 pull 同步最新提交，并在该子工程提交更新后的 submodule 指针；不要长期直接在子工程内嵌的 `deploy_shell` 目录脱离源工程单独维护。

## Engineering Standards

- 工作区工程标准正文入口位于根目录 `harness/docs/workspace/standards/README.md`。
- 开始处理某个技术域前，除根 `README.md` 与子工程 `README.md` 外，还必须优先读取对应标准入口；具体技术域映射统一以 `harness/docs/workspace/standards/README.md` 为准。
- 如果当前任务明确属于某个技术域，但仓库内没有标准目录名，也应优先参考最接近的工作区 standards 文档，而不是只沿用历史实现。
- 标准冲突时，优先以工作区 `harness/docs/workspace/standards/` 正文和当前仓库真实结构为准。

## Documentation Layout

- 轻量仓库文档默认以根 `README.md` 作为主入口，详细布局规则以 `harness/docs/README.md` 和工作区统一归档约定为准。
- 顶层只保留入口文档，细致说明放入合适的主题子目录。

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

## Server YAML Config

- `template_server` 运行配置只认 YAML，不再依赖 env fallback；具体文件角色和目录约定以 `harness/docs/workspace/standards/server/server_golden_path.md` 为准。
- 需要确认真实后端行为时，优先参考 `harness/agents/skills/workspace-server-validate-via-deploy/SKILL.md`，通过标准部署链路验证，而不是默认本地运行。

## Deployment And Verification

- 本工作区所说的 `CICD`，默认指 production / prod / 线上部署。
- `deploy_shell` 是部署标准的唯一来源；所有部署、验证、SSH 目标都必须优先从真实 `deploy_config.sh` 和当前仓库结构动态读取。
- 执行部署后，必须补做线上验证，至少覆盖本次变更涉及的关键功能或关键路径。
- `deploy_shell` 是部署标准的唯一来源，不要为历史子工程目录、旧配置路径或非标准结构继续在 `deploy_shell` 内追加兼容逻辑；发现业务仓库不符合规范时，应优先修改业务仓库本身对齐当前标准。
- `AGENTS.md` 中禁止写入某个具体项目专属的域名、服务器 IP、账号密码、固定容器名、固定部署目录等硬编码信息。
- 部署流程、发布检查和 SSH 排障优先参考：
  - `harness/agents/skills/workspace-deploy-release-flow/SKILL.md`
  - `harness/agents/skills/workspace-web-release-check/SKILL.md`
  - `harness/agents/skills/workspace-server-release-check/SKILL.md`
  - `harness/agents/skills/workspace-server-ssh-diagnose/SKILL.md`
- 如果 `agents_tools` MCP 已接入，优先使用：
  - `workspace.git.batch`
  - `deploy.web.pipeline`
  - `verify.web.http`
  - `deploy.server.pipeline`
  - `verify.server.health`
  - `remote.server.inspect`

## Server Deployment And Verification

- Server 部署完成后，至少验证健康检查和本次变更涉及的关键接口。
- 对 `template_server` 做健康检查时，优先使用已上线域名对应的接口地址验证，不要把公网 `IP:端口` 直接当成对外健康检查地址。
- 域名暂时不可达时，可用 `127.0.0.1:<host-port>`、容器端口或容器内地址做补充排障，但不能替代域名健康检查结论。
- 详细流程优先参考 `harness/agents/skills/workspace-server-release-check/SKILL.md`。
- 如已接入 `agents_tools` MCP，优先使用 `deploy.server.pipeline`、`verify.server.health` 与 `remote.server.inspect`。

## Server SSH

- 当需要排查线上容器、端口映射、线上实际 YAML 配置、nginx 转发或日志问题时，Agent 可以直接 SSH 登录宿主机处理。
- SSH 目标必须优先从当前项目 `deploy_config.sh` 中读取，若项目存在多个部署目标，先识别当前环境后再连接。
- 排查时不要预设远端部署目录、容器名、Docker 网络名或对外域名。
- 详细排障流程优先参考 `harness/agents/skills/workspace-server-ssh-diagnose/SKILL.md`。
- 如已接入 `agents_tools` MCP，优先使用 `remote.server.inspect`。

## Sync Source

- 本文件由工作区根目录 `harness/scripts/sync_agents.sh` 根据 `harness/agents/manifest.sh` 渲染生成。
- 需要调整通用规则时，优先修改工作区根目录 `harness/agents/fragments`、`harness/agents/manifest.sh` 或对应 `harness/agents/overrides/*.md`，不要长期直接手改各子仓库 `AGENTS.md`。
