# AGENTS

## Instruction Priority

- 规则冲突时，优先级依次为：用户当前明确要求 > 更近作用域的 `AGENTS.md` / `overrides` / 目录级规则 > 当前仓库真实代码、真实目录结构与真实 `deploy_config.sh` > 工作区 `harness/docs/workspace/standards/` 正文 > `README.md` 与 `harness/docs/workspace/harness/` 专题文档 > 历史归档与参考材料。
- 任何上层文档与真实代码、真实配置冲突时，先以真实现状为准，再决定是否需要回写文档或治理说明。

## General Rules

- 使用中文回复。
- 使用中文撰写文档。
- 生成 commit 时使用中文。
- 开始处理某个仓库前，优先阅读该仓库根目录 `README.md`；如果存在对应子工程 `README.md`，继续读取子工程说明后再动手。
- 当根 `README.md`、子工程 `README.md`、`deploy_config.sh`、实际目录结构之间出现不一致时，以当前仓库实际文件和配置为准，不要死记旧规则。
- 仅在故障排查、bug 修复、线上问题定位或明确需要复盘历史问题时，优先检索当前仓库的 `docs/issues` 或所属工作区根目录 `harness/docs` 中是否已有类似记录及解决方案。
- 对于明确的代码修改、文档修改、重构、实现新功能、纯说明类问题，不要求默认先检查上述问题归档目录。
- 在 `darren_space` 工作区内，如用户要求“全部提交并推送”“批量 pull/push 整个工作区”这类针对全工作区的 Git 操作，优先直接使用工作区根目录 `darren_space_git.sh`，不要逐仓库手动执行；除非用户明确要求只处理单个仓库，或该脚本不适用。
- 在 `darren_space` 工作区内，如任务明确属于子工程 `fast_deploy` submodule 同步，优先走 `workspace-fast-deploy-submodule-sync` Skill，不要长期直接在子工程内嵌的 `fast_deploy` 目录脱离源工程单独维护。

## Engineering Standards

- 当前任务明确属于某个技术域时，除根 `README.md` 与子工程 `README.md` 外，还必须优先读取 `harness/docs/workspace/standards/README.md` 与对应 standards 正文。
- 如果当前任务明确属于某个技术域，但仓库内没有标准目录名，也应优先参考最接近的工作区 standards 文档，而不是只沿用历史实现。
- 标准冲突时，优先以工作区 `harness/docs/workspace/standards/` 正文和当前仓库真实结构为准。

## Documentation Layout

- 轻量仓库文档入口与归档边界统一以根 `README.md`、`harness/docs/README.md` 和工作区统一归档约定为准。

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

- `template_server` 的 YAML 配置契约、文件角色和目录约定以 `harness/docs/workspace/standards/server/server_golden_path.md` 为准。
- 需要确认真实后端行为时，优先走 `workspace-server-validate-via-deploy` Skill 对应的标准部署链路，而不是默认本地运行。

## Deployment And Verification

- 本工作区所说的 `CICD`，默认指 production / prod / 线上部署。
- `fast_deploy` 是部署标准的唯一来源；所有部署、验证、SSH 目标都必须优先从真实 `deploy_config.sh` 和当前仓库结构动态读取。
- `fast_deploy` 是部署标准的唯一来源，不要为历史子工程目录、旧配置路径或非标准结构继续在 `fast_deploy` 内追加兼容逻辑；发现业务仓库不符合规范时，应优先修改业务仓库本身对齐当前标准。
- 执行部署后，必须补做线上验证；详细发布、验证与排障流程优先走对应 Skill 或 MCP。
- `AGENTS.md` 中禁止写入某个具体项目专属的域名、服务器 IP、账号密码、固定容器名、固定部署目录等硬编码信息。
- 部署流程、发布检查和 SSH 排障优先参考：
  - `harness/agents/skills/workspace-deploy-release-flow/SKILL.md`
  - `harness/agents/skills/workspace-web-release-check/SKILL.md`
  - `harness/agents/skills/workspace-server-release-check/SKILL.md`
  - `harness/agents/skills/workspace-server-ssh-diagnose/SKILL.md`
- 如果 `tools` MCP 已接入，优先使用：
  - `workspace.git.batch`
  - `deploy.web.pipeline`
  - `verify.web.http`
  - `deploy.server.pipeline`
  - `verify.server.health`
  - `remote.server.inspect`

## Server SSH

- 当需要排查线上容器、端口映射、线上实际 YAML 配置、nginx 转发或日志问题时，先从当前项目 `deploy_config.sh` 识别目标环境，再优先使用 `workspace-server-ssh-diagnose` Skill 或 `remote.server.inspect`；不要预设远端部署目录、容器名、Docker 网络名或对外域名。
