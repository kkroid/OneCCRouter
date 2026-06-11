# OneCCRouter Claude Code 开发指南

## 项目定位

单容器 AI 模型路由网关。将 GitHub Copilot Claude 模型 + 任意 Anthropic-compatible API 统一暴露为单一 Anthropic 接口，供 Claude Code 等工具使用。

**167 MB，3 秒启动，零外部依赖。**

## 开发环境

| 项目 | 详情 |
|------|------|
| 操作系统 | Windows 11 Pro (10.0.26200) |
| Shell | bash（Git Bash on Windows） |
| 运行时 | Node.js（通过 Docker/Podman 容器内 `tsx` 运行） |
| 语言 | TypeScript |
| Web 框架 | Hono 4.x |
| 容器 | Podman / Docker + Docker Compose |
| SOCKS5 代理 | `127.0.0.1:1082`（用于访问 GitHub API） |
| IDE | VS Code（`.vscode/settings.json` 已配置） |

## 架构

```
Claude Code CLI
      │
      ▼
  localhost:3456  ───  OneCC Proxy（单容器）
      │
      ├── cp/claude-*  ──▶  Copilot API（Anthropic ↔ OpenAI 翻译）
      └── ds/* | 任意  ──▶  Anthropic-compatible API（直通）
```

## 项目结构

```
.
├── copilot-anthropic-proxy/
│   ├── src/
│   │   ├── index.ts        # Hono 服务入口 + 路由
│   │   ├── router.ts       # Provider 注册 + 模型解析 + settings 生成
│   │   ├── auth.ts         # GitHub Copilot device token
│   │   ├── translate.ts    # Anthropic ↔ OpenAI 请求/响应/流式翻译
│   │   └── types.ts        # TypeScript 类型定义
│   ├── models.conf         # Copilot 模型白名单
│   ├── github_token        # 设备 token（gitignore）
│   ├── Dockerfile
│   └── package.json
├── docker-compose.yml      # 单容器编排
├── out/                    # 生成的 Claude Code 配置
├── .env.example
└── .env                    # Provider 配置（gitignore）
```

## 核心模块

| 模块 | 文件 | 功能 |
|------|------|------|
| 服务入口 | `src/index.ts` | Hono 服务启动、路由注册、启动流程 |
| 路由/模型 | `src/router.ts` | Provider 解析、模型解析、Claude Code settings 生成 |
| 认证 | `src/auth.ts` | GitHub device token 获取/刷新 |
| 翻译层 | `src/translate.ts` | Anthropic ↔ OpenAI 协议翻译（请求/响应/SSE） |
| 类型 | `src/types.ts` | TypeScript 类型定义 |

### Claude Code 项目斜杠命令

| 命令 | 文件 | 用途 |
|------|------|------|
| `/build` | `.claude/commands/build.md` | Docker 镜像构建 + 容器启动 |
| `/commit` | `.claude/commands/commit.md` | 生成 Conventional Commit + Co-Authored-By 签名 |
| `/review` | `.claude/commands/review.md` | TypeScript 代码审查（类型安全、错误处理、协议正确性） |
| `/plan` | `.claude/commands/plan.md` | 任务分解 + 验证标准定义 |

### Claude Code 技能

| 技能 | 调用方式 | 用途 |
|------|---------|------|
| `checklist` | `/checklist` | 大任务分解为结构化 checklist，逐项推进勾选 |
| `audit` | `/audit` | 阶段性自我审核——审查代码/结果/文档，发现问题自动修复 |

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

- 每步必须有可验证的输出（编译通过？容器启动？API 返回正确？）
- 完成明确步骤后，**汇报结果、风险、验证情况和建议的下一步**。
- 如果验证失败，先基于证据定位原因。

---

## 技术约定

### TypeScript（`copilot-anthropic-proxy/src/`）

| 约定 | 说明 |
|------|------|
| 模块系统 | ESM（`"type": "module"`） |
| 运行时 | `tsx`（Node.js TypeScript 执行器） |
| Web 框架 | Hono 4.x |
| 流式 | SSE（Server-Sent Events），通过 Hono `streamSSE` |
| 协议 | Anthropic Messages API ↔ OpenAI Chat Completions |
| 容器 | Docker/Podman，`docker-compose.yml` 编排 |

### 构建部署

```bash
# 构建并启动
podman compose up -d

# 查看日志
podman compose logs -f

# 重启（修改 .env 后）
podman compose up -d --build

# 停止
podman compose down
```

### 代理

- 容器内通过 `http_proxy=http://host.containers.internal:1082` 访问外网
- 主机 curl 使用 `--proxy socks5h://127.0.0.1:1082`
- GitHub Copilot API 有 IP 区域限制，必须走代理

---

## 关键限制与约束

1. **协议翻译必须精确**：Anthropic ↔ OpenAI 的请求/响应/流式翻译必须语义等价，不得丢失字段。
2. **容器启动不能阻塞于认证**：服务先启动，设备授权在后台进行。
3. **Provider 配置通过 `.env`**：不硬编码任何 provider 信息。
4. **所有改动需在容器中验证**：修改源码后 `podman compose up -d --build` 重建。
5. **Git 提交信息末尾必须包含**：
   ```
   Generated with [Claude Code](https://claude.ai/code)
   via [Happy](https://happy.engineering)

   Co-Authored-By: Claude <noreply@anthropic.com>
   Co-Authored-By: Happy <yesreply@happy.engineering>
   ```

---

## 验证方式

修改代码后的验证流程：

```bash
# 1. 重建并启动
podman compose up -d --build

# 2. 健康检查
curl http://localhost:3456/health

# 3. 模型列表
curl http://localhost:3456/v1/models

# 4. 非流式请求
curl -X POST http://localhost:3456/v1/messages \
  -H "Content-Type: application/json" \
  -d '{"model":"cp/claude-opus-4.8","max_tokens":50,"messages":[{"role":"user","content":"hi"}]}'

# 5. 查看日志
podman compose logs -f
```
