# auth_service

`auth_service` 是统一认证域仓库，面向业务工程提供独立部署的认证服务，并沉淀其内部使用的会话、Provider 与客户端 SDK。

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
- `POST /api/v1/auth/providers/:provider/callback`
- `POST /api/v1/auth/providers/phone/send-captcha`
- `POST /api/v1/auth/providers/guest/device-id`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `GET /api/v1/auth/me`

## 接入原则

1. 业务工程统一接入远程 `auth_service` 协议，不再各自维护本地登录内核。
2. 新租户通过 YAML 配置和业务 bridge 接口接入，不重复开发微信、Apple、账号密码、验证码和游客登录流程。
3. `session`、`provider/*`、`client` 这些包服务于 `auth_service` 自身和接入 SDK，不作为业务工程继续保留本地认证实现的理由。
