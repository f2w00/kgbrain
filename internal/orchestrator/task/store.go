package task

import (
	"fmt"
	"sync"
)

// Store 是线程安全的内存任务存储, 按插入顺序保留, 超上限时淘汰最旧任务.
type Store struct {
	mu    sync.RWMutex
	tasks map[string]*Task
	order []string
	max   int
}

// NewStore 创建内存存储, maxTasks 控制最大保留数.
func NewStore(maxTasks int) *Store {
	return &Store{
		tasks: make(map[string]*Task),
		max:   maxTasks,
	}
}

// Save 保存或更新任务. 达到上限时淘汰最早写入的任务.
func (s *Store) Save(t *Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.tasks) >= s.max && s.tasks[t.ID] == nil {
		oldest := s.order[0]
		delete(s.tasks, oldest)
		s.order = s.order[1:]
	}
	if s.tasks[t.ID] == nil {
		s.order = append(s.order, t.ID)
	}
	s.tasks[t.ID] = t
}

// Get 按 ID 查询任务.
func (s *Store) Get(id string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task %q not found", id)
	}
	return t, nil
}

// List 返回分页的任务快照列表, 按创建时间倒序.
func (s *Store) List(limit, offset int) []TaskSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if offset >= len(s.order) {
		return nil
	}
	end := min(offset+limit, len(s.order))
	ids := s.order[offset:end]
	out := make([]TaskSnapshot, len(ids))
	for i, id := range ids {
		out[len(ids)-1-i] = s.tasks[id].Snapshot()
	}
	return out
}

// Count 返回当前存储的任务数量.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tasks)
}
