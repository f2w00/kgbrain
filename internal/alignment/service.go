// service.go 提供实体对齐应用服务，负责 job 创建、校验和后台执行协调。
package alignment

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"kgbrain/pkg/idgen"
)

// Service 编排实体对齐 job 的创建、校验和状态查询。
type Service struct {
	repo        Repository
	resources   ResourceReader
	executor    Executor
	dbOpener    BusinessDBOpener
	repoFactory BusinessRepositoryFactory
}

// NewService 创建实体对齐应用服务。
func NewService(repo Repository, resources ResourceReader, opts ...Option) *Service {
	svc := &Service{repo: repo, resources: resources}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

// Start 创建实体对齐异步任务，并启动后台执行器。
func (s *Service) Start(_ context.Context, req StartRequest) (*StartResult, error) {
	if err := validateStartRequest(req); err != nil {
		return nil, err
	}
	if _, err := s.resources.GetLLM(req.LLMResourceID); err != nil {
		return nil, err
	}
	if _, err := s.resources.GetDatabase(req.DatabaseResourceID); err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	job := &Job{
		JobID:                   idgen.GenerateEntityAlignmentJobID(),
		LLMResourceID:           req.LLMResourceID,
		DatabaseResourceID:      req.DatabaseResourceID,
		SourceTable:             req.SourceTable,
		OutputTable:             req.OutputTable,
		Status:                  StatusPending,
		KeyField:                req.KeyField,
		StartID:                 req.StartID,
		EndID:                   req.EndID,
		OnlyWaitingTargetReview: req.OnlyWaitingTargetReview,
		FuzzyTopK:               normalizeFuzzyTopK(req.FuzzyTopK),
		Fields:                  toJobFields(req.Fields),
		CreatedAt:               now,
	}
	if err := s.repo.CreateJob(job); err != nil {
		return nil, err
	}

	go s.runJob(context.Background(), job.JobID)

	return &StartResult{JobID: job.JobID, Status: job.Status}, nil
}

// GetJob 查询实体对齐异步任务状态。
func (s *Service) GetJob(jobID string) (*Job, error) {
	if strings.TrimSpace(jobID) == "" {
		return nil, &validationError{message: "job_id is required"}
	}
	job, err := s.repo.GetJob(jobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, &notFoundError{message: "entity alignment job not found"}
	}
	return job, nil
}

func (s *Service) ListTargets(ctx context.Context, req ListTargetsRequest) (*ListTargetsResult, error) {
	if strings.TrimSpace(req.DatabaseResourceID) == "" {
		return nil, &validationError{message: "database_resource_id is required"}
	}
	targetSetID := strings.TrimSpace(req.TargetSetID)
	if targetSetID == "" {
		return nil, &validationError{message: "target_set_id is required"}
	}
	repo, db, err := s.openBusinessRepo(req.DatabaseResourceID)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	targets, err := repo.LoadTargetLabels(ctx, targetSetID)
	if err != nil {
		return nil, err
	}
	labels := make([]string, 0, len(targets))
	for _, target := range targets {
		labels = append(labels, target.Label)
	}
	return &ListTargetsResult{Labels: labels}, nil
}

func (s *Service) UpsertTargets(ctx context.Context, req UpsertTargetsRequest) error {
	if strings.TrimSpace(req.DatabaseResourceID) == "" {
		return &validationError{message: "database_resource_id is required"}
	}
	targetSetID := strings.TrimSpace(req.TargetSetID)
	if targetSetID == "" {
		return &validationError{message: "target_set_id is required"}
	}
	if len(req.Labels) == 0 {
		return &validationError{message: "labels is required"}
	}
	for _, label := range req.Labels {
		if strings.TrimSpace(label) == "" {
			return &validationError{message: "labels is required"}
		}
	}
	targets := make([]TargetDefinition, 0, len(req.Labels))
	for _, label := range req.Labels {
		targets = append(targets, TargetDefinition{
			TargetSetID: targetSetID,
			Label:       strings.TrimSpace(label),
		})
	}
	repo, db, err := s.openBusinessRepo(req.DatabaseResourceID)
	if err != nil {
		return err
	}
	defer db.Close()
	return repo.UpsertTargets(ctx, targetSetID, targets)
}

func (s *Service) DeleteTarget(ctx context.Context, req DeleteTargetRequest) error {
	if strings.TrimSpace(req.DatabaseResourceID) == "" {
		return &validationError{message: "database_resource_id is required"}
	}
	targetSetID := strings.TrimSpace(req.TargetSetID)
	if targetSetID == "" {
		return &validationError{message: "target_set_id is required"}
	}
	label := strings.TrimSpace(req.Label)
	if label == "" {
		return &validationError{message: "label is required"}
	}
	repo, db, err := s.openBusinessRepo(req.DatabaseResourceID)
	if err != nil {
		return err
	}
	defer db.Close()
	return repo.DeleteTarget(ctx, targetSetID, label)
}

func (s *Service) ListCandidates(
	ctx context.Context,
	req ListCandidatesRequest,
) (*ListCandidatesResult, error) {
	if strings.TrimSpace(req.DatabaseResourceID) == "" {
		return nil, &validationError{message: "database_resource_id is required"}
	}
	targetSetID := strings.TrimSpace(req.TargetSetID)
	if targetSetID == "" {
		return nil, &validationError{message: "target_set_id is required"}
	}
	repo, db, err := s.openBusinessRepo(req.DatabaseResourceID)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	candidates, err := repo.ListTargetCandidates(ctx, targetSetID, strings.TrimSpace(req.Status))
	if err != nil {
		return nil, err
	}
	return &ListCandidatesResult{Candidates: candidates}, nil
}

func (s *Service) ReviewCandidates(ctx context.Context, req ReviewCandidatesRequest) error {
	if strings.TrimSpace(req.DatabaseResourceID) == "" {
		return &validationError{message: "database_resource_id is required"}
	}
	targetSetID := strings.TrimSpace(req.TargetSetID)
	if targetSetID == "" {
		return &validationError{message: "target_set_id is required"}
	}
	if len(req.Actions) == 0 {
		return &validationError{message: "actions is required"}
	}
	for _, action := range req.Actions {
		if strings.TrimSpace(action.CandidateID) == "" {
			return &validationError{message: "candidate_id is required"}
		}
		switch action.Resolution {
		case "add_as_label", "map_to_existing":
			if strings.TrimSpace(action.Label) == "" {
				return &validationError{message: "label is required"}
			}
		case "reject_as_null":
		default:
			return &validationError{message: "invalid resolution"}
		}
	}
	repo, db, err := s.openBusinessRepo(req.DatabaseResourceID)
	if err != nil {
		return err
	}
	defer db.Close()
	return repo.ReviewTargetCandidates(ctx, req.SourceTable, targetSetID, req.Actions)
}

// runJob 在后台 goroutine 中执行实体对齐：先标记 running，再委派 executor 执行，最终标记 succeeded/failed。
func (s *Service) runJob(ctx context.Context, jobID string) {
	defer func() {
		if r := recover(); r != nil {
			_ = s.repo.MarkFailed(jobID, fmt.Sprintf("panic: %v", r))
		}
	}()
	if err := s.repo.MarkRunning(jobID); err != nil {
		return
	}
	job, err := s.repo.GetJob(jobID)
	if err != nil {
		_ = s.repo.MarkFailed(jobID, err.Error())
		return
	}
	if job == nil {
		_ = s.repo.MarkFailed(jobID, "entity alignment job not found")
		return
	}
	select {
	case <-ctx.Done():
		_ = s.repo.MarkFailed(jobID, ctx.Err().Error())
		return
	default:
	}
	if s.executor != nil {
		req := StartRequest{
			LLMResourceID:           job.LLMResourceID,
			DatabaseResourceID:      job.DatabaseResourceID,
			SourceTable:             job.SourceTable,
			OutputTable:             job.OutputTable,
			KeyField:                job.KeyField,
			StartID:                 job.StartID,
			EndID:                   job.EndID,
			OnlyWaitingTargetReview: job.OnlyWaitingTargetReview,
			FuzzyTopK:               &job.FuzzyTopK,
			Fields:                  fromJobFields(job.Fields),
		}
		if err := s.executor.Execute(ctx, job, req, s.resources); err != nil {
			_ = s.repo.MarkFailed(jobID, err.Error())
			return
		}
	}
	if err := s.repo.MarkSucceeded(jobID); err != nil {
		_ = s.repo.MarkFailed(jobID, err.Error())
	}
}

func validateStartRequest(req StartRequest) error {
	if strings.TrimSpace(req.LLMResourceID) == "" {
		return &validationError{message: "llm_resource_id is required"}
	}
	if strings.TrimSpace(req.DatabaseResourceID) == "" {
		return &validationError{message: "database_resource_id is required"}
	}
	if strings.TrimSpace(req.SourceTable) == "" {
		return &validationError{message: "source_table is required"}
	}
	if strings.TrimSpace(req.OutputTable) == "" {
		return &validationError{message: "output_table is required"}
	}
	keyField := strings.TrimSpace(req.KeyField)
	if keyField == "" {
		return &validationError{message: "key_field is required"}
	}
	if len(req.Fields) == 0 {
		return &validationError{message: "fields is required"}
	}
	if req.StartID != nil && req.EndID != nil && *req.StartID > *req.EndID {
		return &validationError{message: "start_id must be less than or equal to end_id"}
	}
	if req.FuzzyTopK != nil && *req.FuzzyTopK <= 0 {
		return &validationError{message: "fuzzy_top_k must be greater than 0"}
	}
	for _, field := range req.Fields {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			return &validationError{message: "fields.name is required"}
		}
		if name == keyField {
			return &validationError{message: "key_field cannot be an alignment field"}
		}
		if field.BatchSize != nil && *field.BatchSize <= 0 {
			return &validationError{message: "fields.batch_size must be greater than 0"}
		}
	}
	return nil
}

func normalizeFuzzyTopK(v *int) int {
	if v == nil {
		return DefaultFuzzyTopK
	}
	return *v
}

func toJobFields(fields []FieldRequest) []FieldConfig {
	result := make([]FieldConfig, 0, len(fields))
	for _, field := range fields {
		result = append(result, FieldConfig{
			Name:             field.Name,
			TargetSetID:      field.TargetSetID,
			BatchSize:        field.BatchSize,
			BatchConcurrency: field.BatchConcurrency,
		})
	}
	return result
}

func fromJobFields(fields []FieldConfig) []FieldRequest {
	result := make([]FieldRequest, 0, len(fields))
	for _, field := range fields {
		result = append(result, FieldRequest{
			Name:             field.Name,
			TargetSetID:      field.TargetSetID,
			BatchSize:        field.BatchSize,
			BatchConcurrency: field.BatchConcurrency,
		})
	}
	return result
}

func (s *Service) openBusinessRepo(
	databaseResourceID string,
) (BusinessRepository, io.Closer, error) {
	if s.dbOpener == nil || s.repoFactory == nil {
		return nil, nil, fmt.Errorf("entity alignment business access is not configured")
	}
	dbResource, err := s.resources.GetDatabase(databaseResourceID)
	if err != nil {
		return nil, nil, err
	}
	bizDB, err := s.dbOpener(dbResource)
	if err != nil {
		return nil, nil, fmt.Errorf("open business database: %w", err)
	}
	return s.repoFactory(bizDB), bizDB, nil
}
