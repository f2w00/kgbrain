package task

import (
	"encoding/json"
	"sync"
	"time"

	"kgbrain/internal/logger"

	"go.uber.org/zap"
)

// Status 表示任务生命周期状态.
const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type Status string

// Task 表示一次 Agent 链式执行任务, 包含完整的状态机.
// SessionID 关联回发起任务的 session, 供 hook (如通知) 查询 session 配置.
type Task struct {
	mu        sync.RWMutex
	ID        string    `json:"task_id"`
	SessionID string    `json:"session_id"`
	Status    Status    `json:"status"`
	Progress  int       `json:"progress"`
	Chain     []string  `json:"chain"`
	Results   map[string]any `json:"results,omitempty"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

// New 创建一个状态为 queued 的任务.
func New(id string, chain []string) *Task {
	return &Task{
		ID:        id,
		Status:    StatusQueued,
		Chain:     chain,
		Results:   make(map[string]any),
		CreatedAt: time.Now(),
	}
}

// MarkRunning 将任务状态设为 running, 记录开始时间.
func (t *Task) MarkRunning() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Status = StatusRunning
	now := time.Now()
	t.StartedAt = &now

	logger.L().Debug("task marked running", zap.String("task_id", t.ID))
}

// SetProgress 设置进度百分比 (0-100), 超出边界自动修正.
func (t *Task) SetProgress(p int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	t.Progress = p
}

// SetAgentResult 保存某个 agent 的执行结果到任务.
func (t *Task) SetAgentResult(name string, result any) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Results[name] = result
}

// MarkCompleted 将任务状态设为 completed, 进度设为 100.
func (t *Task) MarkCompleted() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Status = StatusCompleted
	t.Progress = 100
	now := time.Now()
	t.EndedAt = &now

	logger.L().Info("task completed",
		zap.String("task_id", t.ID),
		zap.Duration("duration", t.EndedAt.Sub(*t.StartedAt)),
	)
}

// MarkFailed 将任务状态设为 failed, 记录错误信息.
func (t *Task) MarkFailed(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Status = StatusFailed
	t.Error = err.Error()
	now := time.Now()
	t.EndedAt = &now

	logger.L().Error("task failed",
		zap.String("task_id", t.ID),
		zap.String("error", t.Error),
	)
}

// Snapshot 返回当前任务的线程安全快照, 用于外部读取和持久化.
func (t *Task) Snapshot() TaskSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return TaskSnapshot{
		ID:        t.ID,
		SessionID: t.SessionID,
		Status:    t.Status,
		Progress:  t.Progress,
		Chain:     t.Chain,
		Results:   copyMap(t.Results),
		Error:     t.Error,
		CreatedAt: t.CreatedAt,
		StartedAt: t.StartedAt,
		EndedAt:   t.EndedAt,
	}
}

// TaskSnapshot 是 Task 的只读快照, 不含锁, 可安全传输.
type TaskSnapshot struct {
	ID        string     `json:"task_id"`
	SessionID string     `json:"session_id"`
	Status    Status     `json:"status"`
	Progress  int        `json:"progress"`
	Chain     []string   `json:"chain"`
	Results   map[string]any `json:"results,omitempty"`
	Error     string     `json:"error,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

// copyMap 深拷贝 map[string]any, 避免快照与原始 Task 共享内存.
func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		data, _ := json.Marshal(v)
		var val any
		_ = json.Unmarshal(data, &val)
		out[k] = val
	}
	return out
}
