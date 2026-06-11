// models.go 定义结构化抽取模块的核心数据模型。
package enrichextract

import "encoding/json"

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusPartial   = "partial"
	StatusFailed    = "failed"

	DefaultSourceJSONField   = "raw_data"
	DefaultConcurrency       = 1
	DefaultMaxRetries        = 2
	MinPageSize              = 100
	MaxPageSize              = 1000
	MaxConcurrency           = 128
	MaxRetries               = 5
	WriteBatchSize           = 100
	ProcessTypeEnrichExtract = "enrich_extract"
)

// Job 记录一次结构化抽取异步任务的参数快照、状态和进度。
type Job struct {
	JobID              string
	Status             string
	LLMResourceID      string
	DatabaseResourceID string
	SourceTable        string
	OutputTable        string
	KeyField           string
	SourceJSONField    string
	TargetExample      []map[string]any
	TargetFields       []string
	PriorityFieldHints map[string]string
	StartID            *int64
	EndID              *int64
	Overwrite          bool
	Concurrency        int
	PageSize           int
	MaxRetries         int
	LastKey            *int64
	ProcessedRows      int64
	SucceededRows      int64
	FailedRows         int64
	CreatedAt          string
	StartedAt          string
	UpdatedAt          string
	FinishedAt         string
	ErrorMessage       string
}

// SourceRow 表示从业务数据库读取的一行源数据。
type SourceRow struct {
	Key int64
	Raw json.RawMessage
}

// OutputRow 表示写入业务数据库的一行结果数据，包含主键和 LLM 输出的目标字段。
type OutputRow struct {
	Key    int64
	Values map[string]any
}

// RowError 记录单行处理中发生的错误。
type RowError struct {
	SourceKey    int64
	Stage        string
	Attempts     int
	ErrorMessage string
}

// PageResult 记录一页数据中所有成功和失败的行。
type PageResult struct {
	Successes []OutputRow
	Errors    []RowError
}

// ProgressUpdate 表示一次页面处理后需要更新的进度信息。
type ProgressUpdate struct {
	LastKey       int64
	ProcessedRows int64
	SucceededRows int64
	FailedRows    int64
}

// ColumnMeta 表示业务数据库中一张表的单列元信息。
type ColumnMeta struct {
	Name          string
	UDTName       string
	FormattedType string
}
