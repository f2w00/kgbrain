package agents

import (
	"fmt"
	"sort"
	"sync"
)

type Registry struct {
	mu     sync.RWMutex
	agents map[string]Agent
}

func NewRegistry() *Registry {
	return &Registry{agents: make(map[string]Agent)}
}

func (r *Registry) Register(a Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := a.Name()
	if _, exists := r.agents[name]; exists {
		return fmt.Errorf("agent %q already registered", name)
	}
	r.agents[name] = a
	return nil
}

func (r *Registry) Get(name string) (Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.agents[name]
	if !ok {
		return nil, fmt.Errorf("agent %q not found", name)
	}
	return a, nil
}

func (r *Registry) List() []Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.agents))
	for n := range r.agents {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]Agent, len(names))
	for i, n := range names {
		out[i] = r.agents[n]
	}
	return out
}
