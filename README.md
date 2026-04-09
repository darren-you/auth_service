# auth_service

`auth_service` 是统一认证域仓库，面向业务工程提供独立部署的认证服务，并沉淀认证服务自身使用的会话、Provider 与接入 SDK。

## 当前结构

- `template_server/`
  - 当前认证服务主体，基于 `go-zero` 的 REST 服务实现。
- `client/`
  - 业务 server 调用 `auth_service` 的 Go client。
- `guest/`、`phone/`、`provider/`、`session/`
  - 服务内部复用的认证域能力包，不应成为业务仓继续自建本地登录内核的理由。

## 独立认证服务

`template_server` 是当前统一认证域实现，按以下层次拆分：

- `auth kernel`
- `provider adapters`
- `tenant/project config`
- `identity/session store`
- `business bridge`

核心数据表：

- `auth_tenants`
- `auth_provider_configs`
- `auth_users`
- `auth_identities`
- `auth_sessions`

核心接口：

- `GET /api/v1/auth/providers/:provider/login-url`
- `POST /api/v1/auth/providers/password/register`
- `POST /api/v1/auth/providers/:provider/callback`
- `POST /api/v1/auth/providers/phone/send-captcha`
- `POST /api/v1/auth/providers/guest/device-id`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `GET /api/v1/auth/me`
- `PUT /api/v1/auth/me`
- `PUT /api/v1/auth/internal/users`

微信小程序接入要点：

- 前端通过 `wx.login` 获取 `code`
- 业务后端把 `tenant_key + client_type=miniprogram + code` 转发到 `POST /api/v1/auth/providers/wechat_miniprogram/callback`
- `auth_service` 使用租户级 `app_id / app_secret` 执行 `code2session`
- 业务 bridge 需要接收 `provider=wechat`，并按 `open_id / union_id / session_key` 同步本地用户

微信 provider 拆分约定：

- `wechat_app` 对应 App 登录
- `wechat_web` 对应 Web 扫码登录
- `wechat_miniprogram` 对应微信小程序登录

## 接入原则

1. 业务工程统一接入远程 `auth_service` 协议，不再各自维护本地登录内核。
2. 新租户通过 YAML 配置和业务 bridge 接口接入，不重复开发微信、Apple、手机号快捷登录、验证码和游客登录流程。
3. 业务侧只保留与自身用户模型有关的本地同步逻辑，认证状态机、Provider 回调和 token 规则统一由 `auth_service` 承担。
4. 当业务侧需要把新的登录方式绑定到当前账号时，应通过 `current_user_id / current_user_role` 把当前业务用户上下文透传给 bridge，而不是在业务仓各自实现另一套验证码认证内核。
5. 业务侧允许用户修改认证域共享资料字段（如昵称、头像）时，必须同步回写 `auth_service`，不能只改本地业务库，否则后续登录和 token 刷新会把旧资料重新带回业务侧。
6. 业务侧管理后台如果要修改其他用户的共享资料字段（如昵称、头像、角色、状态），也必须走 `auth_service` 的受信接口，不再允许只在业务库本地改这些字段。
