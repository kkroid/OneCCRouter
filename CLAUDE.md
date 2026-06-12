# OneCCRouter Claude Code 开发指南

## 项目定位

**个人 AI 模型路由网关** — 将 GitHub Copilot Claude 模型 + 任意 Anthropic-compatible API 统一暴露为单一 Anthropic 接口，供 Claude Code 等工具使用。

- 每人独立部署一套，单机运行
- 不追求高并发（几个工具同时调用）
- **稳定性第一**：不能崩、不能内存泄漏、异常可复现
- **可排查性好**：日志日期滚动、错误栈完整、请求可追踪

## 架构

```
Claude Code CLI / VS Code / 其他工具
              │
              ▼  Anthropic Messages API (localhost)
    ┌─────────────────────────┐
    │   onecc-router (Go)     │  ← 单二进制守护进程
    │   · HTTP proxy          │
    │   · 协议翻译             │
    │   · gRPC server         │
    └─────────────────────────┘
```

## 开发环境

| 项目 | 详情 |
|------|------|
| 操作系统 | Windows 11 Pro (10.0.26200) |
| Shell | bash（Git Bash on Windows） |
| SOCKS5 代理 | `127.0.0.1:1082`（用于访问 GitHub Copilot API 等外网） |
| IDE | VS Code（主力）+ Claude Code（AI 辅助） |

### 技术栈

| 层 | 语言 | 框架/库 | 说明 |
|----|------|---------|------|
| 守护进程 | **Go** 1.22+ | gRPC, protobuf, cobra, lumberjack | 代理核心 + API 路由 + 协议翻译 |

### 为什么 Go？

- **稳定性**：GC + 内存安全，没有悬空指针/buffer overflow，守护进程长期运行更可靠
- **可排查性**：panic 自动输出完整 goroutine 栈；`context.Context` 天然携带 trace ID；error wrapping 递归展开错误链
- **网络代理是 Go 的主场**：`net/http` + goroutine + channel 写代理/转发/SSE 流式数据比 C++ 少 3-5 倍代码量，更不容易出错
- **部署简单**：单个 ~15MB exe，无运行时依赖，复制就能跑

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
├── onecc-router.example.yaml          # 配置模板
└── go.mod
```

## Claude Code 项目斜杠命令

| 命令 | 文件 | 用途 |
|------|------|------|
| `/build` | `.claude/commands/build.md` | Go 编译 + 验证 |
| `/commit` | `.claude/commands/commit.md` | Conventional Commit + Co-Authored-By 签名 |
| `/review` | `.claude/commands/review.md` | Go 代码审查 |
| `/plan` | `.claude/commands/plan.md` | 任务分解 + 验证标准定义 |

---

## 核心行为准则

### 1. 编码前思考 — 不假设、不隐藏困惑、呈现权衡

动手前必须：
- **明确陈述假设**。如果不确定，**必须先问**，不要默默选择一种理解。
- 如果存在多种解释，**全部列出**，不要替我选。
- 如果存在更简单、更稳妥的做法，**大胆指出并给出建议**。
- 如果我的需求有问题，**必须及时指出**，不要盲目执行。

### 2. 简洁优先 — 用最少代码解决问题，不做推测性开发

- 用能解决问题的最少代码完成任务，**不添加需求之外的功能**。
- **不为一次性逻辑创建抽象**，不添加未要求的灵活性。
- 不为不可能发生或没有证据表明会发生的场景堆叠防御代码。
- 如果实现明显可以更短、更清晰，**主动收敛复杂度**。

### 3. 精准修改 — 只碰必须碰的，只清理自己造成的混乱

- **只修改完成当前请求必须修改的内容**；不要顺手重构、改格式。
- **匹配项目现有风格**，即使你个人更倾向于另一种写法。
- 对自己没有充分理解的现有代码，**不要做旁路改动**。
- 如果发现无关的死代码或历史问题，**可以汇报，但不要擅自删除**。

### 4. 验证闭环 — 定义成功标准，循环验证直到达成

- 每步必须有可验证的输出（编译通过？gRPC 调用返回正确？代理请求正常？）
- 完成明确步骤后，**汇报结果、风险、验证情况和建议的下一步**。
- 如果验证失败，先基于证据定位原因。

---

## 技术约定

### Go 后台

| 约定 | 说明 |
|------|------|
| 版本 | Go 1.22+ |
| 模块 | `github.com/kkroid/onecc-router` |
| CLI 框架 | `cobra`（子命令） + `viper`（配置绑定） |
| 日志 | `log/slog` + `lumberjack`（日期滚动：按天切分，保留 30 天，单文件最大 100MB） |
| 请求 ID | 每个请求生成 UUID，通过 `context.Context` 传递，日志全部带 request_id |
| 错误处理 | `fmt.Errorf("...: %w", err)` 包装错误链，绝不吞错误 |
| 代理 | 尊重 `HTTP_PROXY`/`HTTPS_PROXY` 环境变量；Copilot API 走 SOCKS5 代理 |
| gRPC | `google.golang.org/grpc`，监听 `127.0.0.1` 仅本地 |
| HTTP | `net/http` 标准库 |
| 测试 | `go test`，表驱动测试 |

#### 日志格式

```json
{"time":"2026-06-11T10:30:00.000+08:00","level":"INFO","msg":"proxy request","request_id":"a1b2c3d4","method":"POST","model":"cp/claude-opus-4.8","duration_ms":1234,"status":200}
```

### protobuf / gRPC

| 约定 | 说明 |
|------|------|
| 语法 | proto3 |
| 通信 | `grpc.InsecureCredentials()`（localhost only） |
| 服务 | `ProxyService` — 11 RPCs（状态、模型、provider、登录、配置保存） |

### 构建命令

```bash
# 编译
go build -ldflags="-s -w" -o build/onecc-router.exe ./cmd/onecc-router/
```

### 配置文件

- `onecc-router.yaml`：完整配置（端口、日志、代理、provider、model slots）
- `onecc-router.example.yaml`：配置模板（提交到 git）

---

## 关键限制与约束

1. **稳定性 > 功能**：新功能可以慢慢加，但不能引入崩溃或内存问题。
2. **每个请求可追踪**：日志必须带 request_id，出问题时能复现完整请求链路。
3. **协议翻译必须精确**：Anthropic ↔ OpenAI 语义等价，不丢失字段。
4. **仅本地通信**：gRPC 和 HTTP API 都只监听 `127.0.0.1`，不暴露到网络。
5. **Git 提交信息末尾必须包含**：
   ```
   Generated with [Claude Code](https://claude.ai/code)
   via [Happy](https://happy.engineering)

   Co-Authored-By: Claude <noreply@anthropic.com>
   Co-Authored-By: Happy <yesreply@happy.engineering>
   ```

---

## 验证方式

```bash
# 1. 启动
./build/onecc-router.exe
# 预期日志：[INFO] onecc-router starting on 127.0.0.1:3456

# 2. 健康检查
curl http://localhost:3456/health
# 预期：{"status":"ok"}

# 3. 模型列表
curl http://localhost:3456/v1/models

# 4. 非流式代理请求
curl -X POST http://localhost:3456/v1/messages \
  -H "Content-Type: application/json" \
  -d '{"model":"cp/claude-opus-4.8","max_tokens":50,"messages":[{"role":"user","content":"hi"}]}'

# 5. 流式请求
curl -N -X POST http://localhost:3456/v1/messages \
  -H "Content-Type: application/json" \
  -d '{"model":"cp/claude-opus-4.8","max_tokens":100,"stream":true,"messages":[{"role":"user","content":"hello"}]}'

# 6. gRPC 调用
grpcurl -plaintext 127.0.0.1:1083 onecc.v1.ProxyService/GetStatus
```
