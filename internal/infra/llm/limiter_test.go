package llm

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestResourceLimiterManagerSharesSlotsByResourceID(t *testing.T) {
	mgr := &resourceLimiterManager{entries: make(map[string]*limiterEntry)}
	limit := 1

	release1, err := mgr.Acquire(context.Background(), "kg-assist", &limit)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer release1()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if _, err := mgr.Acquire(ctx, "kg-assist", &limit); err == nil {
		t.Fatal("second acquire should wait and then timeout")
	}
}

func TestResourceLimiterManagerSeparatesDifferentResources(t *testing.T) {
	mgr := &resourceLimiterManager{entries: make(map[string]*limiterEntry)}
	limit := 1

	release1, err := mgr.Acquire(context.Background(), "kg-assist", &limit)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer release1()

	release2, err := mgr.Acquire(context.Background(), "kg-assist-2", &limit)
	if err != nil {
		t.Fatalf("second resource acquire: %v", err)
	}
	defer release2()
}

func TestResourceLimiterManagerReplacesEntryWhenLimitChanges(t *testing.T) {
	mgr := &resourceLimiterManager{entries: make(map[string]*limiterEntry)}
	one := 1
	two := 2

	release1, err := mgr.Acquire(context.Background(), "kg-assist", &one)
	if err != nil {
		t.Fatalf("acquire with limit 1: %v", err)
	}
	defer release1()

	var running int32
	var peak int32
	var wg sync.WaitGroup
	start := make(chan struct{})

	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			release, err := mgr.Acquire(
				context.Background(),
				"kg-assist",
				&two,
			)
			if err != nil {
				t.Errorf("acquire with limit 2: %v", err)
				return
			}
			cur := atomic.AddInt32(&running, 1)
			for {
				old := atomic.LoadInt32(&peak)
				if cur <= old || atomic.CompareAndSwapInt32(&peak, old, cur) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			atomic.AddInt32(&running, -1)
			release()
		}()
	}

	close(start)
	wg.Wait()

	if got := atomic.LoadInt32(&peak); got != 2 {
		t.Fatalf("expected peak concurrency 2 after limit update, got %d", got)
	}
}

func TestResourceLimiterManagerSkipsWhenLimitNil(t *testing.T) {
	mgr := &resourceLimiterManager{entries: make(map[string]*limiterEntry)}
	release, err := mgr.Acquire(context.Background(), "kg-assist", nil)
	if err != nil {
		t.Fatalf("acquire without limit: %v", err)
	}
	release()
}
