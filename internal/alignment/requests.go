// requests.go 定义实体对齐模块各层的请求/响应模型。
package alignment

// StartRequest 是应用层启动实体对齐任务的输入模型。
type StartRequest struct {
	LLMResourceID           string
	DatabaseResourceID      string
	SourceTable             string
	OutputTable             string
	KeyField                string
	StartID                 *int64
	EndID                   *int64
	OnlyWaitingTargetReview bool
	FuzzyTopK               *int
	Fields                  []FieldRequest
}

// FieldRequest 描述单个待对齐字段及其 mapping 写入配置。
type FieldRequest struct {
	Name             string
	TargetSetID      string
	BatchSize        *int
	BatchConcurrency *int
}

type ListTargetsRequest struct {
	DatabaseResourceID string
	TargetSetID        string
}

type UpsertTargetsRequest struct {
	DatabaseResourceID string
	TargetSetID        string
	Labels             []string
}

type DeleteTargetRequest struct {
	DatabaseResourceID string
	TargetSetID        string
	Label              string
}

type ListCandidatesRequest struct {
	DatabaseResourceID string
	TargetSetID        string
	Status             string
}

type ReviewCandidatesRequest struct {
	DatabaseResourceID string
	TargetSetID        string
	SourceTable        string
	Actions            []ReviewCandidateAction
}

// StartResult 是 Start 调用创建 job 后返回给接口层的最小结果。
type StartResult struct {
	JobID  string
	Status string
}

type ListTargetsResult struct {
	Labels []string
}

type ListCandidatesResult struct {
	Candidates []TargetCandidate
}

// ExecuteRequest 描述一次实体对齐真实执行所需的完整业务输入。
type ExecuteRequest struct {
	SourceTable             string
	OutputTable             string
	KeyField                string
	StartID                 *int64
	EndID                   *int64
	OnlyWaitingTargetReview bool
	FuzzyTopK               int
	Fields                  []FieldConfig
}
