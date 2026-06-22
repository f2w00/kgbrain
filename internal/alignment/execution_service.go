// execution_service.go 提供实体对齐领域执行服务，编排字段对齐全流程。
package alignment

import (
	"context"
	"fmt"
)

// DomainService 承载实体对齐的核心业务流程。
type DomainService struct {
	repo             BusinessRepository
	defaultBatchSize int
	processRecorder  ProcessRecorder
}

// NewDomainService 创建实体对齐领域执行服务。
func NewDomainService(
	repo BusinessRepository,
	defaultBatchSize int,
	processRecorder ProcessRecorder,
) *DomainService {
	return &DomainService{
		repo:             repo,
		defaultBatchSize: defaultBatchSize,
		processRecorder:  processRecorder,
	}
}

// Execute 串行执行实体对齐流程。
//
// 处理步骤：
//  1. 规范化字段配置（去重、排序 targets，裁剪 mapping 写入 batch_size）
//  2. 校验源表、输出表和 mapping 表结构，获取源表列元信息
//  3. 逐字段处理：
//     a. 查询源表该字段所有非空不重复原始值（受 start_id/end_id 范围限制）
//     b. 如果开启复用，加载已有 mapping，并更新使用计数
//     c. 计算缺失的原始值，逐个调用 LLM 生成映射
//     d. 生成结果按 BatchSize 批量写入 mapping 表（upsert）
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
	preparedFields := make([]PreparedField, 0, len(req.Fields))
	seen := make(map[string]struct{}, len(req.Fields))
	for _, field := range req.Fields {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := field.Name
		if _, ok := seen[name]; ok {
			return fmt.Errorf("duplicate alignment field %q", name)
		}
		seen[name] = struct{}{}
		targetSetID := field.TargetSetID
		if targetSetID == "" {
			targetSetID = name
		}
		targetDefs, err := s.repo.LoadTargetLabels(ctx, targetSetID)
		if err != nil {
			return err
		}
		targets := make([]string, 0, len(targetDefs))
		for _, target := range targetDefs {
			targets = append(targets, target.Label)
		}
		prepared, err := PrepareField(field, targets, s.defaultBatchSize)
		if err != nil {
			return err
		}
		preparedFields = append(preparedFields, prepared)
	}
	sourceColumns, err := s.repo.EnsureExecutionReady(ctx, req, preparedFields)
	if err != nil {
		return err
	}
	fuzzyTopK := req.FuzzyTopK
	if fuzzyTopK <= 0 {
		fuzzyTopK = DefaultFuzzyTopK
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
		existing, err := s.repo.LoadExistingMappings(ctx, req, field, rawValues)
		if err != nil {
			return err
		}
		if len(existing) > 0 {
			if err := s.repo.TouchMappings(ctx, req, field, existing); err != nil {
				return err
			}
		}
		missing := MissingRawValues(rawValues, existing)
		pending := make([]MappingRecord, 0, field.BatchSize)
		for _, rawValue := range missing {
			recalledTargets, err := s.repo.RecallTopKTargets(
				ctx,
				field.TargetSetID,
				rawValue,
				fuzzyTopK,
			)
			if err != nil {
				return err
			}
			if len(recalledTargets) == 0 {
				pending = append(pending, MappingRecord{
					RawValue: rawValue,
					Status:   MappingStatusNeedsCandidate,
				})
				if len(pending) < field.BatchSize {
					continue
				}
				if err := s.flushGeneratedMappings(ctx, req, field, pending); err != nil {
					return err
				}
				pending = pending[:0]
				continue
			}
			recalledField, err := PrepareRecalledField(field, recalledTargets)
			if err != nil {
				return err
			}
			generated, err := GenerateMapping(ctx, llmClient, recalledField, rawValue)
			if err != nil {
				return err
			}
			pending = append(pending, generated)
			if len(pending) < field.BatchSize {
				continue
			}
			if err := s.flushGeneratedMappings(ctx, req, field, pending); err != nil {
				return err
			}
			pending = pending[:0]
		}
		if err := s.flushGeneratedMappings(ctx, req, field, pending); err != nil {
			return err
		}
	}
	if err := s.repo.WriteOutputRows(ctx, req, sourceColumns, preparedFields); err != nil {
		return err
	}
	processRecords, err := s.repo.BuildSourceRangeProcessRecords(ctx, req, preparedFields)
	if err != nil {
		return err
	}
	return s.processRecorder.UpsertMany(ctx, processRecords)
}

func (s *DomainService) flushGeneratedMappings(
	ctx context.Context,
	req ExecuteRequest,
	field PreparedField,
	records []MappingRecord,
) error {
	if len(records) == 0 {
		return nil
	}
	if err := s.repo.UpsertTargetCandidates(ctx, field.TargetSetID, records); err != nil {
		return err
	}
	return s.repo.UpsertMappings(ctx, req, field, records)
}
