// contracts.go 定义实体对齐 feature 对外依赖接口和核心抽象。
package alignment

import (
	"context"
	"database/sql"

	"kgbrain/internal/processrecord"
	"kgbrain/internal/resource"
)

const (
	// DefaultBatchSize 控制单次批量写入 mapping/candidate 表的默认记录数。
	DefaultBatchSize = 20
	// DefaultBatchConcurrency 已禁用，LLM 固定单值串行处理。
	DefaultBatchConcurrency = 1
)

// ResourceReader 抽象资源读取能力，便于实体对齐服务校验依赖并执行任务。
type ResourceReader interface {
	GetLLM(id string) (*resource.LLMResource, error)
	GetDatabase(id string) (*resource.DatabaseResource, error)
}

// LLMClient 抽象实体对齐执行阶段所需的最小 LLM 能力。
type LLMClient interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// LLMFactory 根据资源配置创建实体对齐使用的 LLM 客户端。
type LLMFactory func(r *resource.LLMResource) (LLMClient, error)

// BusinessDBOpener 根据数据库资源配置打开业务 Postgres 连接。
type BusinessDBOpener func(r *resource.DatabaseResource) (*sql.DB, error)

// Executor 抽象实体对齐后台执行逻辑，便于测试中注入轻量实现。
type Executor interface {
	Execute(
		ctx context.Context,
		job *Job,
		req StartRequest,
		resources ResourceReader,
	) error
}

// Repository 定义实体对齐 job 在服务本地 SQLite 中的持久化能力。
type Repository interface {
	CreateJob(job *Job) error
	GetJob(jobID string) (*Job, error)
	MarkRunning(jobID string) error
	MarkSucceeded(jobID string) error
	MarkFailed(jobID string, errorMessage string) error
}

// BusinessRepository 抽象业务 Postgres 上的表结构与读写能力。
type BusinessRepository interface {
	EnsureExecutionReady(
		ctx context.Context,
		req ExecuteRequest,
		fields []PreparedField,
	) ([]ColumnMeta, error)
	SelectDistinctRawValues(
		ctx context.Context,
		req ExecuteRequest,
		fieldName string,
	) ([]string, error)
	LoadExistingMappings(
		ctx context.Context,
		req ExecuteRequest,
		field PreparedField,
		rawValues []string,
	) (map[string]MappingRecord, error)
	TouchMappings(
		ctx context.Context,
		req ExecuteRequest,
		field PreparedField,
		records map[string]MappingRecord,
	) error
	UpsertMappings(
		ctx context.Context,
		req ExecuteRequest,
		field PreparedField,
		records []MappingRecord,
	) error
	LoadTargetLabels(
		ctx context.Context,
		targetSetID string,
	) ([]TargetDefinition, error)
	RecallTopKTargets(
		ctx context.Context,
		targetSetID string,
		rawValue string,
		topK int,
	) ([]string, error)
	UpsertTargets(
		ctx context.Context,
		targetSetID string,
		targets []TargetDefinition,
	) error
	DeleteTarget(
		ctx context.Context,
		targetSetID string,
		label string,
	) error
	UpsertTargetCandidates(
		ctx context.Context,
		targetSetID string,
		records []MappingRecord,
	) error
	ListTargetCandidates(
		ctx context.Context,
		targetSetID string,
		status string,
	) ([]TargetCandidate, error)
	ReviewTargetCandidates(
		ctx context.Context,
		sourceTable string,
		targetSetID string,
		actions []ReviewCandidateAction,
	) error
	BuildSourceRangeProcessRecords(
		ctx context.Context,
		req ExecuteRequest,
		fields []PreparedField,
	) ([]processrecord.Record, error)
	WriteOutputRows(
		ctx context.Context,
		req ExecuteRequest,
		sourceColumns []ColumnMeta,
		fields []PreparedField,
	) error
}

// ProcessRecorder 记录业务数据在实体对齐处理上的最终状态。
type ProcessRecorder interface {
	UpsertMany(ctx context.Context, records []processrecord.Record) error
	UpsertSourceRange(ctx context.Context, record processrecord.SourceRangeRecord) error
}

// ProcessRecorderFactory 基于业务库连接创建处理状态记录器。
type ProcessRecorderFactory func(db *sql.DB) (ProcessRecorder, error)
