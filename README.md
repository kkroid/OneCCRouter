# OneCCRouter

基于 [9router](https://github.com/decolua/9router) 的 AI 模型路由网关，将 GitHub Copilot Claude 模型及任意 Anthropic-compatible API 统一暴露为单一入口，供 [Claude Code](https://docs.anthropic.com/en/docs/claude-code) 等工具使用。

## 架构

```
Claude Code CLI
      │
      ▼
  localhost:3456  ───  9router (Provider Router)
      │
      ├── cp/claude-opus-4.8  ──▶  copilot-anthropic:4142  ──▶  Copilot API
      ├── cp/claude-fable-5   ──▶  copilot-anthropic:4142  ──▶  Copilot API
      └── ds/* | your/*       ──▶  任意 Anthropic-compatible API
```

| 组件 | 说明 |
|------|------|
| **9router** | AI 模型路由网关，统一 Anthropic-compatible 入口 (`:3456`) |
| **copilot-anthropic** | 将 Copilot OpenAI 格式翻译为 Anthropic 格式的代理 (`:4142`) |
| **register-providers** | 一次性启动脚本，读取 `providers.json` 自动注册所有 provider，并生成 Claude Code 配置文件 |

## 可用模型

模型由 [`providers.json`](providers.json) 定义，默认包含：

| 前缀 | 模型 ID | 来源 |
|------|--------|------|
| `cp/` | `claude-opus-4.8` | GitHub Copilot |
| `cp/` | `claude-fable-5` | GitHub Copilot |
| `ds/` | `deepseek-v4-pro` | 任意 Anthropic API |
| `ds/` | `deepseek-v4-flash` | 任意 Anthropic API |

> 添加新 provider 只需编辑 `providers.json`，重新运行 `podman compose up -d` 即可。

## 前置条件

- **Podman** 或 **Docker** + Docker Compose
- **代理 `127.0.0.1:1082`** — GitHub Copilot 对 Claude 模型有 IP 区域限制（Clash/V2Ray 等）
- **各 provider 的 API Key**（按需配置）
- **GitHub Device Token** — 用于 Copilot API 认证（见步骤 3）

## 部署步骤

### 1. 克隆项目

```bash
git clone <repo-url>
cd OneCCRouter
```

### 2. 配置环境变量

```bash
cp .env.example .env
```

编辑 `.env`，按 `providers.json` 中 `apiKeyEnv` 字段填入对应的 Key：

```env
PROVIDER_DS_KEY=sk-xxxxxxxx
```

### 3. 获取 GitHub Copilot Token

```bash
podman run --rm -it \
  -v ./copilot-anthropic-proxy/github_token:/root/.local/share/copilot-api/github_token \
  ghcr.io/ericc-ch/copilot-api:latest \
  bun run auth.js
```

按提示打开 GitHub 验证页面，完成后 token 自动保存。

### 4. 启动服务

```bash
podman compose up -d
```

首次启动会自动 pull 镜像、构建 proxy、注册所有 provider。启动后会在项目 `out/` 目录生成 Claude Code 配置文件。

### 5. 验证

```bash
curl -X POST http://localhost:3456/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: x" \
  -d '{"model":"cp/claude-opus-4.8","max_tokens":50,"messages":[{"role":"user","content":"say hi"}]}'
```

## 在 Claude Code 中使用

启动后会自动生成 `out/claude-code-settings.json`，复制到 Claude Code 配置目录即可：

```json
{
  "apiKey": "x",
  "baseUrl": "http://localhost:3456/v1",
  "model": "cp/claude-opus-4.8"
}
```

`_availableModels` 字段列出了所有可用模型，可按需切换 `model` 值。

## 自定义 Provider

编辑 `providers.json` 添加新的 Anthropic-compatible API：

```json
{
  "name": "My Provider",
  "prefix": "my",
  "baseUrl": "https://my-api.example.com/anthropic",
  "apiKeyEnv": "PROVIDER_MY_KEY",
  "models": ["model-a", "model-b"]
}
```

然后在 `.env` 中填入 Key，重新启动即可。

## 管理

```bash
# 查看状态
podman compose ps

# 查看日志
podman compose logs -f

# 重启（修改 providers.json 后）
podman compose up -d
```

9router Web 管理界面: [http://localhost:3456](http://localhost:3456)（默认密码 `123456`）

## 项目结构

```
.
├── copilot-anthropic-proxy/   # Copilot → Anthropic 格式转换代理
│   ├── src/
│   │   ├── index.ts           # Hono 服务入口
│   │   ├── auth.ts            # Copilot token 管理
│   │   ├── translate.ts       # Anthropic ↔ OpenAI 格式翻译
│   │   └── types.ts           # 类型定义
│   ├── models.conf            # 允许的 Copilot 模型列表
│   ├── github_token           # Copilot 设备 token（gitignore）
│   ├── Dockerfile
│   └── package.json
├── providers.json             # 所有 Anthropic provider 配置（单一数据源）
├── docker-compose.yml         # 容器编排
├── register-providers.sh      # 自动注册 + 生成 Claude Code 配置
├── out/                       # 生成的配置文件（gitignore）
├── .env.example               # 环境变量模板
└── .env                       # API Keys（gitignore）
```
