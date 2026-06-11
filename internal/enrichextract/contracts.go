// contracts.go 定义 enrichextract feature 对外依赖接口和核心抽象。
package enrichextract

import (
	"context"
	"database/sql"

	"github.com/cloudwego/eino/schema"

	"kgbrain/internal/resource"
)

// ResourceReader 提供 enrichextract 读取 Resource 模块配置的能力。
type ResourceReader interface {
	GetLLM(id string) (*resource.LLMResource, error)
	GetDatabase(id string) (*resource.DatabaseResource, error)
}

// LLMClient 封装 LLM 会话级调用，一次请求多个消息并获取字符串响应。
type LLMClient interface {
	GenerateXformMessages(ctx context.Context, msgs []*schema.Message) (string, error)
}

// LLMFactory 根据 LLMResource 配置创建 LLMClient。
type LLMFactory func(r *resource.LLMResource) (LLMClient, error)

// BusinessDBOpener 根据 DatabaseResource 配置打开业务数据库连接。
type BusinessDBOpener func(r *resource.DatabaseResource) (*sql.DB, error)

// Executor 封装 enrichextract 后台执行的完整流程：连接资源、按页读取、逐行 LLM、批量写入。
type Executor interface {
	Execute(ctx context.Context, job *Job, req StartRequest, resources ResourceReader) error
}

// Repository 定义 enrichextract job 在本地 SQLite 中的持久化接口。
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

// BusinessRepository 定义业务 Postgres 侧的表结构校验、分页读取和批量写入接口。
type BusinessRepository interface {
	EnsureExecutionReady(ctx context.Context, req ExecuteRequest) error
	SelectSourcePage(ctx context.Context, req ExecuteRequest, lastKey *int64) ([]SourceRow, error)
	BatchWriteOutputRows(ctx context.Context, req ExecuteRequest, rows []OutputRow) error
	WriteOutputRow(ctx context.Context, req ExecuteRequest, row OutputRow) error
}

// BusinessRepositoryFactory 根据业务数据库连接创建 BusinessRepository。
type BusinessRepositoryFactory func(db *sql.DB) BusinessRepository
