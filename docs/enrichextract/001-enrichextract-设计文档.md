# EnrichExtract - 结构化抽取与轻量补全服务

> 创建日期: 2026-06-10
> 状态: 设计阶段
> 作者: AI

## 1. 概述

`enrichextract` 用于从业务数据库中的 `source_table` 读取原始数据行，
调用 LLM 生成标准化后的目标字段，并将结果写入调用者指定的
`output_table`。

该模块主要解决以下问题：

- 将原始表中的非标准字段、半结构化描述或缺失信息转为标准化目标字段。
- 复用现有 `xform` 的目标示例、逐行 LLM 和输出字段对齐思路。
- 使用 ConnectRPC 和 feature-first 代码结构实现新模块。
- 面向大数据量处理，数据库侧分页读取、批量写入。

典型数据流：

```text
source_table -> 逐行 LLM 抽取/补全 -> output_table
```

## 2. 设计边界

第一阶段只处理以下范围：

- 使用 ConnectRPC 暴露服务接口。
- 按 feature-first 方式组织代码，模块名为 `enrichextract`。
- 通过 `llm_resource_id` 选择 LLM Resource。
- 通过 `database_resource_id` 选择业务数据库连接。
- 从一个 `source_table` 读取数据，source 表使用 JSONB 字段保存原始 payload。
- 将结果写入一个调用者指定的 `output_table`。
- 输出字段结构由必填的 `output_schema` 显式定义，不从 `target_example` 推导。
- `auto_create_output_table=true` 时，服务可在 `output_table` 不存在时按
  `output_schema` 自动建表；已存在表只校验，不自动改结构。
- `output_table` 仅写入 `key_field` 和 `output_schema` 定义的目标字段。
- `source_table` 中的 JSONB 原始 payload 只作为 LLM 输入，不直接复制到 output 表。
- 不写入 `ext_info` 或其他系统元数据字段。
- `key_field` 仅支持整数类型，并支持 `start_id` / `end_id` 范围处理。
- LLM 按行调用，避免批量输出顺序变化或主键被模型篡改导致错配。
- 数据库按页读取 source rows，并按页批量写入 output rows。
- page 内使用有限 worker pool 并发调用 LLM，不跨 page 复用 worker pool。
- 单行 LLM 调用支持有限重试，重试耗尽后记录行级错误并继续处理。
- 成功中间结果不持久化，只在当前 page 内存中暂存。
- Job 参数快照、执行进度、统计状态和行级失败记录写入本地 SQLite。
- 服务重启后恢复 `pending` / `running` job，从 `last_key` 后继续处理。
- 不使用 Redis Stream，不沿用 `xform` 的 append/result-list 消费模型。
- 不支持枚举约束、人工审核、任务暂停、任务取消和恢复中的间态结果回放。

## 3. 与现有模块的关系

### 3.1 与 xform 的关系

复用思路：

- `target_example` 表达 LLM 输出示例。
- `output_schema` 显式定义目标字段集合和字段类型。
- LLM 逐行处理。
- LLM 输出必须对齐到目标字段集合。
- 主键不交给 LLM 决定，服务端强制注入原始主键。

不复用的点：

- 不使用 JSON-RPC。
- 不使用 Redis Stream。
- 不支持 submit/append/close/get_result 的流式输入输出模型。
- 不通过 profile 读取旧 LLM 配置，改用 Resource 模块。

### 3.2 与 entity alignment 的关系

参考 `alignment` 的工程结构：

- `module.go` 作为 feature 装配入口。
- `service.go` 负责 job 创建、状态查询和后台执行调度。
- `executor.go` 串联资源、LLM 和业务数据库连接。
- `business_repository.go` 负责业务库表结构校验、分页读取和批量写入。
- `handler.go` 负责 ConnectRPC 请求转换和错误映射。

不复用 `alignment` 的 mapping 逻辑：

- 不做 distinct raw value 归一化。
- 不维护 mapping table。
- 不执行 mapping join 写出。
- 不复用字段级 targets 列表。

## 4. ConnectRPC API 草案

服务名：

```proto
service EnrichExtractService {
  rpc StartEnrichExtract(StartEnrichExtractRequest)
      returns (StartEnrichExtractResponse);
  rpc GetEnrichExtractJob(GetEnrichExtractJobRequest)
      returns (GetEnrichExtractJobResponse);
}
```

启动请求：

```proto
message StartEnrichExtractRequest {
  string llm_resource_id = 1;
  string database_resource_id = 2;

  string source_table = 3;
  string output_table = 4;
  string key_field = 5;

  repeated EnrichExtractOutputColumn output_schema = 6;
  google.protobuf.Struct target_example = 7;

  optional int64 start_id = 8;
  optional int64 end_id = 9;

  optional int32 concurrency = 10;
  optional bool overwrite = 11;
  optional int32 page_size = 12;
  optional int32 max_retries = 13;
  optional string source_json_field = 14;
  map<string, string> priority_field_hints = 15;
  optional bool auto_create_output_table = 16;
  optional int32 llm_timeout_seconds = 17;
}

message EnrichExtractOutputColumn {
  string name = 1;
  EnrichExtractOutputColumnType type = 2;
}

enum EnrichExtractOutputColumnType {
  ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_UNSPECIFIED = 0;
  ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_TEXT = 1;
  ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_BIGINT = 2;
}
```

字段说明：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `llm_resource_id` | string | 是 | 用于读取 LLM Resource 配置 |
| `database_resource_id` | string | 是 | 用于读取 Database Resource 配置 |
| `source_table` | string | 是 | 原始数据来源表，支持 `schema.table` |
| `output_table` | string | 是 | 结果写入表，支持 `schema.table` |
| `key_field` | string | 是 | 整数主键字段，用于分页、去重和 upsert |
| `output_schema` | EnrichExtractOutputColumn[] | 是 | 输出字段结构，是校验、建表、写库和 prompt 字段列表的唯一来源 |
| `target_example` | Struct | 是 | LLM 输出结构示例，字段集合必须与 `output_schema` 一致 |
| `start_id` | int64 | 否 | 基于 `key_field` 的处理范围下界 |
| `end_id` | int64 | 否 | 基于 `key_field` 的处理范围上界 |
| `concurrency` | int32 | 否 | LLM 逐行调用并发数，默认保守值 |
| `overwrite` | bool | 否 | 是否覆盖已有 output 行，默认 false |
| `page_size` | int32 | 否 | 数据库分页读取大小，默认保守值 |
| `max_retries` | int32 | 否 | 单行 LLM 最大重试次数，默认保守值 |
| `source_json_field` | string | 否 | source 表中保存原始 payload 的 JSONB 字段，默认 `raw_data` |
| `priority_field_hints` | map<string, string> | 否 | 重点字段说明，key 必须是目标字段名，value 是该字段的抽取/推断说明 |
| `auto_create_output_table` | bool | 否 | 输出表不存在时是否按 `output_schema` 自动建表，默认 false |
| `llm_timeout_seconds` | int32 | 否 | 单行单次 LLM 调用超时（秒），默认 30 秒 |

请求示例：

```json
{
  "llm_resource_id": "qwen_local",
  "database_resource_id": "museum_pg",
  "source_table": "public.artifact_raw",
  "output_table": "public.artifact_extract",
  "key_field": "id",
  "output_schema": [
    {"name": "standard_name", "type": "ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_TEXT"},
    {"name": "dynasty", "type": "ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_TEXT"},
    {"name": "material", "type": "ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_TEXT"}
  ],
  "target_example": {
    "standard_name": "青花瓷盘",
    "dynasty": "明代",
    "material": "瓷"
  },
  "start_id": 1000,
  "end_id": 2000,
  "concurrency": 2,
  "overwrite": false,
  "page_size": 100,
  "max_retries": 2,
  "llm_timeout_seconds": 30,
  "source_json_field": "raw_data",
  "auto_create_output_table": true,
  "priority_field_hints": {
    "dynasty": "朝代信息。优先从名称、标题、描述、年代、分类等字段中提取，例如“明代青花瓷盘”应提取为“明代”。",
    "material": "材质信息。优先从名称、描述、工艺、材质字段中提取，例如瓷、铜、玉、纸本等。"
  }
}
```

`output_schema` 处理规则：

- `output_schema` 必填且不能为空。
- `output_schema.name` 不能为空、不能重复、不能等于 `key_field`。
- `output_schema.type` 不能为 `UNSPECIFIED`；初版仅支持 `TEXT` 和 `BIGINT`。
- 服务按字段名排序后作为 prompt 目标字段、LLM 输出裁剪和写库字段列表。

`target_example` 处理规则：

- `target_example` 必填，用于展示 LLM 输出格式，不参与数据库 DDL 推断。
- `target_example` 的字段集合必须与 `output_schema.name` 完全一致。
- 示例不应包含 `key_field`；如果 LLM 输出中包含 `key_field`，服务仍会删除。

`priority_field_hints` 处理规则：

- `key` 必须属于 `output_schema` 定义的目标字段。
- `key` 不允许等于 `key_field`。
- `value` 不能为空字符串。
- 该字段只影响 prompt，不新增输出列，也不强制非空。

`llm_timeout_seconds` 处理规则：

- 控制单行单次 LLM 调用超时，默认值为 30 秒。
- 每次 retry 会重新应用该超时。
- 允许范围为 `1 ~ 600` 秒。

## 5. 输入与输出表规则

### 5.1 source_table 规则

`source_table` 必须由调用者提前创建，用于存放原始输入数据。

第一阶段要求 source 表至少包含两类字段：

- `key_field`：整数主键字段，用于分页、join/filter、恢复进度和 output upsert。
- `source_json_field`：JSONB 字段，用于保存原始 payload，默认字段名为 `raw_data`。

示例表结构：

```sql
CREATE TABLE artifact_raw (
  id BIGINT PRIMARY KEY,
  raw_data JSONB NOT NULL
);
```

source 表校验规则：

- `source_table` 存在。
- `source_table.key_field` 存在且是整数类型。
- `source_table.key_field` 存在 primary key 或 unique 约束。
- `source_table.source_json_field` 存在且类型为 `jsonb`。

行级规则：

- 每行 `source_json_field` 的值必须是 JSON object。
- 如果某行值是 array/string/number/bool/null，记录 `validate_source` 行级错误。
- source JSON object 会被解析为 `map[string]any` 后用于构造 LLM 输入。

### 5.2 raw payload 与主键规则

`source_table.key_field` 是唯一可信主键。

如果 `source_json_field` 的 JSON object 中存在与 `key_field` 同名的字段，构造
LLM prompt 前必须剥离该字段。

示例：

```text
source_table.id = 123
source_table.raw_data = {"id": 999, "title": "明代青花瓷盘"}
```

处理规则：

- LLM 输入中删除 `raw_data.id`。
- 不比较 `raw_data.id` 与 `source_table.id` 是否一致。
- LLM 输出中的 `id` 一律删除。
- `output_table.id` 一律使用 `source_table.id = 123`。

如果 payload 中的原始 ID 有业务意义，调用者应使用其他字段名，例如
`external_id`，避免与 `key_field` 同名。

### 5.3 output_table 规则

`output_table` 默认必须由调用者提前创建；如果 `auto_create_output_table=true`，
且输出表不存在，服务会按 `key_field` 和 `output_schema` 自动创建。

服务启动 job 前执行以下校验：

- `output_table` 存在；或 `auto_create_output_table=true` 且有权限创建。
- `output_table.key_field` 存在且是整数类型。
- `output_table.key_field` 存在 primary key 或 unique 约束。
- `output_schema` 中的列在 `output_table` 中全部存在。
- `output_schema` 中的列类型与定义一致。
- `output_schema` 不允许包含 `key_field`。
- `target_example` 不能为空。
- `start_id <= end_id`，如果二者都传入。

示例表结构：

```sql
CREATE TABLE artifact_extract (
  id BIGINT PRIMARY KEY,
  standard_name TEXT,
  dynasty TEXT,
  material TEXT
);
```

自动建表时生成的结构：

```sql
CREATE TABLE IF NOT EXISTS artifact_extract (
  id BIGINT PRIMARY KEY,
  standard_name TEXT,
  dynasty TEXT,
  material TEXT
);
```

服务不会：

- 添加缺失列。
- 修改字段类型。
- 添加索引或约束。
- 写入 `ext_info`。
- 复制 source JSONB payload 到 output 表。
- 复制 source 表的非目标字段。

## 6. overwrite 语义

`overwrite` 用于控制已存在 output 行的处理方式。

### 6.1 overwrite=false

默认行为。

处理前过滤：

```text
读取 source rows 时排除 output_table 中已存在 key 的行，避免浪费 LLM。
```

写入时兜底：

```sql
INSERT INTO output_table (...)
VALUES (...)
ON CONFLICT (key_field) DO NOTHING;
```

该模式支持断点续跑和重复提交。

### 6.2 overwrite=true

处理前不过滤已存在 output 行。

写入时执行 upsert 更新：

```sql
INSERT INTO output_table (...)
VALUES (...)
ON CONFLICT (key_field) DO UPDATE SET
  target_field_1 = EXCLUDED.target_field_1,
  target_field_2 = EXCLUDED.target_field_2;
```

## 7. 执行流程

```text
1. ConnectRPC 接收 StartEnrichExtract 请求。
2. 应用服务校验请求参数并创建 job。
3. 后台 goroutine 标记 job 为 running。
4. executor 根据 resource_id 获取 LLM 客户端和业务数据库连接。
5. business repository 校验 source_table / output_table / key_field / source_json_field / output_schema。
6. 领域执行服务按 key_field 做 keyset pagination。
7. overwrite=false 时，分页查询阶段排除 output_table 已存在 key 的行。
8. 对当前 page 的每一行单独调用 LLM，可按 concurrency 并发。
9. 解析 source JSONB，校验其值为 JSON object，并剥离同名 `key_field`。
10. 解析和校验 LLM 输出。
11. 当前 page 内收集成功结果。
12. 将成功结果批量写入 output_table 普通列。
13. 如果批量写入失败，降级为逐行写入以定位坏行。
14. 单行失败写入本地 SQLite job error 表，继续处理其他行。
15. 当前 page 完成后更新统计信息和 `last_key`。
16. 全部 page 处理完成后，根据成功/失败数量标记 job 状态。
```

数据库读写方式：

```text
DB: page read
LLM: per row
DB: batch write
```

避免以下方式：

```text
读一行 -> LLM -> 写一行
```

## 8. 分页策略

`key_field` 仅支持整数，因此使用 keyset pagination。

分页规则：

- 按 `key_field` 升序读取。
- 不使用 `OFFSET`。
- 每页读取 `key_field > last_key` 的下一批数据。
- 如果传入 `start_id`，初始下界为 `start_id`。
- 如果传入 `end_id`，查询不超过 `end_id`。
- 每页处理完成后，`last_key` 更新为当前页最大 key。
- `last_key` 按读取到的 source rows 推进，不按成功结果推进。
- 单行失败不会阻塞 `last_key` 推进，避免坏行导致 job 卡住。

示意 SQL：

```sql
SELECT s.id, s.raw_data
FROM source_table s
WHERE s.id > $1
  AND s.id <= $2
ORDER BY s.id
LIMIT $3;
```

`overwrite=false` 时加入 output 过滤：

```sql
SELECT s.id, s.raw_data
FROM source_table s
LEFT JOIN output_table o ON o.id = s.id
WHERE o.id IS NULL
  AND s.id > $1
  AND s.id <= $2
ORDER BY s.id
LIMIT $3;
```

实际 SQL 中的 `id` 和 `raw_data` 分别由 `key_field` 和 `source_json_field`
配置决定，并需要做 SQL identifier quote。

`page_size` 默认值建议根据 `concurrency` 动态计算：

```text
page_size = clamp(concurrency * 10, 100, 1000)
```

如果 `source_json_field` 单行 JSON 较大，调用者应调低 `page_size`，例如
`100` 或 `200`。

## 9. 并发模型

`enrichextract` 采用 page-scoped worker pool。

并发只在单个 page 内生效，不跨 page 复用 worker pool：

```text
1. 读取一个 page。
2. 创建最多 concurrency 个临时 worker。
3. worker 逐行调用 LLM 并返回 RowResult。
4. 当前 page 所有 row 完成后，批量写入 output_table。
5. 释放 worker，推进 last_key，进入下一页。
```

不采用长期全局 worker pool：

```text
reader -> rowQueue -> workerPool -> resultQueue -> writer
```

原因：

- page 是天然恢复边界，崩溃后最多重做当前 page。
- 内存上限明确，约等于 `page_size` 级别。
- 批量写入边界清晰，不需要额外 flush 策略。
- 不需要协调 reader、worker 和 writer 的长期生命周期。
- goroutine 创建成本远小于 LLM 网络调用耗时。

每个 `RowResult` 必须携带 source key：

```text
source_key
values
error
```

即使 page 内并发导致返回顺序变化，也以原始 source key 注入
`key_field` 并写入 output_table，不依赖 LLM 返回顺序。

并发控制分两层：

| 层级 | 来源 | 作用 |
|------|------|------|
| job 级 | `StartEnrichExtractRequest.concurrency` | 限制当前 job 单个 page 内同时处理多少行 |
| resource 级 | `llm_resource.max_concurrency` | 限制同一 LLM resource 的全局并发请求数 |

实际 LLM 并发满足：

```text
实际并发 <= min(job.concurrency, llm_resource.max_concurrency)
```

如果 `llm_resource.max_concurrency` 未配置，则仅受 job 级 `concurrency` 限制。

## 10. LLM Prompt、输出校验与重试

### 10.1 Prompt 输入

系统消息需要表达：

- 角色是结构化抽取与轻量补全助手。
- 只输出 JSON object。
- 只输出目标字段。
- 不输出源字段。
- 不输出主键字段。
- 无法判断的目标字段输出 JSON null。
- 所有目标字段都必须输出。

目标字段来自 `output_schema`，prompt 同时展示字段类型。`target_example` 只作为
输出格式示例，不参与目标字段推导。

用户消息中的源数据来自 `source_json_field` 对应 JSONB 值。构造 prompt 前，
如果源 JSON object 中存在与 `key_field` 同名的字段，服务会先剥离该字段，
避免 LLM 看到不可信主键。

`priority_field_hints` 可用于表达重点字段说明。不要通过 `target_example` 中的
特殊值约定表达重点字段，避免污染业务示例。

### 10.2 LLM 输出校验

LLM 单行返回后执行以下校验和归一：

```text
1. 从 LLM 响应中提取 JSON。
2. JSON 必须是 object，不接受 array/string/number。
3. 删除 key_field，LLM 返回的主键一律不可信。
4. 只保留 output_schema 中的字段。
5. output_schema 中缺失的字段补 null。
6. 删除所有非目标字段。
7. 按 output_schema 归一化字段类型，TEXT 转字符串，BIGINT 转 int64。
8. 注入 source row 的原始 key_field。
9. 写入 output_table。
```

字段类型会先在 Go 侧按 `output_schema` 校验和归一化，再由 `output_table` 的
数据库列类型兜底校验。如果写库失败，该行记录为失败，不中断整个 job。

### 10.3 source JSON 校验

从数据库读出的 `source_json_field` 按 JSON 处理。

建议 Go 侧读取和解析方式：

```text
数据库类型: jsonb
Go 扫描类型: []byte 或 json.RawMessage
业务处理类型: map[string]any
LLM 输入: JSON object 字符串
```

行级校验规则：

```text
1. JSONB 字段必须能解析为 JSON。
2. 解析结果必须是 JSON object。
3. 如果 object 中存在 key_field 同名字段，构造 prompt 前删除。
4. 不校验 raw payload 中的同名字段是否等于 source_table.key_field。
```

校验失败记录为：

```text
stage = validate_source
```

### 10.4 单行重试

重试粒度是单行，不是 page。

每个 worker 处理一行时，按 `max_retries` 执行有限重试：

```text
1. 第一次立即调用 LLM。
2. 如果失败且错误可重试，按指数退避等待后重试。
3. 重试成功则返回成功 RowResult。
4. 重试耗尽后返回 RowError，并继续处理 page 内其他行。
```

`max_retries` 表示额外重试次数：

```text
max_retries = 0 -> 总共尝试 1 次
max_retries = 2 -> 总共尝试 3 次
```

建议默认值：

```text
max_retries = 2
```

建议限制范围：

```text
0 <= max_retries <= 5
```

可重试错误：

- LLM 请求超时。
- 网络错误。
- HTTP 429 / rate limit。
- HTTP 5xx。
- 模型返回空内容。
- 模型返回非 JSON。
- JSON 解析失败。
- JSON 不是 object。

不可重试错误：

- 请求参数校验失败。
- Resource 不存在。
- 表结构校验失败。
- `key_field` 非整数。
- `output_schema` 缺失、字段重复或类型不支持。
- `target_example` 为空。
- `output_table` 缺目标字段。
- context 已取消。

退避策略：

```text
attempt 0: 立即执行
attempt 1: sleep 1s
attempt 2: sleep 2s
attempt 3: sleep 4s
最大 sleep: 8s
```

重试等待期间占用当前 job 的 worker slot，但不占用 LLM resource slot；
只有真实发起 LLM 请求时才占用 LLM resource slot。

### 10.5 写入失败处理

正常路径按 page 批量写入 output_table。

如果批量写入失败，执行降级策略：

```text
1. 批量写失败后可做一次短暂等待后的整批重试。
2. 如果仍失败，降级为逐行写入。
3. 逐行写成功的保留。
4. 逐行写失败的记录 `write_output` 行级错误。
```

该策略只在异常路径使用，正常路径仍是批量写入。

不做 page 级整体重试，避免重复调用已成功行的 LLM。

## 11. 中间结果、Job 存储与恢复

成功中间结果不持久化。

存储位置：

| 数据 | 存储位置 | 说明 |
|------|----------|------|
| source rows | 业务 Postgres `source_table` | 原始输入数据 |
| 当前 page 成功结果 | 进程内存 | 批量写入前临时暂存 |
| 最终结果 | 业务 Postgres `output_table` | 标准化目标字段结果 |
| job 元数据 | 本地 SQLite | job 参数快照、状态、进度和统计信息 |
| 行级失败 | 本地 SQLite | 失败 key、阶段、错误信息 |

不持久化成功中间结果的原因：

- `output_table` 已经是最终落点。
- 避免大数据量导致本地 store 膨胀。
- 避免成功结果与 output 表形成双写一致性问题。
- 避免额外存储业务敏感信息。
- 断点续跑可以通过 `output_table` 已存在 key 判断。

崩溃语义：

```text
如果进程在当前 page 批量写入前崩溃，当前 page 已完成 LLM 的内存结果会丢失。
服务重启恢复时，未写入 output_table 的行会重新处理。
```

### 11.1 Job 参数快照

创建 job 时必须把用户输入参数完整快照到本地 SQLite，否则服务重启后无法恢复
执行上下文。

建议 `enrich_extract_jobs` 字段：

```text
job_id
status

llm_resource_id
database_resource_id
source_table
output_table
key_field
source_json_field
output_schema_json
target_example_json
auto_create_output_table

start_id
end_id
overwrite
concurrency
page_size
max_retries

last_key
total_rows
processed_rows
succeeded_rows
failed_rows

created_at
started_at
updated_at
finished_at
error_message
```

其中：

- `target_example_json` 保存原始目标示例。
- `source_json_field` 保存 source 表原始 payload 字段名，未传时为 `raw_data`。
- `output_schema_json` 保存输出字段结构，便于恢复、校验和查询。
- `auto_create_output_table` 保存是否允许自动创建输出表。
- `last_key` 保存 page 级执行进度。
- `processed_rows` 统计已经尝试处理的 source rows。
- `succeeded_rows` 统计成功写入或确认成功的 rows。
- `failed_rows` 统计行级失败 rows。

### 11.2 服务重启恢复

服务启动时扫描本地 SQLite 中的活跃 job：

```text
status in ('pending', 'running')
```

恢复流程：

```text
1. 读取 job 参数快照。
2. 重新获取 LLM Resource 和 Database Resource。
3. 重新打开业务数据库连接。
4. 重新校验 source_table / output_table / key_field / source_json_field / output_schema。
5. 从 last_key 后继续分页处理。
6. 沿用原 job 的 overwrite / concurrency / page_size / max_retries。
```

终态 job 不自动恢复：

```text
succeeded
partial
failed
```

恢复时如果遇到资源不存在、表不存在、字段缺失等初始化错误，直接标记 job 为
`failed`。

### 11.3 恢复一致性语义

执行进度以 page 为边界持久化。

`last_key` 更新时机：

```text
当前 page 的 LLM 处理、批量写入、错误记录和统计更新全部完成后，
last_key = 当前 page 最大 key。
```

如果服务在 page 中间崩溃：

- `last_key` 尚未推进。
- 重启后会重新读取当前 page。
- `overwrite=false` 时，已写入 `output_table` 的行会被 join/filter 跳过。
- 写入时 `ON CONFLICT DO NOTHING` 再次兜底。

如果 `overwrite=true`，当前 page 崩溃后可能被重新处理，并再次生成结果覆盖
已有 output 行。第一阶段接受该 at-least-once page 语义，不额外持久化
每个 key 的成功中间状态。

单行重试耗尽后记录行级错误，并允许 `last_key` 推进。该行在同一个 job 恢复
时不会自动回头处理；用户可用相同参数重新提交新 job，在 `overwrite=false` 下
通过 output_table 过滤自动重试未成功写入的失败行。

## 12. Job 状态与错误记录

建议状态：

```text
pending
running
succeeded
partial
failed
```

状态含义：

| 状态 | 说明 |
|------|------|
| `pending` | job 已创建，后台执行尚未开始 |
| `running` | job 正在执行 |
| `succeeded` | 所有候选行均处理成功 |
| `partial` | 部分行失败，部分行成功；失败行已记录但不会在同一 job 内自动重试 |

`partial` 处理语义：

- `partial` 表示至少一行成功、至少一行失败。
- 单行失败不会阻塞后续分页，`last_key` 仍会在当前 page 完成后推进。
- 当前版本不提供失败行外部查询接口，也不提供只重试失败行的接口。
- 如需重试失败行，可用相同参数重新提交新 job；`overwrite=false` 时已成功写入
  `output_table` 的行会被过滤，未成功写入的行会再次进入处理。
- 上层编排服务依赖 enrichextract 时，必须显式定义遇到 `partial` 后是否继续下游。
| `failed` | 初始化失败、表校验失败或全部候选行失败 |

行级错误表建议字段：

```text
id
job_id
source_key
stage
attempts
error_message
created_at
```

`stage` 可选值：

```text
read_source
validate_source
llm_generate
parse_json
validate_output
write_output
```

第一阶段不建议存完整 raw row、完整 prompt 或完整 LLM response，避免数据膨胀
和敏感信息泄露。

## 13. 建议代码结构

```text
internal/enrichextract/
  contracts.go
  models.go
  requests.go
  service.go
  execution_service.go
  executor.go
  business_repository.go
  repository.go
  handler.go
  module.go
  prompt.go
  validator.go
  errors.go
```

职责：

| 文件 | 职责 |
|------|------|
| `contracts.go` | 定义 ResourceReader、LLMClient、Repository、BusinessRepository 等接口 |
| `models.go` | 定义 Job、状态、字段配置和行结果模型 |
| `requests.go` | 定义应用层请求和响应模型 |
| `service.go` | 创建 job、查询 job、调度后台执行 |
| `execution_service.go` | 编排分页读取、逐行 LLM、批量写入 |
| `executor.go` | 串联 resource、LLM factory 和业务数据库连接 |
| `business_repository.go` | 业务库表校验、分页读取、批量写入 |
| `repository.go` | 本地 SQLite job 和错误记录持久化 |
| `handler.go` | ConnectRPC 请求转换和错误映射 |
| `module.go` | feature 内部依赖装配 |
| `prompt.go` | 构造 LLM system/user prompt |
| `validator.go` | 校验并对齐 LLM 输出字段 |
| `errors.go` | validation/not found 等错误分类 |

## 14. 第一阶段不做事项

- 不实现自动补列或自动改列类型。
- 不实现 `ext_info`。
- 不实现字段级枚举约束。
- 不实现人工审核。
- 不实现 Redis Stream。
- 不实现成功中间结果持久化。
- 不实现任务暂停和取消。
- 不实现失败行在同一 job 内自动回头重跑。
- 不复制 source 表非目标字段到 output 表。

## 15. 待确认事项

- 默认 `concurrency` 和 `page_size` 的具体数值。
- 默认 `max_retries` 的具体数值，当前建议为 2。
- 行级错误是否需要提供分页查询 API。
- 是否需要在 job 查询响应中返回 `processed_rows`、`succeeded_rows`、`failed_rows`。
- 后续是否需要扩展 `output_schema` 字段类型，例如 `numeric`、`boolean`、`jsonb`。
