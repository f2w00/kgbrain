# kgc — 知识图谱补全（已废弃）

> 被 `enrichextract` 取代。

## 概述

kgc（Knowledge Graph Construction）提供两类 LLM 驱动的数据补全能力：

- **kgc.enrich**：定义明确的补全 tasks，每 task 指定源字段与目标字段，支持文本推断和图片描述两种模式
- **kgc.autofill**：只传入目标结构示例，LLM 自主推断源字段到目标字段的映射关系，返回完整目标结构

两者均为同步接口，单次 LLM 调用完成一批数据（≤100 行）的处理。

## RPC 方法

| 方法 | 请求 | 响应 |
|------|------|------|
| `kgc.enrich` | `{ profile_id, data[], examples[], tasks[], max_image_kb? }` | `{ data[], enriched_count }` |
| `kgc.autofill` | `{ profile_id, data[], targets_example[] }` | `{ data[] }` |

## 代码结构

| 文件 | 职责 |
|------|------|
| `domain/kgc/kgc.go` | 领域模型：Request、Result、TaskDef |
| `domain/kgc/service.go` | enrich 服务入口，组装 prompt → 调 LLM → 解析输出 → 回填 |
| `domain/kgc/autofill.go` | autofill 服务入口 |
| `domain/kgc/autofill_service.go` | autofill core：自动推断映射 → 转换数据 |
| `domain/kgc/autofill_prompt.go` | autofill 的 prompt 模板 |
| `domain/kgc/llm.go` | LLMClient 接口定义 |
| `domain/kgc/prompt.go` | enrich 的 prompt 构造，含 task 分组 + 示例注入 |
| `application/kgc/service.go` | 应用层：读取 profile → 创建 LLM 客户端 → 调 domain |
| `delivery/rpc/handlers/kgc.go` | JSON-RPC 方法注册与参数解析 |
