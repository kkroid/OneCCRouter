# OneCCRouter

**个人 AI 模型路由网关** — 将 GitHub Copilot Claude 模型 + 任意 Anthropic-compatible API 统一暴露为单一 Anthropic 接口，供 [Claude Code](https://docs.anthropic.com/en/docs/claude-code) 等工具使用。

**21 MB 单文件，零运行时依赖。**

## 架构

```
Claude Code CLI / VS Code / 其他工具
              │
              ▼  Anthropic Messages API (localhost:3456)
    ┌─────────────────────────┐
    │   onecc-router (Go)     │  ← 守护进程
    │   · HTTP proxy          │
    │   · 协议翻译             │
    │   · gRPC server (:1083) │
    └──────┬──────────────────┘
           │ gRPC
           ▼
    ┌─────────────────────────┐
    │   OneCC Panel (QT/C++)  │  ← 桌面管理面板（后续）
    └─────────────────────────┘
```

## 可用模型

由 `.env` 中的 `PROVIDER_<PREFIX>_*` 变量定义：

| 前缀 | 模型 ID | 说明 |
|------|--------|------|
| `cp/` | `claude-opus-4.8` | GitHub Copilot |
| `cp/` | `claude-fable-5` | GitHub Copilot |
| `ds/` | `deepseek-v4-pro` | DeepSeek（示例） |
| `ds/` | `deepseek-v4-flash` | DeepSeek（示例） |

> 添加新 provider：`.env` 中加 `PROVIDER_XX_*` 变量，重启或通过 gRPC `ReloadConfig` 热加载。

## 前置条件

- **Go 1.22+**（仅编译需要）

## 快速开始

### 1. 编译

```bash
git clone <repo-url> && cd OneCCRouter
go build -o build/onecc-router.exe ./cmd/onecc-router/
```

### 2. 配置

```bash
# 服务配置（端口、日志、代理）
cp onecc-router.example.yaml onecc-router.yaml

# Provider 配置（模型、API Key）
cp .env.example .env
# 编辑 .env 填入你的 API Key
```

`.env` 格式：

```env
PROVIDER_DS_NAME=DeepSeek
PROVIDER_DS_BASE_URL=https://api.deepseek.com/anthropic
PROVIDER_DS_API_KEY=sk-your-key
PROVIDER_DS_MODELS=deepseek-v4-pro,deepseek-v4-flash

PROVIDER_CP_NAME=GitHub Copilot
PROVIDER_CP_API_KEY=not-needed
PROVIDER_CP_MODELS=claude-opus-4.8,claude-fable-5
```

### 3. 启动（首次自动引导登录）

```bash
./build/onecc-router.exe
```

如果配置了 Copilot 但未登录，会自动弹出 GitHub 设备授权流程。Token 保存在 `~/.onecc/github_token`。

### 4. 启动

```bash
./build/onecc-router.exe serve
```

```
🚀 onecc-router starting on 127.0.0.1:3456 (HTTP) + 127.0.0.1:1083 (gRPC)
```

### 5. 验证

```bash
# 健康检查
curl http://localhost:3456/health

# 模型列表
curl http://localhost:3456/v1/models

# 非流式推理
curl -X POST http://localhost:3456/v1/messages \
  -H "Content-Type: application/json" \
  -d '{"model":"cp/claude-opus-4.8","max_tokens":50,"messages":[{"role":"user","content":"hi"}]}'

# 流式推理
curl -N -X POST http://localhost:3456/v1/messages \
  -H "Content-Type: application/json" \
  -d '{"model":"cp/claude-opus-4.8","max_tokens":100,"stream":true,"messages":[{"role":"user","content":"hello"}]}'
```

## Claude Code 配置

```json
{
  "apiKey": "x",
  "baseUrl": "http://localhost:3456/v1",
  "model": "cp/claude-opus-4.8"
}
```

## CLI 命令

```bash
onecc-router                # 启动守护进程（无 token 自动引导登录）
onecc-router --daemon       # 后台运行
onecc-router status         # 检查运行状态
onecc-router --help         # 帮助
onecc-router --version      # 版本
```

## gRPC 接口（QT 面板对接）

```protobuf
service ProxyService {
  rpc ListModels(ListModelsRequest) returns (ListModelsResponse);
  rpc ListProviders(ListProvidersRequest) returns (ListProvidersResponse);
  rpc GetStatus(GetStatusRequest) returns (GetStatusResponse);
  rpc ReloadConfig(ReloadConfigRequest) returns (ReloadConfigResponse);
}
```

监听 `127.0.0.1:1083`，可通过 `grpcurl` 调试：

```bash
grpcurl -plaintext 127.0.0.1:1083 list
```

## 项目结构

```
OneCCRouter/
├── cmd/onecc-router/main.go           # CLI 入口 (serve/login/status)
├── internal/
│   ├── auth/                          # GitHub device OAuth + token 管理
│   ├── config/                        # YAML 配置加载
│   ├── grpc/                          # gRPC ProxyService 实现
│   ├── log/                           # slog + lumberjack + request_id
│   ├── proxy/                         # HTTP 代理 (Copilot + External)
│   ├── router/                        # Provider 解析 + 模型路由
│   └── translate/                     # Anthropic ↔ OpenAI 协议翻译
├── proto/onecc/v1/service.proto       # gRPC 接口定义
├── onecc-router.example.yaml          # 服务配置模板
├── .env.example                       # Provider 配置模板
└── go.mod
```

## 配置参考

### onecc-router.yaml

```yaml
server:
  host: "127.0.0.1"
  http_port: 3456
  grpc_port: 1083

log:
  level: "info"
  dir: "~/.onecc/logs"
  max_size_mb: 100
  max_age_days: 30
  max_backups: 10

proxy:
  http_proxy: "socks5h://127.0.0.1:1082"
  https_proxy: "socks5h://127.0.0.1:1082"
```

## 日志

JSON 格式，按天滚动，保留 30 天：

```json
{"time":"2026-06-11T10:30:00.000+08:00","level":"INFO","msg":"proxy request","request_id":"a1b2c3d4","method":"POST","model":"cp/claude-opus-4.8","status":200}
```

在 `~/.onecc/logs/onecc-router.log` 中查看。
