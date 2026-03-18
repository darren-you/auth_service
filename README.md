# authbox

`authbox` 是统一认证域的基础能力仓库，面向多个 App / Web / macOS 项目复用。

当前提供的共享模块：

- `session`：统一 access token / refresh token 生成与解析
- `provider/wechat`：微信 OAuth 基础客户端，支持 App 授权码换 token 与 Web 扫码登录地址生成
- `provider/apple`：Apple Sign In 授权码校验与唯一标识提取
- `phone`：短信验证码发送/校验基础服务
- `guest`：游客虚拟设备 ID 签发与校验

设计原则：

- 只沉淀跨项目可复用的认证基础设施
- 不直接绑定某个业务项目的用户表、错误码和响应 DTO
- 允许各项目保留自己的用户升级、注册、资料落库逻辑

推荐接入方式：

1. 项目服务端依赖 `authbox` 的 provider / session 模块。
2. 项目本地实现自己的用户仓储与业务绑定。
3. 项目对外 API 可以保留稳定协议，也可以逐步演进到统一认证域协议。
