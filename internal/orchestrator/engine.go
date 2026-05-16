// Package orchestrator 编排多 Agent 的链式执行任务生命周期.
// 支持同步 (ExecuteSync) 和异步 (Submit) 两种模式,
// 通过 Hook 机制在任务状态变更时触发飞书通知等副作用.
package orchestrator

import (
	"context"
	"fmt"
	"sync"

	"kgbrain/internal/agents"
	"kgbrain/internal/logger"
	"kgbrain/internal/orchestrator/task"
	"kgbrain/pkg/idgen"

	"go.uber.org/zap"
)

// Engine 编排多 Agent 的链式执行.
// 支持同步 (阻塞直到完成) 和异步 (goroutine + 任务队列) 两种模式.
// 通过 AddHook 注册的回调在任务状态变更时触发 (用于飞书通知等).
type Engine struct {
	store    *task.Store
	registry *agents.Registry
	taskChan chan *task.Task
	hooks    []TaskHook
	mu       sync.RWMutex
}

// TaskHook 在任务状态变更时被调用 (running → completed / failed).
type TaskHook func(t *task.Task)

func NewEngine(store *task.Store, registry *agents.Registry, workers int) *Engine {
	e := &Engine{
		store:    store,
		registry: registry,
		taskChan: make(chan *task.Task, 1024),
	}
	for range workers {
		go e.worker()
	}
	return e
}

func (e *Engine) AddHook(h TaskHook) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.hooks = append(e.hooks, h)
}

// Submit 创建任务后立即返回, 由 goroutine 异步执行.
// 调用方可通过 task.get_status / task.get_result 轮询进度.
func (e *Engine) Submit(chain []string, data map[string]any, opts map[string]any) (*task.Task, error) {
	t := task.New(idgen.GenerateTaskID(), chain)
	t.SessionID, _ = opts["session_id"].(string)
	e.store.Save(t)

	e.taskChan <- t

	go func() {
		_ = e.execute(context.Background(), t, data, opts)
	}()

	logger.L().Info("task submitted",
		zap.String("task_id", t.ID),
		zap.Strings("chain", chain),
	)
	return t, nil
}

// ExecuteSync 阻塞直到所有 agent 执行完毕或某个 agent 失败.
// 返回值包含所有 agent 的执行结果.
func (e *Engine) ExecuteSync(chain []string, data map[string]any, opts map[string]any) (*task.TaskSnapshot, error) {
	t := task.New(idgen.GenerateTaskID(), chain)
	t.SessionID, _ = opts["session_id"].(string)
	e.store.Save(t)

	logger.L().Info("task submitted (sync)",
		zap.String("task_id", t.ID),
		zap.Strings("chain", chain),
	)

	err := e.execute(context.Background(), t, data, opts)
	snap := t.Snapshot()
	return &snap, err
}

// execute 按链式顺序依次执行 agent, 前一个 agent 的输出作为后一个的输入.
func (e *Engine) execute(ctx context.Context, t *task.Task, data map[string]any, opts map[string]any) error {
	t.MarkRunning()
	e.fireHooks(t)

	for i, agentName := range t.Chain {
		select {
		case <-ctx.Done():
			t.MarkFailed(ctx.Err())
			e.fireHooks(t)
			return ctx.Err()
		default:
		}

		agent, err := e.registry.Get(agentName)
		if err != nil {
			t.MarkFailed(err)
			e.fireHooks(t)
			return err
		}

		progress := (i * 100) / len(t.Chain)
		t.SetProgress(progress)

		logger.L().Info("executing agent",
			zap.String("task_id", t.ID),
			zap.String("agent", agentName),
			zap.Int("step", i+1),
			zap.Int("total", len(t.Chain)),
		)

		result := agent.Execute(ctx, agents.Input{Data: data, Options: opts})
		t.SetAgentResult(agentName, map[string]any{
			"status":      result.Status,
			"data":        result.Data,
			"duration_ms": result.DurationMs,
		})

		if result.Status == "failed" {
			t.MarkFailed(fmt.Errorf("agent %q failed: %s", agentName, result.Error))
			e.fireHooks(t)
			return fmt.Errorf("agent %q failed", agentName)
		}

		if result.Data != nil {
			data = result.Data
		}
	}

	t.SetProgress(100)
	t.MarkCompleted()
	e.fireHooks(t)
	return nil
}

// worker 消费 taskChan, 任务实际由 Submit 的 goroutine 执行, worker 暂为空占位.
func (e *Engine) worker() {
	for range e.taskChan {
	}
}

// fireHooks 遍历所有已注册的 TaskHook 并依次调用.
func (e *Engine) fireHooks(t *task.Task) {
	e.mu.RLock()
	hooks := e.hooks
	e.mu.RUnlock()

	for _, h := range hooks {
		h(t)
	}
}

// GetTask 按 ID 查询任务快照.
func (e *Engine) GetTask(id string) (*task.TaskSnapshot, error) {
	t, err := e.store.Get(id)
	if err != nil {
		return nil, err
	}
	snap := t.Snapshot()
	return &snap, nil
}

// ListTasks 按创建时间倒序返回任务列表, 支持分页.
func (e *Engine) ListTasks(limit, offset int) []task.TaskSnapshot {
	return e.store.List(limit, offset)
}
