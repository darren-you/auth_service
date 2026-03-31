# AGENTS

## Template Server Scope

- 当前目录是 `template_server` 子工程；进入后先读当前目录 `README.md`，再按 `harness/docs/workspace/standards/server/server_golden_path.md` 与 `server_contract_and_observability.md` 执行。
- Server 改动以当前目录源码、`config/`、`deploy_config.sh` 和当前目录 README 说明为真实入口；接口、错误语义与观测字段变更优先同步回写上级 `docs/api/` 与 `docs/features/server/`。
- 运行配置只认 YAML；涉及行为验证、配置生效、健康检查或发布验证时，优先走 `workspace-server-validate-via-deploy`、`workspace-server-release-check`、`deploy.server.pipeline` 与 `verify.server.health`，不要默认以本地运行结果作为标准结论。
