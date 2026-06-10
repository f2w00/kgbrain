// execution_service.go 编排分页读取、逐行 LLM 和批量写入流程。
package enrichextract

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"kgbrain/pkg/extract"
)

type DomainService struct {
	repo     BusinessRepository
	jobRepo  Repository
	jobID    string
}

func NewDomainService(repo BusinessRepository, jobRepo Repository, jobID string) *DomainService {
	return &DomainService{repo: repo, jobRepo: jobRepo, jobID: jobID}
}

func (s *DomainService) Execute(ctx context.Context, llm LLMClient, req ExecuteRequest) error {
	if err := s.repo.EnsureExecutionReady(ctx, req); err != nil {
		return err
	}
	lastKey := req.LastKey
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		rows, err := s.repo.SelectSourcePage(ctx, req, lastKey)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		pageResult := s.processPage(ctx, llm, req, rows)
		writeErrors := s.writeSuccessRows(ctx, req, pageResult.Successes)
		pageResult.Errors = append(pageResult.Errors, writeErrors...)
		if err := s.jobRepo.AddErrors(s.jobID, pageResult.Errors); err != nil {
			return fmt.Errorf("save row errors: %w", err)
		}

		maxKey := rows[len(rows)-1].Key
		processed := int64(len(rows))
		succeeded := int64(len(pageResult.Successes) - len(writeErrors))
		failed := int64(len(pageResult.Errors))
		if err := s.jobRepo.UpdateProgress(s.jobID, ProgressUpdate{
			LastKey:       maxKey,
			ProcessedRows: processed,
			SucceededRows: succeeded,
			FailedRows:    failed,
		}); err != nil {
			return fmt.Errorf("update progress: %w", err)
		}
		lastKey = &maxKey
	}
}

func (s *DomainService) processPage(
	ctx context.Context,
	llm LLMClient,
	req ExecuteRequest,
	rows []SourceRow,
) PageResult {
	workerCount := min(req.Concurrency, len(rows))
	jobs := make(chan SourceRow)
	results := make(chan rowProcessResult, len(rows))
	for i := 0; i < workerCount; i++ {
		go func() {
			for row := range jobs {
				results <- s.processOneRow(ctx, llm, req, row)
			}
		}()
	}
	for _, row := range rows {
		jobs <- row
	}
	close(jobs)
	page := PageResult{
		Successes: make([]OutputRow, 0, len(rows)),
		Errors:    make([]RowError, 0),
	}
	for range rows {
		result := <-results
		if result.Err != nil {
			page.Errors = append(page.Errors, *result.Err)
			continue
		}
		page.Successes = append(page.Successes, result.Output)
	}
	return page
}

type rowProcessResult struct {
	Output OutputRow
	Err    *RowError
}

func (s *DomainService) processOneRow(
	ctx context.Context,
	llm LLMClient,
	req ExecuteRequest,
	row SourceRow,
) rowProcessResult {
	source, err := ParseSourceJSON(row.Raw, req.KeyField)
	if err != nil {
		return rowProcessResult{Err: &RowError{
			SourceKey:    row.Key,
			Stage:        "validate_source",
			Attempts:     1,
			ErrorMessage: err.Error(),
		}}
	}
	var lastErr error
	for attempt := 0; attempt <= req.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(backoff(attempt))
		}
		if err := ctx.Err(); err != nil {
			return rowProcessResult{Err: &RowError{
				SourceKey:    row.Key,
				Stage:        "llm_generate",
				Attempts:     attempt + 1,
				ErrorMessage: err.Error(),
			}}
		}
		msgs := BuildMessages(source, req.TargetExample, req.TargetFields)
		resp, err := llm.GenerateXformMessages(ctx, msgs)
		if err != nil {
			lastErr = fmt.Errorf("llm generate: %w", err)
			continue
		}
		content := extract.JSON(resp)
		if content == "" {
			lastErr = fmt.Errorf("llm returned no json")
			continue
		}
		var value any
		if err := json.Unmarshal([]byte(content), &value); err != nil {
			lastErr = fmt.Errorf("parse llm output: %w", err)
			continue
		}
		obj, ok := value.(map[string]any)
		if !ok {
			lastErr = fmt.Errorf("llm output must be json object")
			continue
		}
		aligned := AlignOutputFields(StripKey(obj, req.KeyField), req.TargetFields)
		return rowProcessResult{Output: OutputRow{Key: row.Key, Values: aligned}}
	}
	return rowProcessResult{Err: &RowError{
		SourceKey:    row.Key,
		Stage:        "parse_json",
		Attempts:     req.MaxRetries + 1,
		ErrorMessage: lastErr.Error(),
	}}
}

func (s *DomainService) writeSuccessRows(
	ctx context.Context,
	req ExecuteRequest,
	rows []OutputRow,
) []RowError {
	if len(rows) == 0 {
		return nil
	}
	if err := s.repo.BatchWriteOutputRows(ctx, req, rows); err == nil {
		return nil
	}
	errors := make([]RowError, 0)
	for _, row := range rows {
		if err := s.repo.WriteOutputRow(ctx, req, row); err != nil {
			errors = append(errors, RowError{
				SourceKey:    row.Key,
				Stage:        "write_output",
				Attempts:     1,
				ErrorMessage: err.Error(),
			})
		}
	}
	return errors
}

func backoff(attempt int) time.Duration {
	d := time.Duration(1<<uint(attempt-1)) * time.Second
	if d > 8*time.Second {
		return 8 * time.Second
	}
	return d
}
