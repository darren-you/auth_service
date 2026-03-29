# server_deviation_observability_semantics

## 1. 基本信息

- 仓库：`auth_service`
- 仓库 archetype：`shared_domain_service`
- 技术域：`server`
- 对应标准：`docs/workspace/standards/server/server_contract_and_observability.md`
- 提出人：`darren`
- 负责人：`darren`
- 提出日期：`2026-03-29`
- 当前状态：`retired`

## 2. 偏离点

`auth_service/template_server` 当前已经满足下面几项：

- 健康检查路径为 `GET /api/v1/health`
- 对外契约入口为 `template_server/auth.api`
- 业务 JSON 响应已使用 `code / timestamp / msg / data`

这份记录对应的历史偏离已经回收，当前已显式满足下面几项：

1. 仓库内已显式约定 `X-Request-ID` 作为统一入口 Header。
2. 仓库内已显式约定 `trace_id` 作为统一上下文 key。
3. 当前请求级日志已在仓库内显式收口 `trace_id / http.method / http.route / http.status_code / latency_ms`。
4. 认证桥接调用 `POST /api/v1/internal/auth/sync` 与第三方 Provider HTTP 调用已显式继续透传 `X-Request-ID`。

## 3. 偏离原因

`auth_service` 是较早落地的 `go-zero` 共享域服务，历史上先完成了：

- 契约先行
- 统一 response envelope
- 统一错误处理

本次已补齐自定义 request id middleware、request logger middleware 和 `trace_id` 常量收口层，并把历史偏离回收至工作区统一标准。

## 4. 影响范围

历史上影响过：

- `auth_service/template_server` 的请求链路排障与日志关联
- `auth_service` 到业务服务 `auth sync` 桥接调用的跨服务关联
- 未来工作区对 `shared_domain_service` 的统一观测检查

当前已回收，不再构成活动偏离。

不会影响：

- 健康检查路径
- API 契约入口
- 当前业务响应 envelope

## 5. 风险与代价

- 如果后续改动绕过 `internal/observability` 和 `internal/middleware`，仍可能重新引入链路字段不一致。
- 新增下游 HTTP 调用如果不复用现有 `PropagateRequestID`，仍会再次产生服务间链路断点。

## 6. 回收计划

- 是否计划回收：`completed`
- 完成日期：`2026-03-29`
- 回收结果：`已补统一 request id 中间件、显式 trace_id 上下文收口、最低请求日志字段收口，并在 auth bridge 与第三方 Provider HTTP 调用中透传 X-Request-ID`

## 7. 相关文档

- 标准正文：`docs/workspace/standards/server/server_contract_and_observability.md`
- 机制说明：`docs/workspace/standards/deviation_adr_usage.md`
- 相关实现：`auth_service/template_server/auth.go`
- 相关实现：`auth_service/template_server/internal/config/config.go`
- 相关实现：`auth_service/template_server/internal/errorx/error_handler.go`
- 相关实现：`auth_service/template_server/internal/middleware/requestidmiddleware.go`
- 相关实现：`auth_service/template_server/internal/middleware/requestlogmiddleware.go`
- 相关实现：`auth_service/template_server/internal/observability/requestid.go`
- 相关实现：`auth_service/template_server/internal/logic/authflow.go`
- 相关实现：`auth_service/template_server/pkg/provider/getui/client.go`
- 相关实现：`auth_service/template_server/pkg/provider/wechat/client.go`
- 相关问题归档或 ADR：`docs/workspace/standards/adr/adr_server_contract_and_observability.md`
- 相关问题归档或 ADR：`docs/workspace/standards/adr/adr_shared_domain_service_observability_transition.md`
