// Package processrecord 记录业务数据在各处理能力上的最终状态。
package processrecord

const (
	// StatusSucceeded 表示指定处理能力已成功完成。
	StatusSucceeded = "succeeded"
	// StatusFailed 表示指定处理能力最终处理失败。
	StatusFailed = "failed"
	// StatusWaitingTargetReview 表示指定处理能力需等待候选值审核后重跑。
	StatusWaitingTargetReview = "waiting_target_review"
)

// Record 表示一条业务数据在某个处理能力上的最终状态。
type Record struct {
	SourceTable string
	SourceKey   int64
	ProcessType string
	Status      string
}

// SourceRangeRecord 表示按源表主键范围批量登记处理状态的请求。
type SourceRangeRecord struct {
	SourceTable string
	KeyField    string
	StartID     *int64
	EndID       *int64
	ProcessType string
	Status      string
}
