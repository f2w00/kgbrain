package xform

// types.go 定义了 xform 模块的所有请求/响应 DTO.
// 按 RPC 方法分组:
//  submit  → SubmitRequest / SubmitResponse
//  append  → AppendRequest / AppendResponse
//  close   → CloseRequest / CloseResponse
//  status  → GetStatusRequest / GetStatusResponse
//  result  → GetResultRequest / GetResultResponse
//  delete  → DeleteRequest / DeleteResponse

import (
	"fmt"

	domain "kgbrain/internal/domain/xform"
)

// ── xform.submit ──

// SubmitRequest 是 xform.submit 的请求参数.
// task_id 可选: 用户不传则服务端自动生成 (idgen.GenerateTaskID).
type SubmitRequest struct {
	TaskID        string           `json:"task_id,omitempty"`
	ProfileID     string           `json:"profile_id"`
	TargetsExample []map[string]any `json:"targets_example"`
	PrimaryKey    string           `json:"primary_key"`
	PoolSize      int              `json:"pool_size"`
	MaxRetries    int              `json:"max_retries"`
	TTLHours      int              `json:"ttl_hours"`
}

// SubmitResponse 是 xform.submit 的响应.
type SubmitResponse struct {
	TaskID string `json:"task_id"`
}

// ── xform.append ──

// AppendRequest 用于向已创建的任务追加源数据.
// 每行数据必须包含 primary_key 指定的字段.
type AppendRequest struct {
	TaskID string           `json:"task_id"`
	Data   []map[string]any `json:"data"`
}

// AppendResponse 返回成功追加的行数.
type AppendResponse struct {
	Appended int `json:"appended"`
}

// ── xform.close ──

// CloseRequest 用于关闭任务, 停止接收新数据.
type CloseRequest struct {
	TaskID string `json:"task_id"`
}

// CloseResponse 返回关闭后的任务状态 (应为 "closing").
type CloseResponse struct {
	Status string `json:"status"`
}

// ── xform.get_status ──

// GetStatusRequest 查询单个任务的状态和队列长度.
type GetStatusRequest struct {
	TaskID string `json:"task_id"`
}

// GetStatusResponse 返回任务状态及三个队列的待处理行数.
type GetStatusResponse struct {
	TaskID        string          `json:"task_id"`
	Status        domain.TaskStatus `json:"status"`
	PendingInput  int             `json:"pending_input"`   // Stream 中未消费消息数 (XLEN)
	PendingOutput int             `json:"pending_output"`  // 结果队列待拉取行数 (LLEN)
	PendingErrors int             `json:"pending_errors"`  // 错误队列待拉取行数 (LLEN)
}

// ── xform.get_result ──

// GetResultRequest 拉取结果 (消费即删).
type GetResultRequest struct {
	TaskID string `json:"task_id"`
	Limit  int    `json:"limit"` // 默认 100
}

// GetResultResponse 返回拉取的 rows + 队列中剩余的 rows.
type GetResultResponse struct {
	TaskID        string             `json:"task_id"`
	Status        domain.TaskStatus  `json:"status"`
	Results       []map[string]any   `json:"results"`              // 本次拉到的成功结果
	Errors        []domain.ErrorEntry `json:"errors"`              // 本次拉到的错误
	PendingOutput int                `json:"pending_output"`       // 拉完后结果队列还剩多少
	PendingErrors int                `json:"pending_errors"`       // 拉完后错误队列还剩多少
}

// ── xform.delete ──

// DeleteRequest 用于强制删除任务.
type DeleteRequest struct {
	TaskID string `json:"task_id"`
}

// DeleteResponse 确认是否删除成功.
type DeleteResponse struct {
	Deleted bool `json:"deleted"`
}

// Validate 校验 SubmitRequest 参数.
// 校验规则:
//  - task_id 非空时: 长度 ≤ 64, 仅允许 [a-zA-Z0-9_-]
//  - profile_id / primary_key 必填
//  - targets_example 非空
//  - pool_size > 0
func (r *SubmitRequest) Validate() error {
	if r.TaskID != "" {
		if len(r.TaskID) > 64 {
			return fmt.Errorf("task_id exceeds max length 64")
		}
		for _, c := range r.TaskID {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
				(c >= '0' && c <= '9') || c == '_' || c == '-') {
				return fmt.Errorf("task_id contains invalid character: %c", c)
			}
		}
	}
	if r.ProfileID == "" {
		return fmt.Errorf("profile_id is required")
	}
	if len(r.TargetsExample) == 0 {
		return fmt.Errorf("targets_example is required")
	}
	if r.PrimaryKey == "" {
		return fmt.Errorf("primary_key is required")
	}
	if r.PoolSize <= 0 {
		return fmt.Errorf("pool_size must be greater than 0")
	}
	if r.MaxRetries < 0 {
		return fmt.Errorf("max_retries must be non-negative")
	}
	if r.TTLHours <= 0 {
		return fmt.Errorf("ttl_hours must be greater than 0")
	}
	return nil
}
