# auth_service_template_server

## 遵循标准

- 工作区 Server 标准：[`../../harness/docs/workspace/standards/server/server_golden_path.md`](../../harness/docs/workspace/standards/server/server_golden_path.md)

## 文档入口

- API 契约入口：[`./auth.api`](./auth.api)
- 历史偏离记录（已回收）：[`../../harness/docs/design/auth_service/server/server_deviation_observability_semantics.md`](../../harness/docs/design/auth_service/server/server_deviation_observability_semantics.md)

## 项目定位

`auth_service/template_server` 是统一认证域的服务子工程，基于 `go-zero` 提供多租户认证配置、第三方登录编排、会话签发、刷新、登出和 `me` 查询能力。

## 当前目录重点

- `auth.api`：认证服务 API 协议定义。
- `internal/handler`、`internal/logic`：`go-zero` 生成的路由处理与业务逻辑入口。
- `internal/model`：认证域数据模型。
- `pkg/provider`、`pkg/session`：Provider 适配和 token/session 规则。

## 核心能力

- 微信 Web/App/小程序登录
  - Web：provider 使用 `wechat_web`，`login-url` 返回 OAuth 地址
  - App：provider 使用 `wechat_app`，`login-url` 只预取一次性 `state`
  - 小程序：provider 使用 `wechat_miniprogram`，前端调用 `wx.login` 获取 `code` 后，直接调用 `POST /api/v1/auth/providers/wechat_miniprogram/callback`
- Firebase Authentication 登录
  - provider 使用 `firebase_auth`
  - 客户端先通过 Firebase SDK 完成 Google 登录并取得 Firebase ID token
  - 业务 server 将 ID token 转发到 `POST /api/v1/auth/providers/firebase_auth/callback`
  - `auth_service` 按租户配置的 Firebase `project_id` 校验 ID token 后，同步业务用户并签发统一业务 token
- Apple 登录
- 手机验证码登录
- 手机验证码登录与已登录态绑定当前业务账号
- 手机验证码可按 `scene=login|bind|rebind` 选择短信模板；`elook` 小程序当前使用「小程序登录验证码」与「小程序换绑验证码」两套腾讯云模板。
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
- `PUT /api/v1/auth/me`
- `PUT /api/v1/auth/internal/users`：仅供业务 server 使用 `X-Auth-Service-Key` 回写指定业务用户的共享资料字段

## 微信小程序接入约定

- `provider` 固定使用 `wechat_miniprogram`
- `client_type` 约定使用 `miniprogram`
- 小程序端先调用微信官方 `wx.login` 获取 `code`
- 服务端回调接口使用 `POST /api/v1/auth/providers/wechat_miniprogram/callback`
- 请求体最小字段为：

```json
{
  "tenant_key": "your_tenant",
  "client_type": "miniprogram",
  "code": "wx.login 返回的 code"
}
```

- `miniprogram` 不使用 `GET /api/v1/auth/providers/wechat_miniprogram/login-url` 返回的 `login_url/state`
- 对应租户的 provider 配置需要补齐：
  - `provider: wechat_miniprogram`
  - `client_type: miniprogram`
  - `app_id`
  - `app_secret`
- 如果该项目需要统一默认头像，应在同一个租户配置中补齐 `default_avatar_url`；认证服务会在签发 session、刷新 token、`/auth/me` 和内部用户资料接口返回资料时按租户归一化头像

## 微信 provider 约定

- App 登录：`provider=wechat_app`，`client_type=app`
- Web 登录：`provider=wechat_web`，`client_type=web`
- 小程序登录：`provider=wechat_miniprogram`，`client_type=miniprogram`
- 业务 bridge 仍接收 `provider=wechat`，作为微信用户同步的统一业务语义，不和认证侧 provider 拆分耦合

## Firebase provider 约定

- 认证侧 provider 固定使用 `firebase_auth`
- 租户 Provider 配置必须提供 Firebase `project_id`，当前放在 `client_id`；也可通过 `extra_json.project_id` 提供
- 业务 bridge 接收 `provider=firebase` 与 `provider_subject=<firebase uid>`，不要把 Firebase UID 写入微信 OpenID 字段
- Firebase 在架构上只作为统一认证域的 Provider，业务项目不直接校验 Firebase 登录态，也不直接签发业务 token

## 业务 bridge 对接约定

`auth_service` 在认证成功后会回调业务侧 `POST /api/v1/internal/auth/sync`。微信小程序登录时，业务 bridge 至少需要支持以下字段：

```json
{
  "tenant_key": "elook",
  "provider": "wechat",
  "client_type": "miniprogram",
  "open_id": "wechat-open-id",
  "union_id": "wechat-union-id",
  "session_key": "wechat-session-key",
  "display_name": "微信昵称",
  "avatar_url": "https://...",
  "current_user_id": 0,
  "current_user_role": ""
}
```

- `provider=wechat` 时，业务侧应优先按 `open_id` 查用户，必要时回退 `union_id`
- 微信小程序链路下，业务侧应落库保存 `open_id / union_id / session_key`
- 首次登录需要落本地用户，并返回 `user_id / display_name / avatar_url / role / status`
- 已有用户再次登录时，应补齐空缺的 `union_id`，并按业务需要更新头像或展示名
- 如果业务侧在“已登录态绑定手机号 / 绑定新登录方式”场景下调用 provider callback，应透传 `current_user_id / current_user_role`，业务 bridge 需要把该登录方式绑定到当前业务用户，而不是创建新的业务账号
- 当业务侧允许用户修改共享资料字段（如昵称、头像）时，应同步调用 `PUT /api/v1/auth/me` 回写认证域，保证后续登录、刷新 token 和 `/auth/me` 的资料读写一致
- 当业务侧管理后台需要修改其他用户的共享资料字段（如昵称、头像、角色、状态）时，应调用 `PUT /api/v1/auth/internal/users`，并使用租户配置中的 `bridge_auth_key` 作为受信鉴权，不再直接把这些字段只写在业务库里

## 手机验证码短信配置

手机号验证码 provider 支持在租户配置中使用结构化 `sms` 配置，并在服务启动同步到 `auth_provider_configs.extra_json`。真实密钥继续使用 `secret_base` 占位符；非敏感的 `sms_sdk_app_id`、签名、模板 ID 与模板参数顺序保留在 YAML 中统一 Git 管理。

`scene` 语义：

- `login`：手机号验证码登录。
- `bind`：首次绑定手机号。
- `rebind`：换绑手机号；如未单独配置模板，会回退使用 `bind` 模板。

## 租户默认头像

每个租户可在 `config/config.dev.yaml` 与 `config/config.prod.yaml` 中按项目配置默认头像：

```yaml
auth:
  tenants:
    - key: elook
      default_avatar_url: https://files.xdarren.com/elook/images/defaults/avatar.jpg
      legacy_default_avatar_urls:
        - https://files.xdarren.com/elook/images/defaults/avatar.png
```

当认证域内头像为空，或等于该租户 `legacy_default_avatar_urls` 中的旧默认头像时，`auth_service` 会返回当前租户的 `default_avatar_url`。用户真正上传过的自定义头像保持原值，不会被默认头像覆盖。

## 部署

```bash
PROJECT_ROOT=/absolute/path/to/auth_service

BuildBranch=origin/master BuildEnv=prod \
bash "$PROJECT_ROOT/fast_deploy/deploy_server/remote_deploy_pipeline.sh" \
  --config "$PROJECT_ROOT/template_server/deploy_config.sh"
```
