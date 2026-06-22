package alignment

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"kgbrain/internal/processrecord"
	"kgbrain/internal/resource"

	_ "modernc.org/sqlite"
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

type fakeBusinessRepo struct {
	reviewSourceTable string
	reviewTargetSetID string
	reviewActions     []ReviewCandidateAction
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

func (r *fakeBusinessRepo) EnsureExecutionReady(
	_ context.Context,
	_ ExecuteRequest,
	_ []PreparedField,
) ([]ColumnMeta, error) {
	return nil, nil
}

func (r *fakeBusinessRepo) SelectDistinctRawValues(
	_ context.Context,
	_ ExecuteRequest,
	_ string,
) ([]string, error) {
	return nil, nil
}

func (r *fakeBusinessRepo) LoadExistingMappings(
	_ context.Context,
	_ ExecuteRequest,
	_ PreparedField,
	_ []string,
) (map[string]MappingRecord, error) {
	return nil, nil
}

func (r *fakeBusinessRepo) TouchMappings(
	_ context.Context,
	_ ExecuteRequest,
	_ PreparedField,
	_ map[string]MappingRecord,
) error {
	return nil
}

func (r *fakeBusinessRepo) UpsertMappings(
	_ context.Context,
	_ ExecuteRequest,
	_ PreparedField,
	_ []MappingRecord,
) error {
	return nil
}

func (r *fakeBusinessRepo) LoadTargetLabels(
	_ context.Context,
	_ string,
) ([]TargetDefinition, error) {
	return nil, nil
}

func (r *fakeBusinessRepo) RecallTopKTargets(
	_ context.Context,
	_ string,
	_ string,
	_ int,
) ([]string, error) {
	return nil, nil
}

func (r *fakeBusinessRepo) UpsertTargets(
	_ context.Context,
	_ string,
	_ []TargetDefinition,
) error {
	return nil
}

func (r *fakeBusinessRepo) DeleteTarget(
	_ context.Context,
	_ string,
	_ string,
) error {
	return nil
}

func (r *fakeBusinessRepo) UpsertTargetCandidates(
	_ context.Context,
	_ string,
	_ []MappingRecord,
) error {
	return nil
}

func (r *fakeBusinessRepo) ListTargetCandidates(
	_ context.Context,
	_ string,
	_ string,
) ([]TargetCandidate, error) {
	return nil, nil
}

func (r *fakeBusinessRepo) ReviewTargetCandidates(
	_ context.Context,
	sourceTable string,
	targetSetID string,
	actions []ReviewCandidateAction,
) error {
	r.reviewSourceTable = sourceTable
	r.reviewTargetSetID = targetSetID
	r.reviewActions = append([]ReviewCandidateAction(nil), actions...)
	return nil
}

func (r *fakeBusinessRepo) BuildSourceRangeProcessRecords(
	_ context.Context,
	_ ExecuteRequest,
	_ []PreparedField,
) ([]processrecord.Record, error) {
	return nil, nil
}

func (r *fakeBusinessRepo) WriteOutputRows(
	_ context.Context,
	_ ExecuteRequest,
	_ []ColumnMeta,
	_ []PreparedField,
) error {
	return nil
}

func validStartRequest() StartRequest {
	return StartRequest{
		LLMResourceID:      "llm_1",
		DatabaseResourceID: "db_1",
		SourceTable:        "public.source",
		OutputTable:        "public.output",
		KeyField:           "id",
		Fields:             []FieldRequest{{Name: "dynasty", TargetSetID: "dynasty"}},
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
	req.OnlyWaitingTargetReview = true

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
	if !job.OnlyWaitingTargetReview {
		t.Fatalf("expected only waiting target review flag on job")
	}
	if job.FuzzyTopK != DefaultFuzzyTopK {
		t.Fatalf("unexpected fuzzy top k: %d", job.FuzzyTopK)
	}
}

func TestServiceRunJobUsesExecutor(t *testing.T) {
	repo := newFakeRepo()
	executor := &fakeExecutor{called: make(chan StartRequest, 1)}
	svc := NewService(repo, fakeResources{}, WithExecutor(executor))
	req := validStartRequest()
	req.StartID = int64Ptr(10)
	req.EndID = int64Ptr(20)
	req.OnlyWaitingTargetReview = true
	fuzzyTopK := 20
	req.FuzzyTopK = &fuzzyTopK

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
		if !calledReq.OnlyWaitingTargetReview {
			t.Fatalf("expected only waiting target review flag in executor request")
		}
		if calledReq.FuzzyTopK == nil || *calledReq.FuzzyTopK != fuzzyTopK {
			t.Fatalf("unexpected fuzzy top k in executor request: %#v", calledReq)
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
		{name: "bad fuzzy top k", mutate: func(r *StartRequest) { v := 0; r.FuzzyTopK = &v }},
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

func TestServiceStartValidationIgnoresDisabledBatchConcurrency(t *testing.T) {
	svc := NewService(newFakeRepo(), fakeResources{})
	req := validStartRequest()
	v := 0
	req.Fields[0].BatchConcurrency = &v
	if _, err := svc.Start(context.Background(), req); err != nil {
		t.Fatalf("expected disabled batch_concurrency to be ignored, got %v", err)
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

func TestServiceReviewCandidatesPassesSourceTable(t *testing.T) {
	jobRepo := newFakeRepo()
	bizRepo := &fakeBusinessRepo{}
	svc := NewService(
		jobRepo,
		fakeResources{},
		WithBusinessAccess(
			func(_ *resource.DatabaseResource) (*sql.DB, error) {
				return sql.Open("sqlite", ":memory:")
			},
			func(_ *sql.DB) BusinessRepository {
				return bizRepo
			},
		),
	)
	err := svc.ReviewCandidates(context.Background(), ReviewCandidatesRequest{
		DatabaseResourceID: "db_1",
		TargetSetID:        "industry",
		SourceTable:        "biz.company_raw",
		Actions: []ReviewCandidateAction{{
			CandidateID: "candidate_1",
			Resolution:  "reject_as_null",
		}},
	})
	if err != nil {
		t.Fatalf("review candidates: %v", err)
	}
	if bizRepo.reviewSourceTable != "biz.company_raw" {
		t.Fatalf("unexpected source table: %#v", bizRepo.reviewSourceTable)
	}
	if bizRepo.reviewTargetSetID != "industry" {
		t.Fatalf("unexpected target set id: %#v", bizRepo.reviewTargetSetID)
	}
	if len(bizRepo.reviewActions) != 1 ||
		bizRepo.reviewActions[0].CandidateID != "candidate_1" {
		t.Fatalf("unexpected review actions: %#v", bizRepo.reviewActions)
	}
}

func TestServiceReviewCandidatesAllowsEmptySourceTable(t *testing.T) {
	jobRepo := newFakeRepo()
	bizRepo := &fakeBusinessRepo{}
	svc := NewService(
		jobRepo,
		fakeResources{},
		WithBusinessAccess(
			func(_ *resource.DatabaseResource) (*sql.DB, error) {
				return sql.Open("sqlite", ":memory:")
			},
			func(_ *sql.DB) BusinessRepository {
				return bizRepo
			},
		),
	)
	err := svc.ReviewCandidates(context.Background(), ReviewCandidatesRequest{
		DatabaseResourceID: "db_1",
		TargetSetID:        "industry",
		Actions: []ReviewCandidateAction{{
			CandidateID: "candidate_1",
			Resolution:  "reject_as_null",
		}},
	})
	if err != nil {
		t.Fatalf("review candidates: %v", err)
	}
}

var _ Repository = (*fakeRepo)(nil)
var _ ResourceReader = (*fakeResources)(nil)
var _ BusinessRepository = (*fakeBusinessRepo)(nil)
