# GraphMind

> A local-first, AI-native project management CLI that models work as an evolving graph—turning raw context into structured tasks, relationships, and insights through conversational modeling.

---

## 问题本质

传统项目管理工具（Linear / Jira 等）采用**表单驱动 + 状态驱动 + 视图驱动**的方式：

- Issue + Fields → 结构化输入
- Todo / Doing / Done → 状态流转
- Board / List → 视图呈现

核心问题在于：

> **为了让人类更容易理解，过早简化了项目的真实结构，导致信息丢失。**

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

### CLI-first — 交互方式

- CLI 作为主要操作入口
- 低摩擦、可脚本化、可嵌入开发流程
- 支持 AI + CLI 混合交互

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
