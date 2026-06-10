// models.go 定义结构化抽取模块的核心数据模型。
package enrichextract

import "encoding/json"

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"

	DefaultSourceJSONField = "raw_data"
	DefaultConcurrency     = 1
	DefaultMaxRetries      = 2
	MinPageSize            = 100
	MaxPageSize            = 1000
	MaxConcurrency         = 128
	MaxRetries             = 5
	WriteBatchSize         = 100
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

type SourceRow struct {
	Key int64
	Raw json.RawMessage
}

type OutputRow struct {
	Key    int64
	Values map[string]any
}

type RowError struct {
	SourceKey    int64
	Stage        string
	Attempts     int
	ErrorMessage string
}

type PageResult struct {
	Successes []OutputRow
	Errors    []RowError
}

type ProgressUpdate struct {
	LastKey       int64
	ProcessedRows int64
	SucceededRows int64
	FailedRows    int64
}

type ColumnMeta struct {
	Name          string
	UDTName       string
	FormattedType string
}
