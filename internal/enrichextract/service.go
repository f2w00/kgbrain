// service.go 提供结构化抽取应用服务，负责 job 创建、查询和恢复。
package enrichextract

import (
	"context"
	"fmt"
	"strings"
	"time"

	"kgbrain/pkg/idgen"
)

// Service 是 enrichextract 的应用层入口，协调仓库、资源读取和后台执行器。
type Service struct {
	repo      Repository
	resources ResourceReader
	executor  Executor
}

// Option 是 Service 构造选项的函数式配置。
type Option func(*Service)

// WithExecutor 注入后台执行器，用于异步执行 job。
func WithExecutor(executor Executor) Option {
	return func(s *Service) {
		s.executor = executor
	}
}

// NewService 创建 enrichextract 应用服务。
func NewService(repo Repository, resources ResourceReader, opts ...Option) *Service {
	svc := &Service{repo: repo, resources: resources}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

// Start 校验并创建新 job，写入本地 SQLite 后立即启动后台 goroutine 执行。
func (s *Service) Start(_ context.Context, req StartRequest) (*StartResult, error) {
	normalized, targetFields, err := NormalizeStartRequest(req)
	if err != nil {
		return nil, err
	}
	if _, err := s.resources.GetLLM(normalized.LLMResourceID); err != nil {
		return nil, err
	}
	if _, err := s.resources.GetDatabase(normalized.DatabaseResourceID); err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	job := &Job{
		JobID:              idgen.GenerateEnrichExtractJobID(),
		Status:             StatusPending,
		LLMResourceID:      normalized.LLMResourceID,
		DatabaseResourceID: normalized.DatabaseResourceID,
		SourceTable:        normalized.SourceTable,
		OutputTable:        normalized.OutputTable,
		KeyField:           normalized.KeyField,
		SourceJSONField:    normalized.SourceJSONField,
		TargetExample:      cloneRows(normalized.TargetExample),
		TargetFields:       append([]string(nil), targetFields...),
		PriorityFieldHints: cloneStringMap(normalized.PriorityFieldHints),
		StartID:            normalized.StartID,
		EndID:              normalized.EndID,
		Overwrite:          *normalized.Overwrite,
		Concurrency:        *normalized.Concurrency,
		PageSize:           *normalized.PageSize,
		MaxRetries:         *normalized.MaxRetries,
		CreatedAt:          now,
	}
	if err := s.repo.CreateJob(job); err != nil {
		return nil, err
	}
	go s.runJob(context.Background(), job.JobID)
	return &StartResult{JobID: job.JobID, Status: job.Status}, nil
}

// GetJob 根据 jobID 查询 job 详情，返回 nil 时触发 notFoundError。
func (s *Service) GetJob(jobID string) (*Job, error) {
	if strings.TrimSpace(jobID) == "" {
		return nil, &validationError{message: "job_id is required"}
	}
	job, err := s.repo.GetJob(jobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, &notFoundError{message: "enrich extract job not found"}
	}
	return job, nil
}

// RecoverActiveJobs 扫描所有 pending/running 的 job 并重新启动后台执行。
// 在服务启动时调用，实现断点续跑。
func (s *Service) RecoverActiveJobs(_ context.Context) error {
	jobs, err := s.repo.ListActiveJobs()
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if job == nil {
			continue
		}
		go s.runJob(context.Background(), job.JobID)
	}
	return nil
}

// runJob 在后台 goroutine 中执行 job：标记运行 → 加载快照 → 调用 executor → 标记终态。
// 执行完成后根据 failed_rows 决定标记 succeeded 还是 failed。
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
		_ = s.repo.MarkFailed(jobID, "enrich extract job not found")
		return
	}
	if err := ctx.Err(); err != nil {
		_ = s.repo.MarkFailed(jobID, err.Error())
		return
	}
	if s.executor == nil {
		_ = s.repo.MarkFailed(jobID, "enrich extract executor dependencies are not configured")
		return
	}
	req := StartRequest{
		LLMResourceID:      job.LLMResourceID,
		DatabaseResourceID: job.DatabaseResourceID,
		SourceTable:        job.SourceTable,
		OutputTable:        job.OutputTable,
		KeyField:           job.KeyField,
		SourceJSONField:    job.SourceJSONField,
		TargetExample:      cloneRows(job.TargetExample),
		PriorityFieldHints: cloneStringMap(job.PriorityFieldHints),
		StartID:            job.StartID,
		EndID:              job.EndID,
		Overwrite:          boolPtr(job.Overwrite),
		Concurrency:        intPtr(job.Concurrency),
		PageSize:           intPtr(job.PageSize),
		MaxRetries:         intPtr(job.MaxRetries),
	}
	if err := s.executor.Execute(ctx, job, req, s.resources); err != nil {
		_ = s.repo.MarkFailed(jobID, err.Error())
		return
	}
	finalJob, err := s.repo.GetJob(jobID)
	if err != nil {
		_ = s.repo.MarkFailed(jobID, err.Error())
		return
	}
	if finalJob != nil {
		if finalJob.FailedRows == 0 {
			if err := s.repo.MarkSucceeded(jobID); err != nil {
				_ = s.repo.MarkFailed(jobID, err.Error())
			}
			return
		}
		if finalJob.SucceededRows > 0 {
			msg := fmt.Sprintf(
				"completed with row errors: succeeded_rows=%d failed_rows=%d",
				finalJob.SucceededRows,
				finalJob.FailedRows,
			)
			_ = s.repo.MarkPartial(jobID, msg)
			return
		}
		msg := fmt.Sprintf("all rows failed: failed_rows=%d", finalJob.FailedRows)
		_ = s.repo.MarkFailed(jobID, msg)
		return
	}
	if err := s.repo.MarkSucceeded(jobID); err != nil {
		_ = s.repo.MarkFailed(jobID, err.Error())
	}
}

func intPtr(v int) *int {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}

// cloneRows 深拷贝 []map[string]any，避免意外修改共享底层数组。
func cloneRows(rows []map[string]any) []map[string]any {
	cloned := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		m := make(map[string]any, len(row))
		for k, v := range row {
			m[k] = v
		}
		cloned = append(cloned, m)
	}
	return cloned
}

// cloneStringMap 深拷贝 map[string]string，避免共享底层引用。
func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(src))
	for key, value := range src {
		cloned[key] = value
	}
	return cloned
}
