// executor.go 提供实体对齐后台执行适配层，串联资源依赖和领域执行。
package alignment

import (
	"context"
	"fmt"
)

type executor struct {
	defaultBatchSize       int
	llmFactory             LLMFactory
	dbOpener               BusinessDBOpener
	repoFactory            BusinessRepositoryFactory
	processRecorderFactory ProcessRecorderFactory
}

// NewExecutor 创建实体对齐应用层执行器。
func NewExecutor(
	defaultBatchSize int,
	llmFactory LLMFactory,
	dbOpener BusinessDBOpener,
	repoFactory BusinessRepositoryFactory,
	processRecorderFactory ProcessRecorderFactory,
) Executor {
	return &executor{
		defaultBatchSize:       defaultBatchSize,
		llmFactory:             llmFactory,
		dbOpener:               dbOpener,
		repoFactory:            repoFactory,
		processRecorderFactory: processRecorderFactory,
	}
}

// Execute 执行实体对齐 job。
//
// 职责：
//  1. 根据 resource_id 获取 LLM 客户端和业务数据库连接
//  2. 创建领域层 Service，委托其执行对齐流程
//  3. 所有数据库操作均通过 BusinessRepository 在业务库上完成
func (e *executor) Execute(
	ctx context.Context,
	job *Job,
	req StartRequest,
	resources ResourceReader,
) error {
	if e == nil || e.llmFactory == nil ||
		e.dbOpener == nil || e.repoFactory == nil || e.processRecorderFactory == nil {
		return fmt.Errorf("entity alignment executor dependencies are not configured")
	}
	llmResource, err := resources.GetLLM(job.LLMResourceID)
	if err != nil {
		return fmt.Errorf("get llm resource: %w", err)
	}
	dbResource, err := resources.GetDatabase(job.DatabaseResourceID)
	if err != nil {
		return fmt.Errorf("get database resource: %w", err)
	}
	llmClient, err := e.llmFactory(llmResource)
	if err != nil {
		return fmt.Errorf("create llm client: %w", err)
	}
	bizDB, err := e.dbOpener(dbResource)
	if err != nil {
		return fmt.Errorf("open business database: %w", err)
	}
	defer bizDB.Close()
	processRecorder, err := e.processRecorderFactory(bizDB)
	if err != nil {
		return fmt.Errorf("create process recorder: %w", err)
	}
	domainSvc := NewDomainService(e.repoFactory(bizDB), e.defaultBatchSize, processRecorder)
	return domainSvc.Execute(ctx, llmClient, ExecuteRequest{
		SourceTable:             req.SourceTable,
		OutputTable:             req.OutputTable,
		KeyField:                req.KeyField,
		StartID:                 req.StartID,
		EndID:                   req.EndID,
		OnlyWaitingTargetReview: job.OnlyWaitingTargetReview,
		FuzzyTopK:               effectiveFuzzyTopK(job.FuzzyTopK),
		Fields:                  job.Fields,
	})
}

func effectiveFuzzyTopK(v int) int {
	if v <= 0 {
		return DefaultFuzzyTopK
	}
	return v
}
