package alignment

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"kgbrain/internal/resource"
)

type fakeRepo struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{jobs: make(map[string]*Job)}
}

func (r *fakeRepo) CreateJob(job *Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copyJob := *job
	r.jobs[job.JobID] = &copyJob
	return nil
}

func (r *fakeRepo) GetJob(jobID string) (*Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	job := r.jobs[jobID]
	if job == nil {
		return nil, nil
	}
	copyJob := *job
	return &copyJob, nil
}

func (r *fakeRepo) MarkRunning(jobID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[jobID].Status = StatusRunning
	r.jobs[jobID].StartedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

func (r *fakeRepo) MarkSucceeded(jobID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[jobID].Status = StatusSucceeded
	r.jobs[jobID].FinishedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

func (r *fakeRepo) MarkFailed(jobID string, errorMessage string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[jobID].Status = StatusFailed
	r.jobs[jobID].ErrorMessage = errorMessage
	r.jobs[jobID].FinishedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

type fakeResources struct{}

type fakeExecutor struct {
	err    error
	called chan StartRequest
}

func (fakeResources) GetLLM(id string) (*resource.LLMResource, error) {
	return &resource.LLMResource{ID: id}, nil
}

func (fakeResources) GetDatabase(id string) (*resource.DatabaseResource, error) {
	return &resource.DatabaseResource{ID: id}, nil
}

func (e *fakeExecutor) Execute(
	_ context.Context,
	_ *Job,
	req StartRequest,
	_ ResourceReader,
) error {
	if e.called != nil {
		e.called <- req
	}
	return e.err
}

func validStartRequest() StartRequest {
	return StartRequest{
		LLMResourceID:      "llm_1",
		DatabaseResourceID: "db_1",
		SourceTable:        "public.source",
		OutputTable:        "public.output",
		KeyField:           "id",
		Fields:             []FieldRequest{{Name: "dynasty", Targets: []string{"唐", "宋"}}},
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}

func TestServiceStartCreatesAndRunsJob(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, fakeResources{})
	req := validStartRequest()
	req.StartID = int64Ptr(100)
	req.EndID = int64Ptr(200)

	result, err := svc.Start(context.Background(), req)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if result.JobID == "" || result.Status != StatusPending {
		t.Fatalf("unexpected start result: %#v", result)
	}

	var job *Job
	for i := 0; i < 20; i++ {
		job, err = svc.GetJob(result.JobID)
		if err != nil {
			t.Fatalf("get job: %v", err)
		}
		if job.Status == StatusSucceeded {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if job == nil || job.Status != StatusSucceeded {
		t.Fatalf("job did not succeed: %#v", job)
	}
	if job.StartID == nil || *job.StartID != 100 {
		t.Fatalf("unexpected job start id: %#v", job)
	}
	if job.EndID == nil || *job.EndID != 200 {
		t.Fatalf("unexpected job end id: %#v", job)
	}
	if len(job.Fields) != 1 || job.Fields[0].Name != "dynasty" {
		t.Fatalf("unexpected job fields: %#v", job)
	}
}

func TestServiceRunJobUsesExecutor(t *testing.T) {
	repo := newFakeRepo()
	executor := &fakeExecutor{called: make(chan StartRequest, 1)}
	svc := NewService(repo, fakeResources{}, WithExecutor(executor))
	req := validStartRequest()
	req.StartID = int64Ptr(10)
	req.EndID = int64Ptr(20)

	result, err := svc.Start(context.Background(), req)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	select {
	case calledReq := <-executor.called:
		if len(calledReq.Fields) != 1 || calledReq.Fields[0].Name != "dynasty" {
			t.Fatalf("unexpected executor fields: %#v", calledReq.Fields)
		}
		if calledReq.StartID == nil || *calledReq.StartID != 10 {
			t.Fatalf("unexpected executor start id: %#v", calledReq)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("executor was not called")
	}

	job, err := svc.GetJob(result.JobID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if job.Status != StatusSucceeded {
		t.Fatalf("unexpected job status: %#v", job)
	}
}

func TestServiceRunJobMarksFailedOnExecutorError(t *testing.T) {
	repo := newFakeRepo()
	executor := &fakeExecutor{err: errors.New("boom")}
	svc := NewService(repo, fakeResources{}, WithExecutor(executor))

	result, err := svc.Start(context.Background(), validStartRequest())
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	var job *Job
	for i := 0; i < 20; i++ {
		job, err = svc.GetJob(result.JobID)
		if err != nil {
			t.Fatalf("get job: %v", err)
		}
		if job.Status == StatusFailed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if job == nil || job.Status != StatusFailed {
		t.Fatalf("job did not fail: %#v", job)
	}
	if job.ErrorMessage != "boom" {
		t.Fatalf("unexpected error message: %#v", job)
	}
}

func TestServiceStartValidation(t *testing.T) {
	svc := NewService(newFakeRepo(), fakeResources{})

	tests := []struct {
		name   string
		mutate func(*StartRequest)
	}{
		{name: "missing key field", mutate: func(r *StartRequest) { r.KeyField = "" }},
		{name: "missing fields", mutate: func(r *StartRequest) { r.Fields = nil }},
		{name: "key field overlaps field", mutate: func(r *StartRequest) { r.KeyField = "dynasty" }},
		{name: "bad batch size", mutate: func(r *StartRequest) { v := 0; r.Fields[0].BatchSize = &v }},
		{name: "bad batch concurrency", mutate: func(r *StartRequest) { v := 0; r.Fields[0].BatchConcurrency = &v }},
		{
			name: "start id greater than end id",
			mutate: func(r *StartRequest) {
				r.StartID = int64Ptr(20)
				r.EndID = int64Ptr(10)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validStartRequest()
			tt.mutate(&req)
			_, err := svc.Start(context.Background(), req)
			if !IsValidationError(err) {
				t.Fatalf("expected validation error, got %v", err)
			}
		})
	}
}

func TestServiceStartValidationAcceptsOpenEndedRanges(t *testing.T) {
	svc := NewService(newFakeRepo(), fakeResources{})

	tests := []struct {
		name string
		set  func(*StartRequest)
	}{
		{name: "start id only", set: func(r *StartRequest) { r.StartID = int64Ptr(10) }},
		{name: "end id only", set: func(r *StartRequest) { r.EndID = int64Ptr(20) }},
		{name: "same start and end id", set: func(r *StartRequest) {
			r.StartID = int64Ptr(30)
			r.EndID = int64Ptr(30)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validStartRequest()
			tt.set(&req)
			if _, err := svc.Start(context.Background(), req); err != nil {
				t.Fatalf("expected request to be valid, got %v", err)
			}
		})
	}
}

func TestServiceGetMissingJob(t *testing.T) {
	svc := NewService(newFakeRepo(), fakeResources{})
	_, err := svc.GetJob("missing")
	if !IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
}

var _ Repository = (*fakeRepo)(nil)
var _ ResourceReader = (*fakeResources)(nil)
