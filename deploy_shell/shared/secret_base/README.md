# workspace_secret_base

`workspace_secret_base` 是 `darren_space` 的工作区级 secrets 渲染底座。

当前实现目标：

- 仓库内只保留不含真实 secret 的 YAML 模板
- 服务器侧保存加密后的 `.age` bundle
- 部署时在目标服务器完成校验、解密、渲染
- 容器只读挂载最终 `config.yaml`

## 目录

```text
shared/secret_base/
  README.md
  secretctl/
    go.mod
    main.go
  lib/
    common.sh
    bundle.sh
    render.sh
    remote.sh
    validate.sh
  examples/
    template_server_bundle.yaml
    template_server_config_template.yaml
```

## secretctl

支持以下命令：

```bash
cd deploy_shell/shared/secret_base/secretctl

go run . encrypt \
  --in ../examples/template_server_bundle.yaml \
  --out /tmp/prod.secrets.yaml.age \
  --recipient-file /srv/darren_secret_base/keys/age/active_key.pub

go run . validate \
  --template ../examples/template_server_config_template.yaml \
  --bundle /srv/darren_secret_base/bundles/example_repo/template_server/prod.secrets.yaml.age \
  --identity-file /srv/darren_secret_base/keys/age/active_key.txt \
  --expect-repo example_repo \
  --expect-subproject template_server \
  --expect-env prod

go run . render \
  --template ../examples/template_server_config_template.yaml \
  --bundle /srv/darren_secret_base/bundles/example_repo/template_server/prod.secrets.yaml.age \
  --identity-file /srv/darren_secret_base/keys/age/active_key.txt \
  --out /srv/darren_secret_base/runtime/example_repo/template_server/prod/config.yaml \
  --expect-repo example_repo \
  --expect-subproject template_server \
  --expect-env prod \
  --audit-log /srv/darren_secret_base/logs/render/$(date +%F).log \
  --operator "$(whoami)"
```

## 占位符规范

模板 YAML 统一使用完整标量占位符：

```yaml
mysql:
  password: "{{ secret `mysql.password` }}"
```

注意：

- 占位符必须单独占据整个 YAML 标量值，不支持把它嵌到普通字符串中
- 多行 secret 也使用同一占位符写法，`secretctl render` 会输出合法 YAML
- `bundle.secrets` 中的 key 必须与模板占位符完全一致

## 服务器目录约定

默认根目录：

```text
/srv/darren_secret_base/
  keys/age/
  bundles/<repo>/<subproject>/
  runtime/<repo>/<subproject>/<env>/
  logs/render/
```

如需覆盖，可在 `deploy_config.sh` 中设置：

```bash
SECRET_BASE_ROOT="/srv/darren_secret_base"
SECRET_BASE_REPO="auth_service"
SECRET_BASE_SUBPROJECT="template_server"
SECRET_BASE_ENABLED="true"
```
