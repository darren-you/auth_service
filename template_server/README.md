# auth_service_template_server

## 项目定位

`auth_service/template_server` 是统一认证域的独立服务实现，负责多租户认证配置同步、第三方登录编排、会话签发、刷新、登出和 `me` 查询。

## 核心能力

- 微信 Web/App 登录（App 侧通过 `login-url` 预取一次性 `state`）
- Apple 登录
- 手机验证码登录
- Getui 手机号快捷登录
- 游客登录
- 多租户 `tenant / provider / client_type` 配置同步
- `access token / refresh token` 签发与轮换
- 统一 `auth_users / auth_identities / auth_sessions` 存储

## 核心接口

- `GET /api/v1/health`
- `GET /api/v1/auth/providers/:provider/login-url`
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
