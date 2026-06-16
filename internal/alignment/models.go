// models.go 定义实体对齐模块的核心数据模型。
package alignment

const (
	// StatusPending 表示任务已创建但后台执行尚未开始。
	StatusPending = "pending"
	// StatusRunning 表示后台任务正在执行。
	StatusRunning = "running"
	// StatusSucceeded 表示后台任务已经成功结束。
	StatusSucceeded = "succeeded"
	// StatusFailed 表示后台任务执行失败。
	StatusFailed = "failed"
	// ProcessTypeEntityAlignment 表示实体对齐处理类型。
	ProcessTypeEntityAlignment = "entity_alignment"
)

// Job 记录一次实体对齐异步任务的基础状态。
type Job struct {
	JobID                   string
	LLMResourceID           string
	DatabaseResourceID      string
	SourceTable             string
	OutputTable             string
	Status                  string
	ReuseMapping            bool
	KeyField                string
	StartID                 *int64
	EndID                   *int64
	OnlyWaitingTargetReview bool
	Fields                  []FieldConfig
	CreatedAt               string
	StartedAt               string
	FinishedAt              string
	ErrorMessage            string
}

// FieldConfig 保存 job 启动时的字段配置快照，供后台执行时恢复请求上下文。
type FieldConfig struct {
	Name             string `json:"name"`
	TargetSetID      string `json:"target_set_id,omitempty"`
	BatchSize        *int   `json:"batch_size,omitempty"`
	BatchConcurrency *int   `json:"batch_concurrency,omitempty"`
}

// ColumnMeta 描述业务表中一个列的最小元数据。
type ColumnMeta struct {
	Name          string
	UDTName       string
	FormattedType string
}

// PreparedField 保存领域层整理后的字段处理上下文。
type PreparedField struct {
	Name        string
	TargetSetID string
	Targets     []string
	TargetsJSON string
	TargetHash  string
	BatchSize   int
}

// MappingRecord 表示一条最终可复用 mapping。
type MappingRecord struct {
	RawValue     string
	AlignedValue *string
	Status       string
}

type TargetDefinition struct {
	TargetSetID string
	Label       string
	Description string
}

type TargetCandidate struct {
	ID            string
	TargetSetID   string
	RawValue      string
	Frequency     int64
	Status        string
	Resolution    string
	ResolvedLabel string
	ReviewReason  string
	CreatedAt     string
	UpdatedAt     string
}

type ReviewCandidateAction struct {
	CandidateID  string
	Resolution   string
	Label        string
	ReviewReason string
}

type llmMappingRecord struct {
	RawValue     string  `json:"raw_value"`
	Status       string  `json:"status"`
	AlignedValue *string `json:"aligned_value"`
}
