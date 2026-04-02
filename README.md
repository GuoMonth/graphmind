# GraphMind

**为 AI Agent 原生打造的图式项目管理工具。**

GraphMind 是一个 local-first 的项目管理 CLI，专为 [Claude Code](https://docs.anthropic.com/en/docs/claude-code)、[Codex](https://openai.com/index/codex/)、[Copilot](https://github.com/features/copilot) 等 AI Agent 设计。人不直接操作 GraphMind——人和 AI Agent 对话，AI Agent 调用 GraphMind 读写图。

> _"我只需要描述发生了什么，系统就能帮我整理出我该做什么。"_

---

## 为什么

传统项目管理工具（Linear / Jira）为了让人类能直接操作，把项目简化成了表单 + 状态 + 看板。简化给人看没有错，**但不能在存储层就把真实结构丢掉**。

真实项目是**多节点**（任务、决策、风险、发布）、**多关系**（依赖、阻塞、归因、拆解）、**持续演化**（认知不断修正）的——本质是一个**动态演化的图**。

GraphMind 的做法：底层保留完整的图结构，由 AI Agent 梳理后再以人类可理解的方式呈现。

---

## 架构

```
人（用户）
  ↕  自然语言对话
AI Agent（Claude Code / Codex / Copilot）
  ↕  结构化命令 + JSON
GraphMind CLI（gm）
  ↕  读写
Graph（SQLite）
```

| 层 | 职责 |
|---|---|
| **人** | 提供上下文、做决策、确认 proposal |
| **AI Agent** | 对话、追问、结构化、生成 proposal、调用 CLI |
| **GraphMind CLI** | 图的读写接口——结构化输入输出、行为可预测、机器可解析 |
| **Graph** | 以图结构存储项目的完整真实关系 |

---

## 工作流

以一个实际场景为例：

**① 用户描述上下文**

```
用户: 我们决定把支付模块从单体里拆出来做微服务，
      张三负责接口设计，李四负责数据迁移，
      预计下周开始，依赖用户服务的认证接口先稳定下来。
```

**② AI Agent 追问，挖掘细节**

```
AI: 拆分的目标版本是什么？有没有明确的截止时间？
AI: "依赖认证接口稳定"——稳定的标准是什么？谁来判定？
```

**③ AI Agent 调用 CLI 查图，关联已有信息**

```bash
$ gm query --related "认证接口" --format json
```

```
AI: 图中已有"认证接口重构"（#42），进行中，负责人王五。
    你说的依赖是指这个任务吗？
```

**④ AI Agent 生成 Proposal，调用 CLI 写入**

```bash
$ gm proposal create --format json << 'EOF'
{
  "nodes": [
    {"title": "支付模块微服务化", "type": "epic"},
    {"title": "支付接口设计", "type": "task", "assignee": "张三"},
    {"title": "支付数据迁移", "type": "task", "assignee": "李四"}
  ],
  "edges": [
    {"from": "支付模块微服务化", "to": "支付接口设计", "type": "decompose"},
    {"from": "支付模块微服务化", "to": "支付数据迁移", "type": "decompose"},
    {"from": "支付数据迁移", "to": "#42", "type": "depends_on"}
  ]
}
EOF
```

AI Agent 将结果翻译为人话：

```
AI: 计划创建以下结构：
    📦 支付模块微服务化 (epic)
     ├── 支付接口设计 (张三)
     └── 支付数据迁移 (李四) → 依赖 #42 认证接口重构
    确认写入吗？
```

**⑤ 用户确认，Commit**

```bash
$ gm proposal commit <proposal-id>
```

**⑥ 持续演化**

```
用户: 张三说接口设计完成了，但发现需要一个新的网关层。
```

AI Agent 再次调用 CLI 更新状态、创建新节点、调整关系——生成新的 proposal，循环往复。

---

## 设计原则

| 原则 | 含义 |
|---|---|
| **Graph-first** | 项目数据以图结构存储，不提前压缩为表单、列表或看板 |
| **Proposal-first** | 所有变更先生成 proposal，用户确认后才写入，防止错误建模污染系统 |
| **Event-sourced** | 所有变更以事件记录，当前状态由事件投影得到，支持回溯与演化分析 |
| **Evolving Graph** | 图持续演化——支持补充、修正、拆分、合并、重归类 |
| **CLI-as-Tool** | CLI 是 AI Agent 的工具接口，不是人的操作界面。结构化 I/O，行为可预测 |
| **Local-first** | 默认本地运行（SQLite），零配置，单用户优先 |

---

## AI 的角色

AI Agent 不负责决策，负责处理人无法直观驾驭的复杂图关系：

- **结构化** — 从自然语言中提取节点和关系
- **关联** — 将新信息与已有图节点建立连接
- **校验** — 检查图的一致性（循环依赖、缺失关系等）
- **投影** — 将复杂图关系整理为人可理解的视图
- **压缩** — 对大规模图信息做摘要

> AI 是"复杂性整流器"，不是"项目管理者"。

---

## 非目标

当前阶段不追求：

- 企业级权限系统
- 复杂审批流程
- Web UI / 重前端体验
- 完整替代 Linear / Jira
- 通用图数据库

---

## 技术栈

| | |
|---|---|
| 语言 | Go |
| 存储 | SQLite |
| 接口 | CLI（JSON I/O） |

---

## License

[MIT](LICENSE)
