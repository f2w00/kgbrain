# kgbrain 项目笔记

> 字段映射生成服务 — 通过 LLM 推导源字段到目标字段的映射关系，一次 LLM 调用，无限次程序化应用。

---

## 一、项目概览

### 1.1 核心功能

kgbrain 是一个基于 JSON-RPC 2.0 协议的后端服务，主要提供三大能力：

| 能力 | API | 说明 |
|------|-----|------|
| **Profile 管理** | `profile.set/get/delete` | 管理用户 LLM 配置和通知配置 |
| **字段映射** | `mapping.field` | 一次 LLM 调用推导字段映射关系，缓存复用 |
| **内容映射** | `mapping.content` / `mapping.content.set` / `mapping.content.targets.*` | 值级别的语义映射（如"唐朝"→"唐"） |
| **数据补全** | `kgc.enrich` | 多模态数据补全（文本+图片），一次 LLM 调用完成批量补全 |

### 1.2 技术栈

| 组件 | 选型 |
|------|------|
| 协议 | JSON-RPC 2.0 |
| LLM | cloudwego/eino (OpenAI 兼容) |
| 存储 | SQLite (modernc.org/sqlite) |
| 日志 | go.uber.org/zap |
| 配置 | spf13/viper (TOML) |
| 校验 | santhosh-tekuri/jsonschema (OpenRPC YAML) |
| 压缩 | klauspost/compress (gzip/zstd) |
| CLI | spf13/cobra |

### 1.3 架构风格

**DDD（领域驱动设计）+ 分层架构**

```
┌─────────────────────────────────────────────────────────┐
│                     Delivery 层                          │
│  (internal/delivery/rpc/handlers/)                       │
│  JSON-RPC Handler → 解析请求 → 调用 Application Service  │
├─────────────────────────────────────────────────────────┤
│                    Application 层                        │
│  (internal/application/)                                 │
│  用例编排: Profile 查找 → LLM 配置解析 → 调用 Domain     │
├─────────────────────────────────────────────────────────┤
│                      Domain 层                           │
│  (internal/domain/)                                      │
│  纯业务逻辑: 映射规则、Prompt 构建、值对象、Repository 接口│
├─────────────────────────────────────────────────────────┤
│                   Infrastructure 层                      │
│  (internal/infra/)                                       │
│  技术实现: SQLite、LLM Client、中间件、配置加载            │
└─────────────────────────────────────────────────────────┘
```

**依赖方向**：Delivery → Application → Domain ← Infrastructure

---

## 二、目录结构

```
kgbrain/
├── cmd/server/main.go              # 服务入口
├── configs/config.toml             # 配置文件
├── data/kgbrain.db                 # SQLite 数据库（运行时生成）
│
├── internal/
│   ├── app/run.go                  # 启动入口 + 依赖注入
│   │
│   ├── application/                # 应用层（用例编排）
│   │   ├── profile/service.go      # Profile 用例
│   │   ├── mapping/service.go      # 映射用例（字段+内容）
│   │   └── kgc/service.go          # 数据补全用例
│   │
│   ├── domain/                     # 领域层（纯业务逻辑）
│   │   ├── profile/
│   │   │   ├── profile.go          # Profile 实体 + LLMConfig
│   │   │   └── repository.go       # ProfileRepository 接口
│   │   ├── mapping/
│   │   │   ├── mapping.go          # FieldMapping 值对象
│   │   │   ├── content.go          # ContentMapping 值对象
│   │   │   ├── service.go          # 字段映射领域服务
│   │   │   ├── content_service.go  # 内容映射领域服务
│   │   │   ├── prompt.go           # 字段映射 prompt
│   │   │   ├── content_prompt.go   # 内容映射 prompt
│   │   │   └── repository.go       # CacheRepository + ContentRepository 接口
│   │   └── kgc/
│   │       ├── kgc.go              # Request/Result/TaskDef
│   │       ├── service.go          # EnrichService
│   │       ├── prompt.go           # 多模态 prompt 构建 + 图片压缩
│   │       └── llm.go              # LLMClient 接口
│   │
│   ├── delivery/                   # 接口层
│   │   └── rpc/
│   │       ├── server.go           # JSON-RPC Server
│   │       ├── validator.go        # OpenRPC YAML Schema 校验
│   │       ├── method.go           # MethodHandler 类型
│   │       └── handlers/
│   │           ├── profile.go      # profile.* handler
│   │           ├── mapping.go      # mapping.* handler
│   │           ├── kgc.go          # kgc.enrich handler
│   │           └── system.go       # rpc.discover 服务发现
│   │
│   ├── infra/                      # 基础设施层
│   │   ├── store/db.go             # SQLite 连接管理
│   │   ├── repo/
│   │   │   ├── profile.go          # ProfileRepo (SQLite 实现)
│   │   │   └── mapping.go          # CacheRepo (SQLite 实现)
│   │   ├── llm/openai.go           # OpenAI 兼容 LLM 客户端
│   │   └── middleware/compress.go  # 请求体解压中间件 (gzip/deflate/zstd)
│   │
│   ├── config/
│   │   ├── config.go               # 配置结构体
│   │   └── loader.go               # Viper 配置加载
│   │
│   └── logger/
│       └── logger.go               # zap 日志初始化
│
├── pkg/                            # 可复用工具包
│   ├── jsonrpc/types.go            # JSON-RPC 2.0 类型定义
│   ├── extract/json.go             # LLM 响应 JSON 提取
│   ├── hash/key.go                 # FNV-1a hash key 生成
│   └── idgen/                      # ID 生成（如有）
│
├── docs/
│   ├── openrpc.yaml                # OpenRPC 接口定义（权威源）
│   ├── api.md                      # API 参考文档
│   └── mapping-generator/          # 功能设计文档
│       ├── 001-mapping-generator-设计文档.md
│       ├── 002-功能总结-DDD分层重构.md
│       └── 003-内容映射功能设计.md
│
├── tests/
│   ├── integration/                # 全链路集成测试
│   │   ├── server_test.go
│   │   └── helper_test.go
│   └── unit/
│       ├── domain/mapping/mapping_test.go
│       └── extract/json_test.go
│
├── TODO.md                         # 待办事项
├── AGENTS.md                       # AI 协作开发约定
└── README.md                       # 项目说明
```

---

## 三、核心流程

### 3.1 启动流程

```
main.go
  → app.Run()
    → runWithConfig()
      1. 加载配置 (TOML + 环境变量)
      2. 初始化日志 (zap)
      3. 打开 SQLite (WAL 模式)
      4. 创建 Repository (ProfileRepo + CacheRepo)
      5. 创建 Domain Service (MappingService + ContentMappingService + EnrichService)
      6. 创建 LLM Client Factory
      7. 创建 Application Service (Profile + Mapping + KGC)
      8. 创建 JSON-RPC Server + 注册方法
      9. 注册中间件 (解压 + 压缩)
      10. 启动 HTTP 服务
```

### 3.2 字段映射流程 (`mapping.field`)

```
请求: {profile_id, example, target_fields, refresh?}
  │
  ├── 1. 获取 profile → 解析 LLM 配置
  ├── 2. 从 example 提取 source fields
  ├── 3. 计算 cache key = fnv1a(sorted(source) + sorted(target))
  ├── 4. refresh=false? → 查缓存 → 命中则直接返回
  ├── 5. 未命中 → 构建 prompt → 调 LLM
  ├── 6. 解析 LLM 响应 → 提取 JSON → 校验映射关系
  ├── 7. 存入缓存 (SQLite mapping_cache 表)
  └── 8. 返回 {mapping, unmapped_source, unfilled_target, cached}
```

**缓存策略**：
- key = FNV-1a hash (source_fields + target_fields)
- 全局共享，不区分 profile
- 相同字段组合永不二次调用 LLM（除非 refresh=true）

### 3.3 内容映射流程 (`mapping.content`)

```
请求: {profile_id, topic, values, targets?}
  │
  ├── 1. 获取 profile → 解析 LLM 配置
  ├── 2. 处理 targets:
  │   ├── 传入 targets → 使用该列表并持久化到 content_mapping_targets
  │   └── 未传 targets → 从 content_mapping_targets 读取 → 若无则报错
  ├── 3. 查 content_mapping 表 → 已有映射直接使用
  ├── 4. 找出未映射的 values
  ├── 5. 未映射的 values → 构建 prompt → 调 LLM
  ├── 6. 解析 LLM 响应 → 合并到已有映射 → 存入 content_mapping
  └── 7. 返回 {topic, mapping} (完整映射表)
```

**存储设计**：
```sql
-- 内容映射表
CREATE TABLE content_mapping (
    topic TEXT PRIMARY KEY,
    mapping TEXT NOT NULL,    -- JSON: {"唐朝": "唐"}
    updated_at TEXT NOT NULL
)

-- 目标值表
CREATE TABLE content_mapping_targets (
    topic TEXT PRIMARY KEY,
    targets TEXT NOT NULL,    -- JSON: ["唐", "宋", "明", "清"]
    updated_at TEXT NOT NULL
)
```

### 3.4 数据补全流程 (`kgc.enrich`)

```
请求: {profile_id, data[], examples[], tasks[], max_image_kb?}
  │
  ├── 1. 获取 profile → 解析 LLM 配置
  ├── 2. 校验请求 (行数≤100, 任务定义合法)
  ├── 3. 构建多模态消息:
  │   ├── System: 目标字段说明 + 示例（自动过滤源字段）
  │   └── User: 每行文本字段 + 图片（图片自动压缩，Detail=Low）
  ├── 4. 一次 LLM 调用 → 返回 JSON 数组
  ├── 5. 提取 JSON → 校验行数 → 回填到原始数据
  └── 6. 返回 {data[], enriched_count}
```

**图片处理**：
- 自动压缩到 max_image_kb 以内（默认 200KB）
- 叠白底处理 PNG 透明度
- 缩放最长边 ≤ 1024px
- JPEG 循环降 quality 至目标大小

---

## 四、API 列表

| 方法 | 说明 | 关键参数 |
|------|------|---------|
| `profile.set` | 创建/更新 Profile | profile_id, llm, notify |
| `profile.get` | 查询 Profile | profile_id |
| `profile.delete` | 删除 Profile | profile_id |
| `mapping.field` | 生成字段映射 | profile_id, example, target_fields, refresh |
| `mapping.content` | 查询并自动补全内容映射 | profile_id, topic, values, targets(可选) |
| `mapping.content.set` | 设置/更新内容映射表 | profile_id, topic, mapping |
| `mapping.content.targets.set` | 设置内容映射目标值 | profile_id, topic, targets |
| `mapping.content.targets.get` | 获取内容映射目标值 | profile_id, topic |
| `kgc.enrich` | 数据补全 | profile_id, data, tasks, examples, max_image_kb |
| `rpc.discover` | 服务发现（返回 OpenRPC 文档） | 无 |

---

## 五、关键设计决策

| 决策 | 说明 |
|------|------|
| **JSON-RPC 2.0 协议** | 统一接口风格，支持服务发现 (rpc.discover) |
| **OpenRPC YAML 为权威** | 机器可读 + Schema 校验 + 文档生成 |
| **Schema 校验放 server interceptor** | 在 parse 之后 dispatch 之前集中校验 |
| **LLMClient 接口定义在 domain** | 服务层定义接口，infra 隐式实现 |
| **LLMClientFactory 注入 Application** | app 层注入，handler 不导入 infra/llm |
| **缓存全局共享** | 不区分 profile，相同字段组合共享映射结果 |
| **prompt 末尾 `/nothink`** | 禁用 Qwen3 等模型的思考模式，加快响应 |
| **Result 类型加 json tag** | handler 直接序列化 result，减少 map 构造 |
| **DDD 分层** | domain/application/infra/delivery 四层分离 |

---

## 六、数据库表结构

| 表名 | 用途 | 主键 |
|------|------|------|
| `profiles` | 用户配置 | id |
| `mapping_cache` | 字段映射缓存 | cache_key |
| `content_mapping` | 内容映射表 | topic |
| `content_mapping_targets` | 内容映射目标值 | topic |

---

## 七、测试

```bash
# 运行所有测试
go test ./... -count=1

# 单元测试
go test ./tests/unit/... -count=1

# 集成测试
go test ./tests/integration/... -count=1
```

**测试覆盖**：
- `tests/unit/domain/mapping/` — Mapping 值对象校验测试
- `tests/unit/extract/` — JSON 提取测试（含 deepseek 思维链处理）
- `tests/integration/` — 全链路 JSON-RPC 测试

---

## 八、待办事项 (TODO.md)

| 类别 | 事项 |
|------|------|
| **安全** | API key 加密存储 (AES-256) |
| **容量** | Profile 数量上限 (如 10000 条) |
| **通知** | 通知 Sender 接口 (feishu/email/sms) |
| **备份** | SQLite 定时备份策略 |
| **缓存** | profile.clear_cache RPC |
| **优化** | TOON 输出格式替代 JSON (节省 40-60% token) |

---

## 九、开发约定 (AGENTS.md)

- **AI 是协作开发者，不是直接提交者**
- **必须先读取仓库约定文档再编码**
- **基于 worktree 开展工作**
- **写代码前先写设计文档**
- **完成后必须编译 + 测试通过**
- **中文回答和注释**
- **使用 zap 输出日志**

---

## 十、快速开始

```bash
# 构建
go build -o kgbrain ./cmd/server

# 运行
./kgbrain --config configs/config.toml

# 调用示例
curl -s -X POST http://localhost:8848/rpc \
  -d '{"jsonrpc":"2.0","method":"profile.set","params":{
    "profile_id":"demo",
    "llm":{"base_url":"http://localhost:11434/v1","api_key":"","model":"qwen3"}
  },"id":"req_001"}'
```

---

*最后更新: 2026-05-20*
