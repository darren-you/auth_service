# agent_context

## 1. 主入口

- `template_server/`
- `client/`
- `provider/`
- `template_server/auth.api`

## 2. 危险操作

- 业务仓统一接入远程 `auth_service`，不要回退到各仓自建本地登录内核。
- 涉及 `refresh`、session、provider callback 的修改会同时影响多个下游业务仓。
- 新租户接入应优先走 YAML 配置与 bridge，不要复制已有 provider 逻辑。

## 3. 标准验证路径

- Server 标准：[`server_golden_path.md`](../../harness/docs/workspace/standards/server/server_golden_path.md)
- 契约与观测：[`server_contract_and_observability.md`](../../harness/docs/workspace/standards/server/server_contract_and_observability.md)
- 发布后至少验证 `auth.api` 对应的登录、刷新、登出与 `me` 主路径。

## 4. 最近稳定结论

- `auth_service` 是当前工作区统一认证事实源。
- 下游业务仓只应保留用户模型同步和业务桥接，不应复制认证状态机。
