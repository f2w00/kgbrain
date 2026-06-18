// requests.go 定义 enrichextract 模块各层请求/响应模型。
package enrichextract

// StartRequest 是外部启动结构化抽取任务的请求参数。
type StartRequest struct {
	LLMResourceID         string
	DatabaseResourceID    string
	SourceTable           string
	OutputTable           string
	KeyField              string
	SourceJSONField       string
	OutputSchema          []OutputColumn
	TargetExample         map[string]any
	PriorityFieldHints    map[string]string
	AutoCreateOutputTable *bool
	StartID               *int64
	EndID                 *int64
	Concurrency           *int
	Overwrite             *bool
	PageSize              *int
	MaxRetries            *int
}

// StartResult 是启动任务的响应，包含新创建的 job ID 和初始状态。
type StartResult struct {
	JobID  string
	Status string
}

// ExecuteRequest 是领域执行服务所需的完整参数，由 StartRequest 归一化后产生。
type ExecuteRequest struct {
	SourceTable           string
	OutputTable           string
	KeyField              string
	SourceJSONField       string
	OutputSchema          []OutputColumn
	TargetExample         map[string]any
	PriorityFieldHints    map[string]string
	AutoCreateOutputTable bool
	StartID               *int64
	EndID                 *int64
	Overwrite             bool
	Concurrency           int
	PageSize              int
	MaxRetries            int
	LastKey               *int64
}
