// executor.go 提供 enrichextract 后台执行适配层，串联 Resource、LLM、业务数据库和领域服务。
package enrichextract

import (
	"context"
	"fmt"
)

type executor struct {
	jobRepo                Repository
	llmFactory             LLMFactory
	dbOpener               BusinessDBOpener
	repoFactory            BusinessRepositoryFactory
	processRecorderFactory ProcessRecorderFactory
}

// NewExecutor 创建后台执行器，注入依赖工厂。
func NewExecutor(
	jobRepo Repository,
	llmFactory LLMFactory,
	dbOpener BusinessDBOpener,
	repoFactory BusinessRepositoryFactory,
	processRecorderFactory ProcessRecorderFactory,
) Executor {
	return &executor{
		jobRepo:                jobRepo,
		llmFactory:             llmFactory,
		dbOpener:               dbOpener,
		repoFactory:            repoFactory,
		processRecorderFactory: processRecorderFactory,
	}
}

// Execute 执行 job：获取 LLM/数据库资源 → 创建客户端 → 打开连接 → 委托 DomainService 处理。
func (e *executor) Execute(
	ctx context.Context,
	job *Job,
	req StartRequest,
	resources ResourceReader,
) error {
	if e == nil || e.llmFactory == nil || e.dbOpener == nil ||
		e.repoFactory == nil || e.processRecorderFactory == nil {
		return fmt.Errorf("enrich extract executor dependencies are not configured")
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
	domainSvc := NewDomainService(e.repoFactory(bizDB), e.jobRepo, job.JobID, processRecorder)
	targetExample := map[string]any{}
	if len(job.TargetExample) > 0 {
		targetExample = job.TargetExample[0]
	}
	return domainSvc.Execute(ctx, llmClient, ExecuteRequest{
		SourceTable:        req.SourceTable,
		OutputTable:        req.OutputTable,
		KeyField:           req.KeyField,
		SourceJSONField:    req.SourceJSONField,
		TargetFields:       job.TargetFields,
		TargetExample:      targetExample,
		PriorityFieldHints: cloneStringMap(job.PriorityFieldHints),
		StartID:            req.StartID,
		EndID:              req.EndID,
		Overwrite:          job.Overwrite,
		Concurrency:        job.Concurrency,
		PageSize:           job.PageSize,
		MaxRetries:         job.MaxRetries,
		LastKey:            job.LastKey,
	})
}
