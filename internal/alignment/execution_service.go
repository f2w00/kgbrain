// execution_service.go 提供实体对齐领域执行服务，编排字段对齐全流程。
package alignment

import "context"

// DomainService 承载实体对齐的核心业务流程。
type DomainService struct {
	repo             BusinessRepository
	defaultBatchSize int
}

// NewDomainService 创建实体对齐领域执行服务。
func NewDomainService(repo BusinessRepository, defaultBatchSize int) *DomainService {
	return &DomainService{repo: repo, defaultBatchSize: defaultBatchSize}
}

// Execute 串行执行实体对齐流程。
//
// 处理步骤：
//  1. 规范化字段配置（去重、排序 targets，裁剪 batch_size）
//  2. 校验源表、输出表和 mapping 表结构，获取源表列元信息
//  3. 逐字段处理：
//     a. 查询源表该字段所有非空不重复原始值（受 start_id/end_id 范围限制）
//     b. 如果开启复用，加载已有 mapping，并更新使用计数
//     c. 计算缺失的原始值，按 BatchSize 分批调用 LLM 生成映射
//     d. 每批结果写入 mapping 表（upsert）
//  4. 执行数据库侧 INSERT INTO output SELECT ... LEFT JOIN mapping ...
//     一次性完成对齐写入
//
// 当前问题：WriteOutputRows 未分页，大表场景下 Postgres 侧
// work_mem 和锁持有时间可能成为瓶颈。
func (s *DomainService) Execute(
	ctx context.Context,
	llmClient LLMClient,
	req ExecuteRequest,
) error {
	preparedFields, err := PrepareFields(req.Fields, s.defaultBatchSize)
	if err != nil {
		return err
	}
	sourceColumns, err := s.repo.EnsureExecutionReady(ctx, req, preparedFields)
	if err != nil {
		return err
	}
	for _, field := range preparedFields {
		if err := ctx.Err(); err != nil {
			return err
		}
		rawValues, err := s.repo.SelectDistinctRawValues(ctx, req, field.Name)
		if err != nil {
			return err
		}
		if len(rawValues) == 0 {
			continue
		}
		existing := make(map[string]MappingRecord, len(rawValues))
		if req.ReuseMapping {
			existing, err = s.repo.LoadExistingMappings(ctx, req, field, rawValues)
			if err != nil {
				return err
			}
			if len(existing) > 0 {
				if err := s.repo.TouchMappings(ctx, req, field, existing); err != nil {
					return err
				}
			}
		}
		missing := MissingRawValues(rawValues, existing)
		for _, batch := range ChunkStrings(missing, field.BatchSize) {
			generated, err := GenerateMappingsBatch(ctx, llmClient, field, batch)
			if err != nil {
				return err
			}
			if err := s.repo.UpsertMappings(
				ctx,
				req,
				field,
				generated,
				!req.ReuseMapping,
			); err != nil {
				return err
			}
		}
	}
	return s.repo.WriteOutputRows(ctx, req, sourceColumns, preparedFields)
}
