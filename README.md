# GraphMind

**为 AI Agent 原生打造的图式项目管理工具。**

GraphMind 是一个 local-first 的项目管理 CLI，专为 Claude Code、Codex、Copilot 等 AI Agent 设计。它以图（graph）为底层数据结构，让 AI Agent 能够将用户的自然语言上下文转化为结构化的任务、关系和洞察。

人不直接操作 GraphMind。人和 AI Agent 对话，AI Agent 调用 GraphMind 读写图。

---

## 问题本质

传统项目管理工具（Linear / Jira 等）采用**表单驱动 + 状态驱动 + 视图驱动**的方式：

- Issue + Fields → 结构化输入
- Todo / Doing / Done → 状态流转
- Board / List → 视图呈现

核心问题在于：

> **为了让人类更容易理解，过早简化了项目的真实结构，导致信息丢失。**

简化给人看很重要，但**不能以丢弃事物本质为代价**。正确的做法是：底层保留完整的图结构，由 AI 梳理后再以人类可理解的方式呈现。

而真实世界的项目是：

- **多节点** — 任务、决策、风险、发布、讨论
- **多关系** — 依赖、阻塞、归因、拆解、影响
- **持续演化** — 认知不断修正

本质上，它是一个**动态演化的图（graph）**。

---

## 核心目标

> 构建一个以"图"为底层真实结构、以"AI"为建模引擎、以"CLI"为操作入口的项目管理系统。

用户的核心体验：

> _"我只需要描述发生了什么，系统就能帮我整理出我该做什么。"_

---

## 架构分层

```
人（用户）
  ↕ 自然语言对话
AI Agent（Claude Code / Codex）
  ↕ 结构化命令
GraphMind CLI
  ↕ 读写
Graph（SQLite）
```

- **人不直接接触 CLI**。人和 AI Agent 对话。
- **AI Agent 调用 CLI** 来读写图。CLI 是 AI Agent 的工具接口。
- **CLI 不需要"人类友好"**，需要 **"AI Agent 友好"** — 结构化输入输出、可预测的行为、机器可解析的结果。

---

## 核心工作流

### 1. 用户和 AI Agent 对话 — 提供上下文

用户在 Claude Code / Codex 中用自然语言描述正在发生的事情：

```
用户: 我们决定把支付模块从单体里拆出来做微服务，
      张三负责接口设计，李四负责数据迁移，
      预计下周开始，依赖用户服务的认证接口先稳定下来。
```

### 2. AI Agent 追问 — 挖掘关键细节

AI Agent 在对话中主动提问，补全结构化所需的关键信息：

```
AI: 拆分的目标版本是什么？有没有明确的截止时间？
AI: "依赖认证接口稳定"——这个稳定的标准是什么？谁来判定？
AI: 数据迁移有回滚方案吗？
```

### 3. AI Agent 查图 — 调用 CLI 获取上下文

AI Agent 调用 GraphMind CLI 查询已有图关系，结合结果与用户确认：

```bash
# AI Agent 在后台调用 CLI
$ gm query --related "认证接口" --format json
```
```
AI: 我发现图中已有"用户服务认证接口重构"（节点 #42），
    当前状态是进行中，负责人王五。
    你说的"依赖认证接口稳定"是指依赖这个任务完成吗？
```

### 4. AI Agent 生成 Proposal — 调用 CLI 写入

经过几轮对话，AI Agent 调用 CLI 创建 proposal：

```bash
# AI Agent 调用 CLI 提交 proposal
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

AI Agent 将 proposal 以人类可读的方式呈现给用户：

```
AI: 我整理了一下，计划创建以下结构：
    📦 支付模块微服务化 (epic)
     ├── 支付接口设计 (张三)
     └── 支付数据迁移 (李四) → 依赖 #42 认证接口重构
    要确认写入吗？
```

### 5. 用户确认 — Commit

用户同意后，AI Agent 调用 CLI 提交 proposal：

```bash
$ gm proposal commit <proposal-id>
```

### 6. 持续演化 — 图随信息更新

用户随时补充新信息，AI Agent 重复上述流程更新图：

```
用户: 张三说接口设计完成了，但发现需要一个新的网关层，
      这个之前没有考虑到。
```

AI Agent 调用 CLI 更新状态、创建新节点、调整关系，再次生成 proposal 供确认。

---

## 设计原则

### Graph-first — 保留真实结构

项目数据以图结构存储，不以表单为核心建模方式，不提前压缩为列表或看板。

### AI-driven Modeling — 对话式建模

- 用户提供自然语言上下文
- AI 通过提问补全关键信息
- AI 生成结构化 proposal（节点 / 关系 / tag）
- 用户确认后写入系统

> 用户提供语义，AI 负责结构化。

### Evolving Graph — 渐进式认知

图不是一次构建完成。新信息会持续进入，AI 支持：

- **补充**（enrich）
- **修正**（correct）
- **拆分**（split）
- **合并**（merge）
- **重归类**（reclassify）

> 图是不断被修正的认知结构。

### Event-sourced — 事件驱动

- 所有变更以事件形式记录
- 当前状态由事件投影（projection）得到
- 支持回溯和认知演化分析

### Proposal-first — 写入模型

- AI 不直接写入正式数据
- 所有变更先生成 proposal
- 用户确认后 commit

> 防止错误建模污染系统。

### CLI-as-Tool — AI Agent 的工具接口

- CLI 是给 AI Agent（Claude Code / Codex）调用的，不是给人直接操作的
- 输入输出结构化（JSON），机器可解析
- 命令语义明确、行为可预测
- 人通过 AI Agent 间接使用 CLI

### Local-first — 部署模型

- 默认本地运行（SQLite）
- 零配置、开箱即用
- 单用户优先，后续再扩展协作能力

---

## AI 的角色定位

AI 不负责决策，而负责：

| 职责 | 说明 |
|------|------|
| 结构化 | extract structure |
| 关联 | link context |
| 校验 | validate graph |
| 投影 | generate views |
| 压缩 | summarize complexity |

> AI 是"复杂性整流器"，不是"项目管理者"。

---

## 非目标（Non-goals）

当前阶段**不追求**：

- 完整企业级权限系统
- 复杂审批流程
- 重 UI / Web-first 体验
- 完整替代 Linear / Jira
- 通用图数据库产品

---

## 技术栈

- **语言**：Go
- **存储**：SQLite（local-first）
- **交互**：CLI

---

## License

[MIT](LICENSE)
