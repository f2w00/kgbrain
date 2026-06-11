# kgbrain — Knowledge Graph Brain

从原始数据到知识图谱的一站式 LLM 处理管道。

## 处理流程

```
源表 (JSONB 原始数据)
  │
  ▼
EnrichExtract  ─── LLM 抽取/补全为标准字段
  │                 入: source_table.raw_data (JSONB)
  │                 出: output_table (结构化列)
  ▼
EntityAlignment ── LLM 实体对齐/标准化
  │                 入: output_table 的源字段值
  │                 出: 映射到标准目标值列表
  ▼
结构化、对齐后的数据 → 知识图谱导入
```

## 服务

| 服务 | 协议 | 职责 |
|------|------|------|
| Resource | Connect RPC | 管理 LLM API / PostgreSQL 连接配置 |
| EnrichExtract | Connect RPC | JSONB → 结构化字段的 LLM 抽取与补全 |
| Entity Alignment | Connect RPC | 字段值对齐到标准目标值列表 |

## 快速开始

### 构建

```bash
go build -o kgbrain ./cmd/server
```

### 运行

```bash
./kgbrain --config configs/config.toml
```

### 调用示例 (grpcurl)

```bash
# 1. 配置 LLM 资源
grpcurl -plaintext -d '{
  "resource_id": "qwen_local",
  "name": "本地 Qwen",
  "config": {"base_url": "http://localhost:11434/v1", "model": "qwen3"}
}' localhost:8848 kgbrain.v1.ResourceService/SetLLMResource

# 2. 配置 PostgreSQL 资源
grpcurl -plaintext -d '{
  "resource_id": "my_pg",
  "name": "业务库",
  "config": {"type": "DATABASE_TYPE_POSTGRES", "postgres": {"host": "localhost", "port": 5432, "database": "museum", "user": "app", "password": "secret", "sslmode": "disable"}}
}' localhost:8848 kgbrain.v1.ResourceService/SetDatabaseResource

# 3. 启动结构化抽取
grpcurl -plaintext -d '{
  "llm_resource_id": "qwen_local",
  "database_resource_id": "my_pg",
  "source_table": "artifact_raw",
  "output_table": "artifact_structured",
  "key_field": "id",
  "output_schema": [
    {"name": "name", "type": "ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_TEXT"},
    {"name": "dynasty", "type": "ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_TEXT"}
  ],
  "target_example": [{"name": "青花瓷盘", "dynasty": "明代"}],
  "auto_create_output_table": true,
  "priority_field_hints": {
    "dynasty": "朝代信息。优先从名称、标题、描述中提取，例如明代、清代。"
  }
}' localhost:8848 kgbrain.v1.EnrichExtractService/StartEnrichExtract
```

## 技术栈

| 组件 | 选型 |
|------|------|
| 协议 | Connect RPC (gRPC + Connect + gRPC-Web) |
| LLM | cloudwego/eino (OpenAI 兼容) |
| 本地存储 | SQLite (modernc.org/sqlite) |
| 业务数据库 | PostgreSQL |
| 日志 | go.uber.org/zap |

## 文档

| 路径 | 内容 |
|------|------|
| `docs/enrichextract/` | 结构化抽取设计 |
| `docs/entity-alignment/` | 实体对齐设计 |
| `docs/resource/` | 资源服务设计 |
| `docs/deprecated/` | 已废弃模块（kgc / mapping / profile / xform） |

## 测试

```bash
go test ./internal/... -count=1
```
