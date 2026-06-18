# Python 客户端示例（Connect）

> 当前 kgbrain 仅保留 Connect RPC 接口。历史 JSON-RPC 示例已下线，相关历史文档保留在
> `docs/deprecated/` 目录。

本文示例覆盖当前仍可用的三类能力：

- `resource`：管理 LLM / 数据库资源配置
- `enrichextract`：结构化抽取与补全任务
- `entity-alignment`：实体值对齐任务

## 安装依赖

```bash
pip install connectrpc protobuf
```

## 目录准备

以下示例默认你已经生成并能导入 Python 客户端代码，例如：

```text
gen/py/kgbrain/v1/
  enrich_extract_connect.py
  enrich_extract_pb2.py
  entity_alignment_connect.py
  entity_alignment_pb2.py
  resource_connect.py
  resource_pb2.py
```

运行示例前，请确保 `gen/py` 已在 `PYTHONPATH` 中，例如：

```bash
export PYTHONPATH="$PWD/gen/py:$PYTHONPATH"
```

## 通用辅助函数

```python
import time

from google.protobuf import struct_pb2


BASE_URL = "http://localhost:8848"


def make_struct(data: dict) -> struct_pb2.Struct:
    msg = struct_pb2.Struct()
    msg.update(data)
    return msg


def wait_until(done, interval_seconds: int = 2):
    while True:
        result = done()
        if result is not None:
            return result
        time.sleep(interval_seconds)
```

## 一、资源管理（resource）

资源是其他任务的前置依赖。`enrichextract` 和 `entity-alignment` 都通过资源 ID
引用 LLM 和数据库连接配置。

### 1.1 创建或更新 LLM 资源

```python
from kgbrain.v1.resource_pb2 import LLMResourceConfig
from kgbrain.v1.resource_pb2 import SetLLMResourceRequest
from kgbrain.v1.resource_connect import ResourceServiceClientSync


BASE_URL = "http://localhost:8848"


with ResourceServiceClientSync(BASE_URL) as client:

    resp = client.set_l_l_m_resource(
        SetLLMResourceRequest(
            resource_id="qwen_local",
            name="本地 Qwen 模型",
            config=LLMResourceConfig(
                base_url="http://localhost:11434/v1",
                api_key="dummy",
                model="qwen3:latest",
                timeout_seconds=180,
                temperature=0.2,
                max_concurrency=8,
            ),
        )
    )

    print(resp.resource_id, resp.status)
```

### 1.2 查询 LLM 资源

```python
from kgbrain.v1.resource_pb2 import GetLLMResourceRequest
from kgbrain.v1.resource_connect import ResourceServiceClientSync


BASE_URL = "http://localhost:8848"


with ResourceServiceClientSync(BASE_URL) as client:

    resp = client.get_l_l_m_resource(
        GetLLMResourceRequest(resource_id="qwen_local")
    )

    print("resource_id:", resp.resource_id)
    print("name:", resp.name)
    print("base_url:", resp.config.base_url)
    print("model:", resp.config.model)
    print("timeout_seconds:", resp.config.timeout_seconds)
```

### 1.3 创建或更新数据库资源

```python
from kgbrain.v1.resource_pb2 import DATABASE_TYPE_POSTGRES
from kgbrain.v1.resource_pb2 import DatabaseResourceConfig
from kgbrain.v1.resource_pb2 import PostgresResourceConfig
from kgbrain.v1.resource_pb2 import SetDatabaseResourceRequest
from kgbrain.v1.resource_connect import ResourceServiceClientSync


BASE_URL = "http://localhost:8848"


with ResourceServiceClientSync(BASE_URL) as client:

    resp = client.set_database_resource(
        SetDatabaseResourceRequest(
            resource_id="museum_pg",
            name="博物馆业务库",
            config=DatabaseResourceConfig(
                type=DATABASE_TYPE_POSTGRES,
                postgres=PostgresResourceConfig(
                    host="127.0.0.1",
                    port=5432,
                    database="museum",
                    user="postgres",
                    password="postgres",
                    sslmode="disable",
                ),
            ),
        )
    )

    print(resp.resource_id, resp.status)
```

### 1.4 查询数据库资源

```python
from kgbrain.v1.resource_pb2 import GetDatabaseResourceRequest
from kgbrain.v1.resource_connect import ResourceServiceClientSync


BASE_URL = "http://localhost:8848"


with ResourceServiceClientSync(BASE_URL) as client:

    resp = client.get_database_resource(
        GetDatabaseResourceRequest(resource_id="museum_pg")
    )

    print("resource_id:", resp.resource_id)
    print("name:", resp.name)
    print("host:", resp.config.postgres.host)
    print("database:", resp.config.postgres.database)
    print("user:", resp.config.postgres.user)
```

### 1.5 删除资源

```python
from kgbrain.v1.resource_pb2 import DeleteDatabaseResourceRequest
from kgbrain.v1.resource_pb2 import DeleteLLMResourceRequest
from kgbrain.v1.resource_connect import ResourceServiceClientSync


BASE_URL = "http://localhost:8848"


with ResourceServiceClientSync(BASE_URL) as client:

    llm_resp = client.delete_l_l_m_resource(
        DeleteLLMResourceRequest(resource_id="qwen_local")
    )
    print("delete llm:", llm_resp.resource_id, llm_resp.status)

    db_resp = client.delete_database_resource(
        DeleteDatabaseResourceRequest(resource_id="museum_pg")
    )
    print("delete db:", db_resp.resource_id, db_resp.status)
```

## 二、结构化抽取与补全（enrichextract）

`enrichextract` 适用于：

- 从源表 JSON 字段中抽取结构化字段
- 用 LLM 对目标字段做轻量补全
- 后台异步批处理整张表

### 2.1 示例表结构

源表：

```sql
CREATE TABLE public.artifact_raw (
  id BIGINT PRIMARY KEY,
  raw_data JSONB NOT NULL
);
```

输出表：

```sql
CREATE TABLE public.artifact_extract (
  id BIGINT PRIMARY KEY,
  standard_name TEXT,
  dynasty TEXT,
  material TEXT,
  category TEXT
);
```

### 2.2 启动任务并轮询状态

```python
import time

from google.protobuf import struct_pb2

from kgbrain.v1.enrich_extract_pb2 import (
    ENRICH_EXTRACT_JOB_STATUS_FAILED,
    ENRICH_EXTRACT_JOB_STATUS_PARTIAL,
    ENRICH_EXTRACT_JOB_STATUS_SUCCEEDED,
    ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_TEXT,
    EnrichExtractOutputColumn,
    GetEnrichExtractJobRequest,
    StartEnrichExtractRequest,
)
from kgbrain.v1.enrich_extract_connect import EnrichExtractServiceClientSync


BASE_URL = "http://localhost:8848"


def make_struct(data: dict) -> struct_pb2.Struct:
    msg = struct_pb2.Struct()
    msg.update(data)
    return msg


with EnrichExtractServiceClientSync(BASE_URL) as client:

    start_resp = client.start_enrich_extract(
        StartEnrichExtractRequest(
            llm_resource_id="qwen_local",
            database_resource_id="museum_pg",
            source_table="public.artifact_raw",
            output_table="public.artifact_extract",
            key_field="id",
            source_json_field="raw_data",
            output_schema=[
                EnrichExtractOutputColumn(
                    name="standard_name",
                    type=ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_TEXT,
                ),
                EnrichExtractOutputColumn(
                    name="dynasty",
                    type=ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_TEXT,
                ),
                EnrichExtractOutputColumn(
                    name="material",
                    type=ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_TEXT,
                ),
                EnrichExtractOutputColumn(
                    name="category",
                    type=ENRICH_EXTRACT_OUTPUT_COLUMN_TYPE_TEXT,
                ),
            ],
            target_example=make_struct(
                {
                    "standard_name": "青花瓷盘",
                    "dynasty": "明代",
                    "material": "瓷",
                    "category": "瓷器",
                }
            ),
            priority_field_hints={
                "dynasty": (
                    "朝代信息。优先从名称、标题、描述、年代、分类等字段中提取，"
                    "例如“明代青花瓷盘”应提取为“明代”。"
                ),
                "material": (
                    "材质信息。优先从名称、描述、工艺、材质字段中提取，"
                    "例如瓷、铜、玉、纸本等。"
                ),
            },
            start_id=1000,
            end_id=2000,
            concurrency=30,
            page_size=300,
            max_retries=2,
            overwrite=False,
            auto_create_output_table=True,
        )
    )

    job_id = start_resp.job_id
    print(f"任务已创建: {job_id}, status={start_resp.status}")

    while True:
        job_resp = client.get_enrich_extract_job(
            GetEnrichExtractJobRequest(job_id=job_id)
        )

        print(
            f"status={job_resp.status}, "
            f"last_key={job_resp.last_key}, "
            f"processed={job_resp.processed_rows}, "
            f"succeeded={job_resp.succeeded_rows}, "
            f"failed={job_resp.failed_rows}"
        )

        if job_resp.status in (
            ENRICH_EXTRACT_JOB_STATUS_SUCCEEDED,
            ENRICH_EXTRACT_JOB_STATUS_PARTIAL,
            ENRICH_EXTRACT_JOB_STATUS_FAILED,
        ):
            if job_resp.error_message:
                print("error_message:", job_resp.error_message)
            break

        time.sleep(2)
```

### 2.3 使用说明

- `source_table` 需要包含整数主键列 `key_field`
- `source_json_field` 默认为 `raw_data`，推荐使用 `jsonb`
- `output_schema` 是必填输出结构，服务按它校验、建表、写库和约束 LLM 输出字段
- `auto_create_output_table=True` 时，`output_table` 不存在会按 `output_schema` 自动创建
- `auto_create_output_table=False` 时，`output_table` 需要提前建好，包含 `key_field` 和目标输出列
- 已存在的 `output_table` 不会被自动补列或改类型，只会校验结构
- `target_example` 只用于展示 LLM 输出格式，不会原样写入输出表，也不再用于推导字段
- `target_example` 在 proto 中是 `google.protobuf.Struct`，需要显式把 Python `dict`
  转成 `Struct`
- `priority_field_hints` 可选，用于按 `字段名 -> 说明` 强化重点字段抽取；key 必须属于
  `output_schema` 定义的目标字段
- `priority_field_hints` 只影响 prompt，不会新增输出列，也不会强制字段非空
- 当 `overwrite=False` 时，服务会跳过 `output_table` 中已存在的主键记录

## 三、实体对齐（entity-alignment）

`entity-alignment` 适用于：

- 对单个或多个字段做标准值对齐
- 将自由文本映射到固定目标值集合
- 按字段分别配置批大小和并发度

### 3.1 示例场景

假设源表里有 `city_name`、`museum_level` 这类自由填写字段，希望对齐为固定标准值。

### 3.2 启动任务并轮询状态

```python
import time

from kgbrain.v1.entity_alignment_pb2 import (
    ENTITY_ALIGNMENT_JOB_STATUS_FAILED,
    ENTITY_ALIGNMENT_JOB_STATUS_SUCCEEDED,
    EntityAlignmentField,
    GetEntityAlignmentJobRequest,
    StartEntityAlignmentRequest,
)
from kgbrain.v1.entity_alignment_connect import EntityAlignmentServiceClientSync


BASE_URL = "http://localhost:8848"


with EntityAlignmentServiceClientSync(BASE_URL) as client:

    start_resp = client.start_entity_alignment(
        StartEntityAlignmentRequest(
            llm_resource_id="qwen_local",
            database_resource_id="museum_pg",
            source_table="public.museum_raw",
            output_table="public.museum_aligned",
            key_field="id",
            start_id=1,
            end_id=50000,
            fields=[
                EntityAlignmentField(
                    name="city_name",
                    target_set_id="city_name",
                    batch_size=200,
                ),
                EntityAlignmentField(
                    name="museum_level",
                    target_set_id="museum_level",
                    batch_size=100,
                ),
            ],
        )
    )

    job_id = start_resp.job_id
    print(f"任务已创建: {job_id}, status={start_resp.status}")

    while True:
        job_resp = client.get_entity_alignment_job(
            GetEntityAlignmentJobRequest(job_id=job_id)
        )

        print(
            f"status={job_resp.status}, "
            f"source_table={job_resp.source_table}, "
            f"output_table={job_resp.output_table}"
        )

        if job_resp.status in (
            ENTITY_ALIGNMENT_JOB_STATUS_SUCCEEDED,
            ENTITY_ALIGNMENT_JOB_STATUS_FAILED,
        ):
            if job_resp.error_message:
                print("error_message:", job_resp.error_message)
            break

        time.sleep(2)
```

### 3.3 使用说明

- `fields` 是核心配置；标准目标值不再直接写在请求里，而是通过
  `target_set_id` 关联业务库中的 `alignment_targets`
- 服务固定复用已有 mapping；LLM 只处理缺失 mapping 的原始值
- `batch_size` 决定一次送给 LLM 的源值数量
- `batch_concurrency` 当前已禁用；服务端按 batch 串行处理，传入该字段也会被忽略
- `output_table` 需要提前准备好可写结构

### 3.4 审核候选值并仅重跑等待审核记录

当某些原始值被判定为 `needs_candidate` 时，系统会把对应行标记为
`waiting_target_review`。推荐处理闭环如下：

1. 查询候选值列表
2. 调用 `ReviewTargetCandidates` 完成人工审核
3. 重新调用 `StartEntityAlignment`，并设置 `only_waiting_target_review=True`

```python
from kgbrain.v1.entity_alignment_pb2 import (
    EntityAlignmentField,
    ReviewTargetCandidateAction,
    ReviewTargetCandidatesRequest,
    StartEntityAlignmentRequest,
    TargetCandidateResolution,
)
from kgbrain.v1.entity_alignment_connect import EntityAlignmentServiceClientSync


BASE_URL = "http://localhost:8848"


with EntityAlignmentServiceClientSync(BASE_URL) as client:

    # 1. 人工审核候选值。
    client.review_target_candidates(
        ReviewTargetCandidatesRequest(
            database_resource_id="museum_pg",
            source_table="public.museum_raw",
            target_set_id="city_name",
            actions=[
                ReviewTargetCandidateAction(
                    candidate_id="candidate_123",
                    resolution=(
                        TargetCandidateResolution.
                        TARGET_CANDIDATE_RESOLUTION_MAP_TO_EXISTING
                    ),
                    label="西安市",
                    review_reason="人工确认归并到现有标准城市",
                ),
                ReviewTargetCandidateAction(
                    candidate_id="candidate_124",
                    resolution=(
                        TargetCandidateResolution.
                        TARGET_CANDIDATE_RESOLUTION_REJECT_AS_NULL
                    ),
                    review_reason="无实际含义",
                ),
            ],
        )
    )

    # 2. 只重跑当前仍处于 waiting_target_review 的记录。
    rerun_resp = client.start_entity_alignment(
        StartEntityAlignmentRequest(
            llm_resource_id="qwen_local",
            database_resource_id="museum_pg",
            source_table="public.museum_raw",
            output_table="public.museum_aligned",
            key_field="id",
            only_waiting_target_review=True,
            fields=[
                EntityAlignmentField(
                    name="city_name",
                    target_set_id="city_name",
                ),
                EntityAlignmentField(
                    name="museum_level",
                    target_set_id="museum_level",
                ),
            ],
        )
    )

    print("重跑任务已创建:", rerun_resp.job_id)
```

补充说明：

- `ReviewTargetCandidatesRequest.source_table` 当前是保留字段，可传原始任务的 `source_table`，服务端不会用它决定 mapping 表位置
- alignment 内部表固定在 `public` 下共享：`alignment_targets`、`target_candidates`、`entity_alignment_mapping`
- `only_waiting_target_review=True` 时，服务只处理当前仍等待 target 审核的记录，并固定复用审核后回写的 mapping

## 四、统一错误处理示例

```python
from connectrpc.errors import ConnectError


def run_request(callable_):
    try:
        return callable_()
    except ConnectError as err:
        print("Connect 调用失败")
        print("code:", err.code)
        print("message:", err.message)
        raise
```

使用方式：

```python
resp = run_request(
    lambda: client.get_l_l_m_resource(
        GetLLMResourceRequest(resource_id="qwen_local")
    )
)
print(resp)
```

## 五、建议调用顺序

在真实项目里，建议按这个顺序使用：

1. 先配置资源
   - 创建 LLM 资源
   - 创建数据库资源

2. 再启动任务
   - 表结构准备完成后调用 `enrichextract` 或 `entity-alignment`

3. 最后轮询任务状态
   - 成功时检查输出表
   - 失败时读取 `error_message`

## 六、当前可用服务清单

| 服务 | 方法 | 用途 |
|------|------|------|
| `ResourceService` | `SetLLMResource` / `GetLLMResource` / `DeleteLLMResource` | 管理 LLM 资源 |
| `ResourceService` | `SetDatabaseResource` / `GetDatabaseResource` / `DeleteDatabaseResource` | 管理数据库资源 |
| `EnrichExtractService` | `StartEnrichExtract` / `GetEnrichExtractJob` | 结构化抽取与补全 |
| `EntityAlignmentService` | `StartEntityAlignment` / `GetEntityAlignmentJob` | 实体值对齐 |
