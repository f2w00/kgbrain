// service.go 提供实体对齐应用服务，负责 job 创建、校验和后台执行协调。
package alignment

import (
	"context"
	"fmt"
	"strings"
	"time"

	"kgbrain/pkg/idgen"
)

// Service 编排实体对齐 job 的创建、校验和状态查询。
type Service struct {
	repo      Repository
	resources ResourceReader
	executor  Executor
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

	reuseMapping := true
	if req.ReuseMapping != nil {
		reuseMapping = *req.ReuseMapping
	}
	now := time.Now().UTC().Format(time.RFC3339)
	job := &Job{
		JobID:              idgen.GenerateEntityAlignmentJobID(),
		LLMResourceID:      req.LLMResourceID,
		DatabaseResourceID: req.DatabaseResourceID,
		SourceTable:        req.SourceTable,
		OutputTable:        req.OutputTable,
		Status:             StatusPending,
		ReuseMapping:       reuseMapping,
		KeyField:           req.KeyField,
		StartID:            req.StartID,
		EndID:              req.EndID,
		Fields:             toJobFields(req.Fields),
		CreatedAt:          now,
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

// runJob 在后台 goroutine 中执行实体对齐：先标记 running，再委派 executor 执行，最终标记 succeeded/failed。
func (s *Service) runJob(ctx context.Context, jobID string) {
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
			LLMResourceID:      job.LLMResourceID,
			DatabaseResourceID: job.DatabaseResourceID,
			SourceTable:        job.SourceTable,
			OutputTable:        job.OutputTable,
			ReuseMapping:       &job.ReuseMapping,
			KeyField:           job.KeyField,
			StartID:            job.StartID,
			EndID:              job.EndID,
			Fields:             fromJobFields(job.Fields),
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
	for _, field := range req.Fields {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			return &validationError{message: "fields.name is required"}
		}
		if name == keyField {
			return &validationError{message: "key_field cannot be an alignment field"}
		}
		if len(field.Targets) == 0 {
			return &validationError{message: fmt.Sprintf("fields[%s].targets is required", name)}
		}
		if field.BatchSize != nil && *field.BatchSize <= 0 {
			return &validationError{message: "fields.batch_size must be greater than 0"}
		}
		if field.BatchConcurrency != nil && *field.BatchConcurrency <= 0 {
			return &validationError{message: "fields.batch_concurrency must be greater than 0"}
		}
	}
	return nil
}

func toJobFields(fields []FieldRequest) []FieldConfig {
	result := make([]FieldConfig, 0, len(fields))
	for _, field := range fields {
		result = append(result, FieldConfig{
			Name:             field.Name,
			Targets:          append([]string(nil), field.Targets...),
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
			Targets:          append([]string(nil), field.Targets...),
			BatchSize:        field.BatchSize,
			BatchConcurrency: field.BatchConcurrency,
		})
	}
	return result
}
