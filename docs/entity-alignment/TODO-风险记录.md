# 实体对齐服务 - 已知风险

## P0: WriteOutputRows 未分页（数据库内存/锁风险，已分页缓解）

`internal/alignment/business_repository.go:324`

`WriteOutputRows` 用单条 `INSERT INTO output SELECT ... FROM source LEFT JOIN mapping ... WHERE TRUE`
处理全量行，无分页机制。

- 源表数百万行且未指定 `start_id`/`end_id` 时，Postgres 需要大量 work_mem 完成 hash join
- `ON CONFLICT DO UPDATE` 在一个事务中持有行锁，阻塞其他写入
- 缓解：按 `key_field` 的数值范围自动分页执行，每页独立事务，降低单条 SQL 的内存与锁风险

## P1: WriteOutputRows 分页后可能产生部分写入

`internal/alignment/business_repository.go:324`

分页后每个 key range 使用独立事务执行 `INSERT INTO ... SELECT ... ON CONFLICT DO UPDATE`。

- 如果第 N 页失败，前 N-1 页已经提交，output table 会出现部分结果
- 当前写入逻辑基于 `ON CONFLICT DO UPDATE`，重跑同一任务可继续补齐或覆盖已有结果
- 后续如果需要强一致输出，可考虑引入 staging table 或记录分页提交状态后统一发布
