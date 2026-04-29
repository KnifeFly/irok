# orik

Kiro 专用的本地 Claude API 兼容代理。后端使用 Go 重写，前端使用 Vite React + TypeScript + shadcn/Base UI，最终通过 Go `embed` 打成一个二进制文件，并用 Docker Compose 部署到 Ubuntu。

## 功能范围

- 只支持 Kiro 逆向接入，其他 IDE 预留为后续 provider 扩展。
- 对外提供 Claude Messages API：
  - `GET /v1/models`
  - `POST /v1/messages`
  - `POST /v1/messages/count_tokens`
- 使用 TOML 管理配置：
  - `config.toml`：全局服务配置
  - `pools.toml`：Kiro 账号池
  - `prompts.toml`：按模型 system prompt 策略
- Web 控制台支持：
  - admin 单账号密码登录
  - Kiro 账号池增删改查
  - Kiro OAuth 启动
  - Kiro 凭据 JSON 导入
  - 凭据过期、即将过期、缺失和刷新能力状态查看
  - 按模型 system prompt 配置
  - 状态、请求统计、日志尾部查看

## 目录结构

```text
cmd/server/              # Go 服务入口
internal/config/         # config.toml 加载与原子写入
internal/pool/           # pools.toml 账号池
internal/prompt/         # prompts.toml system prompt 策略
internal/provider/kiro/  # Kiro 请求转换、token 刷新、Claude 响应封装
internal/auth/kiro/      # OAuth 和凭据导入
internal/httpapi/        # HTTP 路由、管理 API、静态资源服务
internal/assets/         # 嵌入的前端构建产物
web/                     # Vite React 前端
config/*.example         # 配置模板
docker-compose.yml       # 推荐部署入口
```

## 本地开发

要求：

- Go 1.24+
- Node.js 22+
- npm

安装前端依赖：

```bash
cd web
npm ci
```

构建前端并嵌入 Go：

```bash
cd web
npm run build
```

运行后端：

```bash
cd ..
go run ./cmd/server --config config/config.toml
```

首次运行如果 `config/config.toml`、`config/pools.toml`、`config/prompts.toml` 不存在，服务会按默认值创建。生产环境建议先从模板复制并修改：

```bash
cp config/config.toml.example config/config.toml
cp config/pools.toml.example config/pools.toml
cp config/prompts.toml.example config/prompts.toml
```

默认访问：

```text
http://127.0.0.1:13120
```

## Docker Compose 部署

推荐部署方式：

```bash
docker compose up -d --build
```

Compose 会挂载当前目录的 `./config` 到容器内 `/app/config`。配置和凭据都应放在这个目录下，容器重建不会丢失。

如果 Docker 拉取基础镜像超时，请先修复 Docker registry 镜像源或网络，再重新执行：

```bash
docker compose build
docker compose up -d
```

## 配置说明

### config.toml

核心字段：

```toml
[server]
host = "0.0.0.0"
port = 13120
admin_api_key = "change-me"
public_url = "http://127.0.0.1:13120"

[files]
pools_path = "config/pools.toml"
prompts_path = "config/prompts.toml"
credentials_dir = "config/credentials/kiro"
```

`admin_api_key` 用于保护 `/api/*` 管理接口和 `/v1/*` 模型接口。Web 控制台只有一个 admin 账号，登录密码就是这个 key。前端默认只把密码保存在当前浏览器会话中，勾选“保持登录”后才写入本机 `localStorage`。

### pools.toml

账号池由页面写入，也可以手动编辑：

```toml
[kiro]
nodes = []
```

凭据文件建议放在：

```text
config/credentials/kiro/<node-id>.json
```

凭据包含 `refreshToken` 时，后端会在 access token 临近过期时自动刷新；请求返回 401 时也会尝试刷新一次。控制台的 Kiro 账号池会显示凭据是否有效、即将过期、已过期、文件缺失或 JSON 无效；无法自动刷新的节点需要重新 OAuth 或重新导入凭据。

### prompts.toml

按模型配置 system prompt：

```toml
[[prompts]]
model = "*"
enabled = false
mode = "prepend"
content = ""
note = "Default prompt rule"
```

`mode` 支持：

- `prepend`：追加到请求 system prompt 前面
- `append`：追加到请求 system prompt 后面
- `override`：覆盖请求 system prompt
- `off`：不注入

模型精确匹配优先，找不到时使用 `*` 规则。

## API 示例

列出模型：

```bash
curl http://127.0.0.1:13120/v1/models \
  -H "Authorization: Bearer change-me"
```

发送 Claude Messages 请求：

```bash
curl http://127.0.0.1:13120/v1/messages \
  -H "Authorization: Bearer change-me" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5",
    "max_tokens": 1024,
    "messages": [
      { "role": "user", "content": "hello" }
    ]
  }'
```

流式请求：

```bash
curl http://127.0.0.1:13120/v1/messages \
  -H "Authorization: Bearer change-me" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5",
    "max_tokens": 1024,
    "stream": true,
    "messages": [
      { "role": "user", "content": "hello" }
    ]
  }'
```

## 验证

后端测试：

```bash
go test ./...
```

前端类型检查与构建：

```bash
cd web
npm run typecheck
npm run build
```

Docker Compose 配置检查：

```bash
docker compose config
```

## 当前限制

- v1 只实现 Kiro provider。
- OpenAI/Gemini/Grok/Codex 兼容层暂不迁移。
- OAuth Builder ID 采用后台轮询保存凭据，页面暂不做实时轮询状态流。
- Token 计数为本地估算，不等同 Anthropic 官方 tokenizer。
