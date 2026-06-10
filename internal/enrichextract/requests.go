// requests.go 定义 enrichextract 模块各层请求/响应模型。
package enrichextract

type StartRequest struct {
	LLMResourceID      string
	DatabaseResourceID string
	SourceTable        string
	OutputTable        string
	KeyField           string
	SourceJSONField    string
	TargetExample      []map[string]any
	StartID            *int64
	EndID              *int64
	Concurrency        *int
	Overwrite          *bool
	PageSize           *int
	MaxRetries         *int
}

type StartResult struct {
	JobID  string
	Status string
}

type ExecuteRequest struct {
	SourceTable     string
	OutputTable     string
	KeyField        string
	SourceJSONField string
	TargetFields    []string
	TargetExample   map[string]any
	StartID         *int64
	EndID           *int64
	Overwrite       bool
	Concurrency     int
	PageSize        int
	MaxRetries      int
	LastKey         *int64
}
