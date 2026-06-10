// contracts.go 定义 enrichextract feature 对外依赖接口和核心抽象。
package enrichextract

import (
	"context"
	"database/sql"

	"github.com/cloudwego/eino/schema"

	"kgbrain/internal/resource"
)

type ResourceReader interface {
	GetLLM(id string) (*resource.LLMResource, error)
	GetDatabase(id string) (*resource.DatabaseResource, error)
}

type LLMClient interface {
	GenerateXformMessages(ctx context.Context, msgs []*schema.Message) (string, error)
}

type LLMFactory func(r *resource.LLMResource) (LLMClient, error)

type BusinessDBOpener func(r *resource.DatabaseResource) (*sql.DB, error)

type Executor interface {
	Execute(ctx context.Context, job *Job, req StartRequest, resources ResourceReader) error
}

type Repository interface {
	CreateJob(job *Job) error
	GetJob(jobID string) (*Job, error)
	ListActiveJobs() ([]*Job, error)
	MarkRunning(jobID string) error
	MarkSucceeded(jobID string) error
	MarkFailed(jobID string, errorMessage string) error
	UpdateProgress(jobID string, update ProgressUpdate) error
	AddErrors(jobID string, errors []RowError) error
}

type BusinessRepository interface {
	EnsureExecutionReady(ctx context.Context, req ExecuteRequest) error
	SelectSourcePage(ctx context.Context, req ExecuteRequest, lastKey *int64) ([]SourceRow, error)
	BatchWriteOutputRows(ctx context.Context, req ExecuteRequest, rows []OutputRow) error
	WriteOutputRow(ctx context.Context, req ExecuteRequest, row OutputRow) error
}

type BusinessRepositoryFactory func(db *sql.DB) BusinessRepository
