# 实体对齐服务 - 已知风险与待办

## P0: WriteOutputRows 分页后可能产生部分写入

`internal/alignment/business_repository.go:WriteOutputRows`

分页后每个 key range 使用独立事务执行 `INSERT INTO ... SELECT ... ON CONFLICT DO UPDATE`。

- 如果第 N 页失败，前 N-1 页已经提交，output table 会出现部分结果。
- 当前写入逻辑基于 `ON CONFLICT DO UPDATE`，重跑同一任务可继续补齐或覆盖已有结果。
- **暂不修复**：后续如果需要强一致输出，可考虑引入 staging table 或记录分页提交状态后统一发布。

## P0: 第二轮重构范围较大，需注意模块边界

第二轮不再引入独立 TargetSetService。标准值只服务实体对齐，相关接口挂在 EntityAlignmentService 下。

- 标准值表简化为 `alignment_targets`，以 `target_set_id + label` 为主键。
- EntityAlignmentService 的核心对齐流程需要修改：从 alignment_targets 获取目标列表，输出时处理 needs_candidate。
- candidate 审核直接更新 target 或 mapping，不做 version/publish。

## P1: needs_candidate 判定依赖 LLM 准确性

- LLM 判定 `needs_candidate` 的边界可能模糊：某些值可能应该被判为 `unknown_to_null`。
- 需要设计 prompt 时明确区分"无意义值"和"未覆盖的有意义值"。
- 后续可引入阈值机制：低置信度的匹配结果降级为 needs_candidate 而非强行 matched。

## P1: 候选集去重依赖 raw_value 原值一致性

- 同一原始值在不同编码（全角/半角）下会被视为不同候选。
- 当前版本不对 raw_value 做归一化处理，候选集可能出现近似重复。
- 后续可在录入候选集时增加归一化步骤或模糊去重。

## P1: 标准值集合变化时旧 mapping 的语义边界

- 当前不做 target_set_version，mapping 复用以 `target_set_id + raw_value` 为边界。
- 标准值集合增量更新后，旧 mapping 仍然会被复用。
- 如果某个标准 label 的语义发生变化，需要人工修改或删除对应 mapping。

## P1: 当前召回策略为 pg_trgm 模糊召回 + LLM 判定

- 当前不再将完整 `alignment_targets` 放入每次 LLM prompt。
- 对每个缺失 mapping 的 `raw_value`，先在业务 PostgreSQL 内使用 `pg_trgm` 从
  `alignment_targets.label` 召回 topK 候选，再交给 LLM 做最终
  `matched/unknown_to_null/needs_candidate` 判定。
- `fuzzy_top_k` 默认值为 10，可在启动实体对齐任务时调整。
- 业务库会确保存在 `pg_trgm` 扩展和 `alignment_targets.label` 的 GIN trigram
  索引：`alignment_targets_label_trgm_idx`。
- 该方案是字符级模糊匹配，不是语义召回；字符不重叠但语义相近的值仍可能漏召回。
- 召回结果不会单独落库；最终可复用结果仍写入 `entity_alignment_mapping`。
- 召回为空时直接生成 `needs_candidate`，进入候选审核流程。

### pg_trgm 运维注意事项

- `CREATE EXTENSION IF NOT EXISTS pg_trgm` 需要业务库账号具备创建扩展权限。
- 生产环境更推荐由 DBA 或初始化脚本预先安装 `pg_trgm`，应用侧只做幂等检查和建索引。
- 如果后续发现大小写、空格、标点、全半角影响召回效果，可引入
  `normalized_label` 字段和对应 trigram 索引。

## P2: 行级 waiting_target_review 状态的反向更新

- 当候选值被审核（add_as_label/map_to_existing/reject_as_null）后，之前标记为 waiting_target_review 的行需要等待下一次对齐任务。
- processrecord 不直接监听候选审核事件——设计上由下一次对齐任务统一处理。
- 如果用户长期不重新对齐，waiting_target_review 的行在输出表中为 null，可能导致下游数据质量问题。

## P2: 未匹配行标记为 waiting_target_review 但输出为 null 的语义

- 用户可能困惑：为什么输出表中某些字段为 null？
- null 的可能原因：原始值本身为 null、unknown_to_null、needs_candidate。
- 需要在输出表或关联文档中说明 null 的不同语义，也可考虑在 output_table 增加 `_alignment_status` 辅助字段。

## P3: 菩萨像→佛像 类型的语义推断

- 当前阶段不做 embedding 向量召回 + LLM rerank 的复杂语义推断。
- 明确的映射（唐朝→唐）由 LLM 处理；跨概念映射（菩萨像→佛像）推迟到后续阶段。
- 用户需理解当前阶段的适用范围：只做标准化对齐，不做语义推断。

## P3: 标准值集合过大时引入 embedding 召回

- 当 `alignment_targets` 中某个 `target_set_id` 下 label 数量达到几千或更多时，不应把完整标准表放入 LLM prompt。
- 后续优先考虑 embedding 方案：对标准值 label 生成向量，按 raw_value 向量召回 topK labels，再交给 LLM 做最终确认。
- 该方案当前暂不实现；实现前需补充 target embedding 存储、索引、更新策略和召回参数设计。

## 待办事项

| 优先级 | 事项 | 状态 |
|--------|------|------|
| P0 | 设计 EntityAlignmentService 下的 target/candidate proto 接口 | 已完成 |
| P0 | 定义 alignment_targets 表结构 | 已完成 |
| P0 | 定义 target_candidates 表结构 | 已完成 |
| P0 | 修改 entity_alignment_mapping 表：移除 fallback_original，增加 needs_candidate | 已完成 |
| P0 | 修改 data_process_records：增加 waiting_target_review 状态 | 已完成 |
| P0 | 修改 proto: EntityAlignmentField 移除 targets，增加可选 target_set_id | 已完成 |
| P0 | 实现 target 标准值列表与 upsert 接口 | 已完成 |
| P1 | 修改 alignment 核心流程：从 alignment_targets 加载目标列表 | 已完成 |
| P1 | 修改 LLM prompt：matched/unknown_to_null/needs_candidate 三态 | 已完成 |
| P1 | 实现候选录入：对齐过程中写入 target_candidates | 已完成 |
| P1 | 实现 output 写入过滤：needs_candidate 输出 null | 已完成 |
| P1 | 实现 processrecord 写入时判断 waiting_target_review | 已完成 |
| P1 | 实现 ReviewTargetCandidates 审核并回写 target/mapping | 已完成 |
| P1 | 实现 pg_trgm 模糊召回 topK，再由 LLM 在候选内判定 | 已完成 |
| P2 | 添加 target_set_id 到 mapping 表 | 已完成 |
| P2 | 实现重新对齐时只处理 waiting_target_review 行 | 待开始 |
| P2 | 评估是否增加 normalized_label 以提升模糊召回质量 | 待开始 |
| P3 | 重写第一轮设计文档（已完成） | 已完成 |
| P3 | LLM prompt 优化：降低误判 needs_candidate 的比例 | 待开始 |
| P3 | 设计 embedding 召回方案：标准值向量化、topK 召回、LLM rerank | 后续可选 |
