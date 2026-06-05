# Python 客户端示例

> 所有 Python 示例基于 `requests` 库，适用于 kgbrain JSON-RPC 2.0 接口。

## 快速开始

```bash
pip install requests
```

## 完整示例

```python
import requests

BASE_URL = "http://localhost:8848/rpc"

def rpc(method, params):
    resp = requests.post(
        BASE_URL,
        json={"jsonrpc": "2.0", "method": method, "params": params, "id": "req_001"}
    )
    return resp.json()

# ======================== Profile 管理 ========================

# 创建/更新配置
rpc("profile.set", {
    "profile_id": "demo",
    "llm": {
        "base_url": "http://localhost:11434/v1",
        "api_key": "",
        "model": "qwen3",
        "timeout_seconds": 180
    }
})

# 查询配置
resp = rpc("profile.get", {"profile_id": "demo"})
print(resp)

# 删除配置
rpc("profile.delete", {"profile_id": "demo"})

# ======================== 字段映射 ========================

# 生成字段映射关系
resp = rpc("mapping.field", {
    "profile_id": "demo",
    "example": {"title": "青花瓷瓶", "era": "明代"},
    "target_fields": [{"product_name": "青花瓷瓶", "dynasty": "明代"}],
    "refresh": False
})
mapping = resp["result"]["mapping"]
# 输出: {"product_name": ["title"], "dynasty": ["era"]}

# 批量应用映射
rows = [
    {"title": "青花瓷瓶", "era": "明代"},
    {"title": "铜鼎", "era": "商代"}
]
output = [{tgt: row.get(srcs[0], "") for tgt, srcs in mapping.items()} for row in rows]
print(output)
# [{'product_name': '青花瓷瓶', 'dynasty': '明代'}, {'product_name': '铜鼎', 'dynasty': '商代'}]

# ======================== 内容映射 ========================

# 设置目标值
rpc("mapping.content.targets.set", {
    "profile_id": "demo",
    "topic": "dynasty",
    "targets": ["唐", "宋", "明", "清"]
})

# 查询映射（未命中的值自动调 LLM 映射并缓存）
resp = rpc("mapping.content", {
    "profile_id": "demo",
    "topic": "dynasty",
    "values": ["唐朝", "宋朝", "元朝"]
})
content_mapping = resp["result"]["mapping"]
# 输出: {"唐朝": "唐", "宋朝": "宋", "元朝": "元"}

# 设置内容映射表
rpc("mapping.content.set", {
    "profile_id": "demo",
    "topic": "location",
    "mapping": {"长安": "西安", "汴京": "开封"}
})

# 获取目标值
resp = rpc("mapping.content.targets.get", {
    "profile_id": "demo",
    "topic": "dynasty"
})
print(resp["result"]["targets"])
# ['唐', '宋', '明', '清']

# ======================== 数据补全 (enrich) ========================

# 需要明确定义 tasks: 指定源字段和目标字段
resp = rpc("kgc.enrich", {
    "profile_id": "demo",
    "data": [
        {
            "title": "青花瓷瓶",
            "dynasty": "",
            "material": "",
            "remark": ""
        }
    ],
    "examples": [
        {
            "title": "明代青花山水纹瓶",
            "dynasty": "明代",
            "material": "陶瓷",
            "remark": "瓶身绘有山水图案"
        }
    ],
    "tasks": [
        {"source_type": "text", "source_fields": ["title"], "targets": ["dynasty", "material", "remark"]}
    ]
})
result = resp["result"]
print(f"补全了 {result['enriched_count']} 个字段")
print(result["data"])
# [{'title': '青花瓷瓶', 'dynasty': '明代', 'material': '陶瓷', 'remark': '...'}]

# ======================== 智能补全 (autofill) ========================

# 不需要定义 tasks，传入目标结构示例，LLM 自主推断映射关系
resp = rpc("kgc.autofill", {
    "profile_id": "demo",
    "data": [
        {"title": "青花瓷瓶", "era": "明代"},
        {"title": "唐三彩马", "era": "唐代"}
    ],
    "targets_example": [
        {"product_name": "青花瓷瓶", "dynasty": "明代"}
    ]
})

result = resp["result"]

# 提取补全结果
for row in result["data"]:
    print(row)

# ======================== 错误处理 ========================

def safe_rpc(method, params):
    """带错误处理的 RPC 调用"""
    resp = requests.post(
        BASE_URL,
        json={"jsonrpc": "2.0", "method": method, "params": params, "id": "req"}
    )
    data = resp.json()
    if "error" in data:
        err = data["error"]
        print(f"请求失败: code={err['code']}, message={err['message']}")
        if err.get("data"):
            print(f"  详情: {err['data']}")
        return None
    return data

```

## 异步版本 (aiohttp)

```python
import aiohttp
import asyncio

BASE_URL = "http://localhost:8848/rpc"

async def async_rpc(session, method, params):
    resp = await session.post(
        BASE_URL,
        json={"jsonrpc": "2.0", "method": method, "params": params, "id": "req"}
    )
    return await resp.json()

async def main():
    async with aiohttp.ClientSession() as session:
        # 批量调用
        resp = await async_rpc(session, "kgc.autofill", {
            "profile_id": "demo",
            "data": [
                {"title": "青花瓷瓶", "era": "明代"},
                {"title": "唐三彩马", "era": "唐代"}
            ],
            "targets_example": [
                {"product_name": "青花瓷瓶", "dynasty": "明代"}
            ]
        })
        print(resp)

asyncio.run(main())
```

## 异步大规模数据转换 (xform)

适用于大批量数据（数千到数百万行）的流式异步处理。

```python
import requests
import time
import json

BASE_URL = "http://localhost:8848/rpc"

def rpc(method, params, req_id="req"):
    """发送 JSON-RPC 请求"""
    resp = requests.post(
        BASE_URL,
        json={"jsonrpc": "2.0", "method": method, "params": params, "id": req_id}
    )
    data = resp.json()
    if "error" in data:
        raise Exception(f"RPC Error: {data['error']}")
    return data["result"]

# ======================== 1. 提交任务 ========================

# 方式 A：自定义 task_id（推荐，便于追踪）
result = rpc("xform.submit", {
    "task_id": "my_batch_20260601_001",
    "profile_id": "demo",
    "targets_example": [
        {"product_name": "青花瓷瓶", "dynasty": "明代", "category": "陶瓷"}
    ],
    "primary_key": "relic_id",
    "pool_size": 10,
    "max_retries": 2,
    "ttl_hours": 24
})

# 方式 B：不传 task_id，服务端自动生成
# result = rpc("xform.submit", {
#     "profile_id": "demo",
#     "targets_example": [
#         {"product_name": "青花瓷瓶", "dynasty": "明代", "category": "陶瓷"}
#     ],
#     "primary_key": "relic_id",
#     "pool_size": 10,
#     "max_retries": 2,
#     "ttl_hours": 24
# })

task_id = result["task_id"]
print(f"任务已创建: {task_id}")

# ======================== 2. 持续追加数据 ========================

# 第一批数据
rpc("xform.append", {
    "task_id": task_id,
    "data": [
        {"relic_id": "R001", "title": "青花山水纹瓶", "era": "明代"},
        {"relic_id": "R002", "title": "唐三彩马", "era": "唐代"}
    ]
})

# 第二批数据（可随时追加）
rpc("xform.append", {
    "task_id": task_id,
    "data": [
        {"relic_id": "R003", "title": "商代青铜鼎", "era": "商代"},
        {"relic_id": "R004", "title": "宋代汝窑茶盏", "era": "宋代"}
    ]
})

print("数据已追加")

# ======================== 3. 轮询状态 ========================

while True:
    status = rpc("xform.get_status", {"task_id": task_id})
    print(f"状态: {status['status']}, 待消费: {status['pending_input']}, "
          f"待取结果: {status['pending_output']}, 待取错误: {status['pending_errors']}")

    if status["status"] in ("completed", "partial", "failed"):
        break

    time.sleep(1)

# ======================== 4. 关闭任务 ========================

rpc("xform.close", {"task_id": task_id})
print("任务已关闭，Worker 正在处理剩余数据...")

# 等待最终完成
while True:
    status = rpc("xform.get_status", {"task_id": task_id})
    if status["status"] in ("completed", "partial", "failed"):
        break
    time.sleep(1)

# ======================== 5. 拉取结果（消费即删）========================

all_results = []
while True:
    result = rpc("xform.get_result", {
        "task_id": task_id,
        "limit": 100
    })

    if result["results"]:
        all_results.extend(result["results"])

    if result["errors"]:
        print(f"发现 {len(result['errors'])} 条错误:")
        for err in result["errors"]:
            print(f"  {err['pk_value']}: {err['_error']}")

    if not result["results"] and result["pending_output"] == 0 and result["pending_errors"] == 0:
        break

    time.sleep(0.5)

print(f"共获取 {len(all_results)} 条结果")
for row in all_results[:3]:
    print(json.dumps(row, ensure_ascii=False, indent=2))

# ======================== 6. 清理任务 ========================

rpc("xform.delete", {"task_id": task_id})
print("任务已删除")
```

### xform 接口说明

| 方法 | 说明 | 请求参数 | 返回 |
|------|------|---------|------|
| `xform.submit` | 提交任务 | task_id (可选), profile_id, targets_example, primary_key, pool_size, max_retries, ttl_hours | task_id |
| `xform.append` | 追加数据 | task_id, data | appended |
| `xform.close` | 关闭任务 | task_id | status |
| `xform.get_status` | 查询状态 | task_id | status, pending_input, pending_output, pending_errors |
| `xform.get_result` | 拉取结果 | task_id, limit | results, errors, pending_output, pending_errors |
| `xform.delete` | 删除任务 | task_id | deleted |

### 状态流转

```
submit → open → processing → closing → completed/partial/failed
                                    ↓
                               deleted (随时可删除)
```
