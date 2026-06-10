// executor.go 提供 enrichextract 后台执行适配层。
package enrichextract

import (
	"context"
	"fmt"
)

type executor struct {
	jobRepo     Repository
	llmFactory  LLMFactory
	dbOpener    BusinessDBOpener
	repoFactory BusinessRepositoryFactory
}

func NewExecutor(
	jobRepo Repository,
	llmFactory LLMFactory,
	dbOpener BusinessDBOpener,
	repoFactory BusinessRepositoryFactory,
) Executor {
	return &executor{jobRepo: jobRepo, llmFactory: llmFactory, dbOpener: dbOpener, repoFactory: repoFactory}
}

func (e *executor) Execute(
	ctx context.Context,
	job *Job,
	req StartRequest,
	resources ResourceReader,
) error {
	if e == nil || e.llmFactory == nil || e.dbOpener == nil || e.repoFactory == nil {
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
	domainSvc := NewDomainService(e.repoFactory(bizDB), e.jobRepo, job.JobID)
	targetExample := map[string]any{}
	if len(job.TargetExample) > 0 {
		targetExample = job.TargetExample[0]
	}
	return domainSvc.Execute(ctx, llmClient, ExecuteRequest{
		SourceTable:     req.SourceTable,
		OutputTable:     req.OutputTable,
		KeyField:        req.KeyField,
		SourceJSONField: req.SourceJSONField,
		TargetFields:    job.TargetFields,
		TargetExample:   targetExample,
		StartID:         req.StartID,
		EndID:           req.EndID,
		Overwrite:       job.Overwrite,
		Concurrency:     job.Concurrency,
		PageSize:        job.PageSize,
		MaxRetries:      job.MaxRetries,
		LastKey:         job.LastKey,
	})
}
