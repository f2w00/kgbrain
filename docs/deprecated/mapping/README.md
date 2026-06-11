# mapping — 字段映射与内容映射（已废弃）

> 被 `enrichextract` 和 `entity-alignment` 取代。

## 概述

mapping 模块提供两类映射能力：

- **mapping.field**：根据一条示例数据和目标字段示例，让 LLM 推断源字段到目标字段的映射关系。结果按字段组合缓存
- **mapping.content**：按 topic 维护内容映射表（如朝代名标准化），未命中的值自动调 LLM 映射并缓存

## RPC 方法

| 方法 | 请求 | 响应 |
|------|------|------|
| `mapping.field` | `{ profile_id, example, target_fields[], refresh? }` | `{ mapping, unmapped_source[], unfilled_target[], cached }` |
| `mapping.content` | `{ profile_id, topic, values[], targets[]? }` | `{ topic, mapping }` |
| `mapping.content.set` | `{ profile_id, topic, mapping }` | `{ success }` |
| `mapping.content.targets.set` | `{ profile_id, topic, targets[] }` | `{ success }` |
| `mapping.content.targets.get` | `{ profile_id, topic }` | `{ topic, targets[] }` |

## 代码结构

| 文件 | 职责 |
|------|------|
| `domain/mapping/mapping.go` | 领域模型（MappingResult 等） |
| `domain/mapping/service.go` | field mapping 核心：prompt → LLM → 解析 → 缓存 |
| `domain/mapping/prompt.go` | field mapping 的 prompt 模板 |
| `domain/mapping/content.go` | 内容映射模型 |
| `domain/mapping/content_service.go` | 内容映射核心：LLM 映射 + 缓存 |
| `domain/mapping/content_prompt.go` | 内容映射的 prompt 模板 |
| `domain/mapping/repository.go` | 映射缓存存储接口 |
| `application/mapping/service.go` | 应用层：读取 profile → 创建 LLM → 调 domain |
| `infra/repo/mapping.go` | SQLite 缓存存储实现 |
| `delivery/rpc/handlers/mapping.go` | JSON-RPC 方法注册 |
