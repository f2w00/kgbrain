package llm

import (
	"context"
	"sync"
)

// resourceLimiterManager 管理按 resource_id 共享的全局并发限制器。
// 旧入口不接入，只有显式传入 resource_id 的新功能会使用这里的限制。
type resourceLimiterManager struct {
	mu      sync.Mutex
	entries map[string]*limiterEntry
}

type limiterEntry struct {
	limit int
	slots chan struct{}
}

var globalResourceLimiter = &resourceLimiterManager{
	entries: make(map[string]*limiterEntry),
}

// Acquire 为指定 resource_id 获取一个并发槽位。
// limit=nil 时表示不限制，直接返回 no-op release。
func (m *resourceLimiterManager) Acquire(
	ctx context.Context,
	resourceID string,
	limit *int,
) (func(), error) {
	if limit == nil || resourceID == "" {
		return func() {}, nil
	}

	entry := m.getOrCreate(resourceID, *limit)
	select {
	case entry.slots <- struct{}{}:
		return func() {
			<-entry.slots
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *resourceLimiterManager) getOrCreate(
	resourceID string,
	limit int,
) *limiterEntry {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry, ok := m.entries[resourceID]; ok && entry.limit == limit {
		return entry
	}

	entry := &limiterEntry{
		limit: limit,
		slots: make(chan struct{}, limit),
	}
	m.entries[resourceID] = entry
	return entry
}
