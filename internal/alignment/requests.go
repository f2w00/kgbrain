// requests.go 定义实体对齐模块各层的请求/响应模型。
package alignment

// StartRequest 是应用层启动实体对齐任务的输入模型。
type StartRequest struct {
	LLMResourceID      string
	DatabaseResourceID string
	SourceTable        string
	OutputTable        string
	ReuseMapping       *bool
	KeyField           string
	StartID            *int64
	EndID              *int64
	Fields             []FieldRequest
}

// FieldRequest 描述单个待对齐字段及其 LLM 批处理配置。
type FieldRequest struct {
	Name             string
	Targets          []string
	BatchSize        *int
	BatchConcurrency *int
}

// StartResult 是 Start 调用创建 job 后返回给接口层的最小结果。
type StartResult struct {
	JobID  string
	Status string
}

// ExecuteRequest 描述一次实体对齐真实执行所需的完整业务输入。
type ExecuteRequest struct {
	SourceTable  string
	OutputTable  string
	KeyField     string
	StartID      *int64
	EndID        *int64
	ReuseMapping bool
	Fields       []FieldConfig
}
