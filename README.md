# kgbrain

字段映射生成服务 — 通过 LLM 推导源字段到目标字段的映射关系，一次 LLM 调用，无限次程序化应用。

## 快速开始

### 构建

```bash
go build -o kgbrain ./cmd/server
```

### 运行

```bash
./kgbrain --config configs/config.toml
```

### 调用示例

```bash
# 1. 创建 profile（LLM 配置）
curl -s -X POST http://localhost:8848/rpc \
  -d '{"jsonrpc":"2.0","method":"profile.set","params":{
    "profile_id":"demo",
    "llm":{"base_url":"http://localhost:11434/v1","api_key":"","model":"qwen3"}
  },"id":"req_001"}'

# 2. 生成字段映射
curl -s -X POST http://localhost:8848/rpc \
  -d '{"jsonrpc":"2.0","method":"mapping.generate","params":{
    "profile_id":"demo",
    "example":{"title":"青花瓷瓶","era":"明代","material":"陶瓷"},
    "target_fields":["product_name","dynasty","material_type"]
  },"id":"req_002"}'

# 3. 强制刷新缓存
curl -s -X POST http://localhost:8848/rpc \
  -d '{"jsonrpc":"2.0","method":"mapping.generate","params":{
    "profile_id":"demo",
    "example":{"title":"青花瓷瓶","era":"明代"},
    "target_fields":["product_name","dynasty"],
    "refresh":true
  },"id":"req_003"}'
```

### Python 客户端

```python
import requests

def rpc(method, params):
    return requests.post("http://localhost:8848/rpc",
        json={"jsonrpc":"2.0","method":method,"params":params,"id":"req"}).json()

# 创建配置
rpc("profile.set", {"profile_id":"demo", "llm":{
    "base_url":"http://localhost:11434/v1","api_key":"","model":"qwen3"}})

# 生成映射
resp = rpc("mapping.generate", {
    "profile_id":"demo",
    "example":{"title":"青花瓷瓶","era":"明代"},
    "target_fields":["name","dynasty"]})
mapping = resp["result"]["mapping"]  # {"name": ["title"], "dynasty": ["era"]}

# 批量应用
rows = [{"title":"青花瓷瓶","era":"明代"}, {"title":"铜鼎","era":"商代"}]
output = [{tgt: row[srcs[0]] for tgt, srcs in mapping.items()} for row in rows]
```

## API

| 方法 | 说明 |
|------|------|
| `profile.set` | 创建/更新 Profile（LLM + 通知配置） |
| `profile.get` | 查询 Profile |
| `profile.delete` | 删除 Profile（级联清空 mapping 缓存） |
| `mapping.generate` | 生成字段映射关系（支持 `refresh` 参数强制刷新） |

详见 [docs/api.md](docs/api.md)。

## 技术栈

| 组件 | 选型 |
|------|------|
| 协议 | JSON-RPC 2.0 |
| LLM | cloudwego/eino (OpenAI 兼容) |
| 存储 | SQLite (modernc.org/sqlite) |
| 日志 | go.uber.org/zap |
| Profile | SQLite 持久化，支持 LLM + 通知配置 |
| 映射缓存 | SQLite `mapping_cache` 表，一致性缓存 |
| 模型思考控制 | prompt 末尾 `/nothink` 禁用 Qwen3 思考模式 |

## 特性

- **缓存复用**：相同 `profile_id + 字段组合` 自动从缓存返回，不做二次 LLM 调用
- **强制刷新**：请求参数 `refresh: true` 跳过缓存，重新生成并覆盖
- **LLM 类型安全**：`profile.LLMConfig` 命名结构体，支持 `ParseLLMConfig()` 解析
- **纯 JSON 输出**：`response_format: json_object` + `/nothink`，确保模型输出合法 JSON
- **通知**：支持飞书 webhook 等渠道（通过 profile NotifyConfig 配置）

## 测试

```bash
go test ./tests/... -count=1
```
