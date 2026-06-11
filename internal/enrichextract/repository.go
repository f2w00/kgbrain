// repository.go 提供 enrichextract job 在本地 SQLite 中的持久化能力。
package enrichextract

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// EnrichExtractRepo 实现 Repository 接口，基于 SQLite 存储 job 元数据和行级错误。
type EnrichExtractRepo struct {
	db *sql.DB
}

// NewEnrichExtractRepo 创建仓库实例，自动建表。
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
		priority_field_hints_json TEXT NOT NULL DEFAULT '{}',
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
	if err := ensureEnrichExtractJobColumns(db); err != nil {
		return nil, err
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
	priorityFieldHintsJSON, err := json.Marshal(job.PriorityFieldHints)
	if err != nil {
		return fmt.Errorf("marshal priority field hints: %w", err)
	}
	_, err = r.db.Exec(`INSERT INTO enrich_extract_jobs (
		job_id, status, llm_resource_id, database_resource_id, source_table,
		output_table, key_field, source_json_field, target_example_json,
		target_fields_json, priority_field_hints_json, start_id, end_id, overwrite, concurrency,
		page_size, max_retries, last_key, processed_rows, succeeded_rows,
		failed_rows, created_at, started_at, updated_at, finished_at,
		error_message
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.JobID, job.Status, job.LLMResourceID, job.DatabaseResourceID,
		job.SourceTable, job.OutputTable, job.KeyField, job.SourceJSONField,
		string(targetExampleJSON), string(targetFieldsJSON), string(priorityFieldHintsJSON),
		nullableInt64(job.StartID),
		nullableInt64(job.EndID), boolToInt(job.Overwrite), job.Concurrency,
		job.PageSize, job.MaxRetries, nullableInt64(job.LastKey), job.ProcessedRows,
		job.SucceededRows, job.FailedRows, job.CreatedAt, nullable(job.StartedAt),
		nullable(job.UpdatedAt), nullable(job.FinishedAt), nullable(job.ErrorMessage),
	)
	return err
}

// GetJob 按 jobID 查询单个 job。
func (r *EnrichExtractRepo) GetJob(jobID string) (*Job, error) {
	return r.getJobByQuery(selectJobsSQL+` WHERE job_id = ?`, jobID)
}

// ListActiveJobs 查询所有 pending 或 running 状态的 job，用于服务重启恢复。
func (r *EnrichExtractRepo) ListActiveJobs() ([]*Job, error) {
	rows, err := r.db.Query(selectJobsSQL+` WHERE status IN (?, ?)`, StatusPending, StatusRunning)
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

// MarkRunning 将 job 状态置为 running，设置 started_at 和 updated_at。
func (r *EnrichExtractRepo) MarkRunning(jobID string) error {
	now := nowString()
	_, err := r.db.Exec(`UPDATE enrich_extract_jobs
		SET status = ?, started_at = COALESCE(started_at, ?), updated_at = ?, error_message = NULL
		WHERE job_id = ?`, StatusRunning, now, now, jobID)
	return err
}

// MarkSucceeded 将 job 状态置为 succeeded，设置完成时间和清除错误信息。
func (r *EnrichExtractRepo) MarkSucceeded(jobID string) error {
	now := nowString()
	_, err := r.db.Exec(`UPDATE enrich_extract_jobs
		SET status = ?, finished_at = ?, updated_at = ?, error_message = NULL
		WHERE job_id = ?`, StatusSucceeded, now, now, jobID)
	return err
}

func (r *EnrichExtractRepo) MarkPartial(jobID string, errorMessage string) error {
	now := nowString()
	_, err := r.db.Exec(`UPDATE enrich_extract_jobs
		SET status = ?, finished_at = ?, updated_at = ?, error_message = ?
		WHERE job_id = ?`, StatusPartial, now, now, errorMessage, jobID)
	return err
}

// MarkFailed 将 job 状态置为 failed，记录错误信息并设置完成时间。
func (r *EnrichExtractRepo) MarkFailed(jobID string, errorMessage string) error {
	now := nowString()
	_, err := r.db.Exec(`UPDATE enrich_extract_jobs
		SET status = ?, finished_at = ?, updated_at = ?, error_message = ?
		WHERE job_id = ?`, StatusFailed, now, now, errorMessage, jobID)
	return err
}

// UpdateProgress 更新 job 执行进度：last_key、processed/succeeded/failed_rows 计数。
func (r *EnrichExtractRepo) UpdateProgress(jobID string, update ProgressUpdate) error {
	_, err := r.db.Exec(`UPDATE enrich_extract_jobs
		SET last_key = ?, processed_rows = processed_rows + ?,
			succeeded_rows = succeeded_rows + ?, failed_rows = failed_rows + ?,
			updated_at = ?
		WHERE job_id = ?`, update.LastKey, update.ProcessedRows,
		update.SucceededRows, update.FailedRows, nowString(), jobID)
	return err
}

// AddErrors 批量持久化行级错误，使用事务保证原子写入。
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

// selectJobsSQL 是查询 enrich_extract_jobs 表的完整字段列表。
const selectJobsSQL = `SELECT
	job_id, status, llm_resource_id, database_resource_id, source_table,
	output_table, key_field, source_json_field, target_example_json,
	target_fields_json, priority_field_hints_json, start_id, end_id, overwrite, concurrency,
	page_size, max_retries, last_key, processed_rows, succeeded_rows,
	failed_rows, created_at, started_at, updated_at, finished_at,
	error_message
FROM enrich_extract_jobs`

// scanJob 将一行查询结果扫描为 *Job，处理 NULL 字段和 JSON 反序列化。
func scanJob(s scanner) (*Job, error) {
	job := &Job{}
	var overwrite int
	var targetExampleJSON, targetFieldsJSON, priorityFieldHintsJSON string
	var startID, endID, lastKey sql.NullInt64
	var startedAt, updatedAt, finishedAt, errorMessage sql.NullString
	err := s.Scan(
		&job.JobID, &job.Status, &job.LLMResourceID, &job.DatabaseResourceID,
		&job.SourceTable, &job.OutputTable, &job.KeyField, &job.SourceJSONField,
		&targetExampleJSON, &targetFieldsJSON, &priorityFieldHintsJSON,
		&startID, &endID, &overwrite,
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
	if priorityFieldHintsJSON == "" {
		priorityFieldHintsJSON = "{}"
	}
	if err := json.Unmarshal([]byte(priorityFieldHintsJSON), &job.PriorityFieldHints); err != nil {
		return nil, fmt.Errorf("unmarshal priority field hints: %w", err)
	}
	job.StartedAt = startedAt.String
	job.UpdatedAt = updatedAt.String
	job.FinishedAt = finishedAt.String
	job.ErrorMessage = errorMessage.String
	return job, nil
}

// boolToInt 将 bool 转为 SQLite 整数（0/1）。
func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// nullableInt64 将 *int64 转为 SQL 参数值，nil 对应 NULL。
func nullableInt64(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

// nullable 将空字符串转为 nil（SQL NULL），非空保留指针。
func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// nowString 返回当前 UTC 时间的 RFC3339 格式字符串。
func nowString() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// ensureEnrichExtractJobColumns 为已存在的 SQLite 表补齐新增列，兼容旧版本库文件。
func ensureEnrichExtractJobColumns(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(enrich_extract_jobs)`)
	if err != nil {
		return fmt.Errorf("inspect enrich_extract_jobs columns: %w", err)
	}
	defer rows.Close()
	hasPriorityFieldHints := false
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, pk int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("scan enrich_extract_jobs column: %w", err)
		}
		if strings.EqualFold(name, "priority_field_hints_json") {
			hasPriorityFieldHints = true
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate enrich_extract_jobs columns: %w", err)
	}
	if hasPriorityFieldHints {
		return nil
	}
	if _, err := db.Exec(`ALTER TABLE enrich_extract_jobs
		ADD COLUMN priority_field_hints_json TEXT NOT NULL DEFAULT '{}'`); err != nil {
		return fmt.Errorf("add priority_field_hints_json column: %w", err)
	}
	return nil
}

var _ Repository = (*EnrichExtractRepo)(nil)
