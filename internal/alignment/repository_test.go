package alignment

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func newTestEntityAlignmentRepo(t *testing.T) *EntityAlignmentRepo {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	repo, err := NewEntityAlignmentRepo(db)
	if err != nil {
		t.Fatalf("new entity alignment repo: %v", err)
	}
	return repo
}

func TestEntityAlignmentRepoJobLifecycle(t *testing.T) {
	repo := newTestEntityAlignmentRepo(t)
	startID := int64(100)
	endID := int64(200)
	job := &Job{
		JobID:                   "ea_job_1",
		LLMResourceID:           "llm_1",
		DatabaseResourceID:      "db_1",
		SourceTable:             "public.source",
		OutputTable:             "public.output",
		Status:                  StatusPending,
		ReuseMapping:            true,
		OnlyWaitingTargetReview: true,
		KeyField:                "id",
		StartID:                 &startID,
		EndID:                   &endID,
		CreatedAt:               "2026-06-09T00:00:00Z",
	}
	if err := repo.CreateJob(job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	got, err := repo.GetJob("ea_job_1")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if got == nil || got.Status != StatusPending || got.KeyField != "id" {
		t.Fatalf("unexpected job: %#v", got)
	}
	if got.StartID == nil || *got.StartID != startID {
		t.Fatalf("unexpected job start id: %#v", got)
	}
	if got.EndID == nil || *got.EndID != endID {
		t.Fatalf("unexpected job: %#v", got)
	}
	if len(got.Fields) != 0 {
		t.Fatalf("unexpected job fields: %#v", got)
	}
	if !got.OnlyWaitingTargetReview {
		t.Fatalf("expected only waiting target review flag to be persisted")
	}

	if err := repo.MarkRunning("ea_job_1"); err != nil {
		t.Fatalf("mark running: %v", err)
	}
	got, err = repo.GetJob("ea_job_1")
	if err != nil {
		t.Fatalf("get running job: %v", err)
	}
	if got.Status != StatusRunning || got.StartedAt == "" {
		t.Fatalf("unexpected running job: %#v", got)
	}

	if err := repo.MarkSucceeded("ea_job_1"); err != nil {
		t.Fatalf("mark succeeded: %v", err)
	}
	got, err = repo.GetJob("ea_job_1")
	if err != nil {
		t.Fatalf("get succeeded job: %v", err)
	}
	if got.Status != StatusSucceeded || got.FinishedAt == "" {
		t.Fatalf("unexpected succeeded job: %#v", got)
	}

	if err := repo.MarkFailed("ea_job_1", "boom"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	got, err = repo.GetJob("ea_job_1")
	if err != nil {
		t.Fatalf("get failed job: %v", err)
	}
	if got.Status != StatusFailed || got.ErrorMessage != "boom" {
		t.Fatalf("unexpected failed job: %#v", got)
	}
}

func TestEntityAlignmentRepoJobLifecycleWithNilRange(t *testing.T) {
	repo := newTestEntityAlignmentRepo(t)
	batchSize := 20
	job := &Job{
		JobID:              "ea_job_2",
		LLMResourceID:      "llm_1",
		DatabaseResourceID: "db_1",
		SourceTable:        "public.source",
		OutputTable:        "public.output",
		Status:             StatusPending,
		ReuseMapping:       false,
		KeyField:           "id",
		Fields: []FieldConfig{{
			Name:        "dynasty",
			TargetSetID: "dynasty",
			BatchSize:   &batchSize,
		}},
		CreatedAt: "2026-06-09T00:00:00Z",
	}
	if err := repo.CreateJob(job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	got, err := repo.GetJob("ea_job_2")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if got == nil {
		t.Fatalf("expected job, got nil")
	}
	if got.StartID != nil || got.EndID != nil {
		t.Fatalf("expected nil range, got %#v", got)
	}
	if len(got.Fields) != 1 || got.Fields[0].Name != "dynasty" {
		t.Fatalf("unexpected job fields: %#v", got)
	}
}
