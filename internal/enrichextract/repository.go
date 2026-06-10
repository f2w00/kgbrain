// repository.go 提供 enrichextract job 在本地 SQLite 中的持久化能力。
package enrichextract

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type EnrichExtractRepo struct {
	db *sql.DB
}

func NewEnrichExtractRepo(db *sql.DB) (*EnrichExtractRepo, error) {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS enrich_extract_jobs (
		job_id               TEXT PRIMARY KEY,
		status               TEXT NOT NULL,
		llm_resource_id      TEXT NOT NULL,
		database_resource_id TEXT NOT NULL,
		source_table         TEXT NOT NULL,
		output_table         TEXT NOT NULL,
		key_field            TEXT NOT NULL,
		source_json_field    TEXT NOT NULL,
		target_example_json  TEXT NOT NULL,
		target_fields_json   TEXT NOT NULL,
		start_id             INTEGER,
		end_id               INTEGER,
		overwrite            INTEGER NOT NULL,
		concurrency          INTEGER NOT NULL,
		page_size            INTEGER NOT NULL,
		max_retries          INTEGER NOT NULL,
		last_key             INTEGER,
		processed_rows       INTEGER NOT NULL DEFAULT 0,
		succeeded_rows       INTEGER NOT NULL DEFAULT 0,
		failed_rows          INTEGER NOT NULL DEFAULT 0,
		created_at           TEXT NOT NULL,
		started_at           TEXT,
		updated_at           TEXT,
		finished_at          TEXT,
		error_message        TEXT
	)`); err != nil {
		return nil, fmt.Errorf("create enrich_extract_jobs table: %w", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS enrich_extract_job_errors (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		job_id        TEXT NOT NULL,
		source_key    INTEGER,
		stage         TEXT NOT NULL,
		attempts      INTEGER NOT NULL DEFAULT 1,
		error_message TEXT NOT NULL,
		created_at    TEXT NOT NULL
	)`); err != nil {
		return nil, fmt.Errorf("create enrich_extract_job_errors table: %w", err)
	}
	return &EnrichExtractRepo{db: db}, nil
}

func (r *EnrichExtractRepo) CreateJob(job *Job) error {
	targetExampleJSON, err := json.Marshal(job.TargetExample)
	if err != nil {
		return fmt.Errorf("marshal target example: %w", err)
	}
	targetFieldsJSON, err := json.Marshal(job.TargetFields)
	if err != nil {
		return fmt.Errorf("marshal target fields: %w", err)
	}
	_, err = r.db.Exec(`INSERT INTO enrich_extract_jobs (
		job_id, status, llm_resource_id, database_resource_id, source_table,
		output_table, key_field, source_json_field, target_example_json,
		target_fields_json, start_id, end_id, overwrite, concurrency,
		page_size, max_retries, last_key, processed_rows, succeeded_rows,
		failed_rows, created_at, started_at, updated_at, finished_at,
		error_message
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.JobID, job.Status, job.LLMResourceID, job.DatabaseResourceID,
		job.SourceTable, job.OutputTable, job.KeyField, job.SourceJSONField,
		string(targetExampleJSON), string(targetFieldsJSON), nullableInt64(job.StartID),
		nullableInt64(job.EndID), boolToInt(job.Overwrite), job.Concurrency,
		job.PageSize, job.MaxRetries, nullableInt64(job.LastKey), job.ProcessedRows,
		job.SucceededRows, job.FailedRows, job.CreatedAt, nullable(job.StartedAt),
		nullable(job.UpdatedAt), nullable(job.FinishedAt), nullable(job.ErrorMessage),
	)
	return err
}

func (r *EnrichExtractRepo) GetJob(jobID string) (*Job, error) {
	return r.getJobByQuery(`SELECT * FROM enrich_extract_jobs WHERE job_id = ?`, jobID)
}

func (r *EnrichExtractRepo) ListActiveJobs() ([]*Job, error) {
	rows, err := r.db.Query(`SELECT * FROM enrich_extract_jobs WHERE status IN (?, ?)`, StatusPending, StatusRunning)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := make([]*Job, 0)
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (r *EnrichExtractRepo) MarkRunning(jobID string) error {
	now := nowString()
	_, err := r.db.Exec(`UPDATE enrich_extract_jobs
		SET status = ?, started_at = COALESCE(started_at, ?), updated_at = ?, error_message = NULL
		WHERE job_id = ?`, StatusRunning, now, now, jobID)
	return err
}

func (r *EnrichExtractRepo) MarkSucceeded(jobID string) error {
	now := nowString()
	_, err := r.db.Exec(`UPDATE enrich_extract_jobs
		SET status = ?, finished_at = ?, updated_at = ?, error_message = NULL
		WHERE job_id = ?`, StatusSucceeded, now, now, jobID)
	return err
}

func (r *EnrichExtractRepo) MarkFailed(jobID string, errorMessage string) error {
	now := nowString()
	_, err := r.db.Exec(`UPDATE enrich_extract_jobs
		SET status = ?, finished_at = ?, updated_at = ?, error_message = ?
		WHERE job_id = ?`, StatusFailed, now, now, errorMessage, jobID)
	return err
}

func (r *EnrichExtractRepo) UpdateProgress(jobID string, update ProgressUpdate) error {
	_, err := r.db.Exec(`UPDATE enrich_extract_jobs
		SET last_key = ?, processed_rows = processed_rows + ?,
			succeeded_rows = succeeded_rows + ?, failed_rows = failed_rows + ?,
			updated_at = ?
		WHERE job_id = ?`, update.LastKey, update.ProcessedRows,
		update.SucceededRows, update.FailedRows, nowString(), jobID)
	return err
}

func (r *EnrichExtractRepo) AddErrors(jobID string, errors []RowError) error {
	if len(errors) == 0 {
		return nil
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO enrich_extract_job_errors (
		job_id, source_key, stage, attempts, error_message, created_at
	) VALUES (?, ?, ?, ?, ?, ?)`) 
	if err != nil {
		return err
	}
	defer stmt.Close()
	createdAt := nowString()
	for _, rowErr := range errors {
		if _, err := stmt.Exec(jobID, rowErr.SourceKey, rowErr.Stage,
			rowErr.Attempts, rowErr.ErrorMessage, createdAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *EnrichExtractRepo) getJobByQuery(query string, args ...any) (*Job, error) {
	row := r.db.QueryRow(query, args...)
	job, err := scanJob(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return job, err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanJob(s scanner) (*Job, error) {
	job := &Job{}
	var overwrite int
	var targetExampleJSON, targetFieldsJSON string
	var startID, endID, lastKey sql.NullInt64
	var startedAt, updatedAt, finishedAt, errorMessage sql.NullString
	err := s.Scan(
		&job.JobID, &job.Status, &job.LLMResourceID, &job.DatabaseResourceID,
		&job.SourceTable, &job.OutputTable, &job.KeyField, &job.SourceJSONField,
		&targetExampleJSON, &targetFieldsJSON, &startID, &endID, &overwrite,
		&job.Concurrency, &job.PageSize, &job.MaxRetries, &lastKey,
		&job.ProcessedRows, &job.SucceededRows, &job.FailedRows,
		&job.CreatedAt, &startedAt, &updatedAt, &finishedAt, &errorMessage,
	)
	if err != nil {
		return nil, err
	}
	job.Overwrite = overwrite != 0
	if startID.Valid {
		job.StartID = &startID.Int64
	}
	if endID.Valid {
		job.EndID = &endID.Int64
	}
	if lastKey.Valid {
		job.LastKey = &lastKey.Int64
	}
	if err := json.Unmarshal([]byte(targetExampleJSON), &job.TargetExample); err != nil {
		return nil, fmt.Errorf("unmarshal target example: %w", err)
	}
	if err := json.Unmarshal([]byte(targetFieldsJSON), &job.TargetFields); err != nil {
		return nil, fmt.Errorf("unmarshal target fields: %w", err)
	}
	job.StartedAt = startedAt.String
	job.UpdatedAt = updatedAt.String
	job.FinishedAt = finishedAt.String
	job.ErrorMessage = errorMessage.String
	return job, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullableInt64(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nowString() string {
	return time.Now().UTC().Format(time.RFC3339)
}

var _ Repository = (*EnrichExtractRepo)(nil)
