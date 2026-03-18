# auth_service

`auth_service` 是统一认证域仓库，同时提供共享认证内核和独立部署的认证服务。

## 共享认证内核

- `session`：统一 access token / refresh token 生成、解析和 Bearer token 提取
- `provider/wechat`：微信 OAuth 客户端，支持 Web 扫码登录地址生成、换码、刷新和用户信息获取
- `provider/apple`：Apple Sign In 授权码校验和唯一标识提取
- `phone`：短信验证码发送与校验基础服务
- `guest`：游客设备 ID 生成与校验

## 独立认证服务

`template_server` 是 `AuthServer.md` 对应的 `auth_service` 实现，按统一认证域分成：

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

1. 新项目优先直接对接 `auth_service` 服务协议。
2. 老项目如果暂时不切远程协议，仍可以继续复用根仓库共享认证内核。
3. 新租户只补 YAML 配置，不再重复开发微信、Apple、验证码和游客登录底层逻辑。
