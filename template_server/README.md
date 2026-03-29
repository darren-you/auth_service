# auth_service_template_server

## 遵循标准

- 工作区 Server 标准：[`../../docs/workspace/standards/server/server_golden_path.md`](../../docs/workspace/standards/server/server_golden_path.md)

## 文档入口

- API 契约入口：[`./auth.api`](./auth.api)
- 历史偏离记录（已回收）：[`../docs/features/server/server_deviation_observability_semantics.md`](../docs/features/server/server_deviation_observability_semantics.md)

## 项目定位

`auth_service/template_server` 是统一认证域的服务子工程，基于 `go-zero` 提供多租户认证配置、第三方登录编排、会话签发、刷新、登出和 `me` 查询能力。

## 当前目录重点

- `auth.api`：认证服务 API 协议定义。
- `internal/handler`、`internal/logic`：`go-zero` 生成的路由处理与业务逻辑入口。
- `internal/model`：认证域数据模型。
- `pkg/provider`、`pkg/session`：Provider 适配和 token/session 规则。

## 核心能力

- 微信 Web/App 登录（App 侧通过 `login-url` 预取一次性 `state`）
- Apple 登录
- 手机验证码登录
- Getui 手机号快捷登录
- 游客登录
- 多租户 `tenant / provider / client_type` 配置同步
- `access token / refresh token` 签发与轮换
- 统一 `auth_users / auth_identities / auth_sessions` 存储

## 统一观测语义

- 入口请求统一收口 `X-Request-ID`，并在服务内部使用 `trace_id`
- 请求日志统一输出 `trace_id / http.method / http.route / http.status_code / latency_ms`
- 下游桥接与第三方 Provider HTTP 调用继续透传 `X-Request-ID`

## 核心接口

- `GET /api/v1/health`
- `GET /api/v1/auth/providers/:provider/login-url`
- `POST /api/v1/auth/providers/password/register`
- `POST /api/v1/auth/providers/:provider/callback`
- `POST /api/v1/auth/providers/phone/send-captcha`
- `POST /api/v1/auth/providers/guest/device-id`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `GET /api/v1/auth/me`

## 部署

```bash
PROJECT_ROOT=/absolute/path/to/auth_service

BuildBranch=origin/master BuildEnv=prod \
bash "$PROJECT_ROOT/deploy_shell/deploy_server/remote_deploy_pipeline.sh" \
  --config "$PROJECT_ROOT/template_server/deploy_config.sh"
```
