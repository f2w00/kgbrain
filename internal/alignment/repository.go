// repository.go 提供实体对齐 job 在本地 SQLite 中的持久化能力。
package alignment

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// EntityAlignmentRepo 使用服务本地 SQLite 存储实体对齐 job 状态。
type EntityAlignmentRepo struct {
	db *sql.DB
}

// NewEntityAlignmentRepo 创建实体对齐 job 仓储，并初始化本地 SQLite 表结构。
func NewEntityAlignmentRepo(db *sql.DB) (*EntityAlignmentRepo, error) {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS entity_alignment_jobs (
		job_id               TEXT PRIMARY KEY,
		llm_resource_id      TEXT NOT NULL,
		database_resource_id TEXT NOT NULL,
		source_table         TEXT NOT NULL,
		output_table         TEXT NOT NULL,
		status               TEXT NOT NULL,
		reuse_mapping        INTEGER NOT NULL,
		only_waiting_target_review INTEGER NOT NULL DEFAULT 0,
		key_field            TEXT NOT NULL,
		start_id             INTEGER,
		end_id               INTEGER,
		fields_json          TEXT NOT NULL DEFAULT '[]',
		created_at           TEXT NOT NULL,
		started_at           TEXT,
		finished_at          TEXT,
		error_message        TEXT
	)`); err != nil {
		return nil, fmt.Errorf("create entity_alignment_jobs table: %w", err)
	}
	if err := ensureColumn(db, "entity_alignment_jobs", "key_field", "TEXT"); err != nil {
		return nil, fmt.Errorf("migrate entity_alignment_jobs table: %w", err)
	}
	if err := ensureColumn(db, "entity_alignment_jobs", "start_id", "INTEGER"); err != nil {
		return nil, fmt.Errorf("migrate entity_alignment_jobs table: %w", err)
	}
	if err := ensureColumn(db, "entity_alignment_jobs", "end_id", "INTEGER"); err != nil {
		return nil, fmt.Errorf("migrate entity_alignment_jobs table: %w", err)
	}
	if err := ensureColumn(
		db,
		"entity_alignment_jobs",
		"only_waiting_target_review",
		"INTEGER NOT NULL DEFAULT 0",
	); err != nil {
		return nil, fmt.Errorf("migrate entity_alignment_jobs table: %w", err)
	}
	if err := ensureColumn(db, "entity_alignment_jobs", "fields_json", "TEXT"); err != nil {
		return nil, fmt.Errorf("migrate entity_alignment_jobs table: %w", err)
	}
	return &EntityAlignmentRepo{db: db}, nil
}

// CreateJob 写入 pending 状态的实体对齐 job。
func (r *EntityAlignmentRepo) CreateJob(job *Job) error {
	fieldsJSON, err := json.Marshal(job.Fields)
	if err != nil {
		return fmt.Errorf("marshal entity alignment fields: %w", err)
	}
	_, err = r.db.Exec(`INSERT INTO entity_alignment_jobs (
		job_id,
		llm_resource_id,
		database_resource_id,
		source_table,
		output_table,
		status,
		reuse_mapping,
		only_waiting_target_review,
		key_field,
		start_id,
		end_id,
		fields_json,
		created_at,
		started_at,
		finished_at,
		error_message
	)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.JobID,
		job.LLMResourceID,
		job.DatabaseResourceID,
		job.SourceTable,
		job.OutputTable,
		job.Status,
		boolToInt(job.ReuseMapping),
		boolToInt(job.OnlyWaitingTargetReview),
		job.KeyField,
		nullableInt64(job.StartID),
		nullableInt64(job.EndID),
		string(fieldsJSON),
		job.CreatedAt,
		nullable(job.StartedAt),
		nullable(job.FinishedAt),
		nullable(job.ErrorMessage),
	)
	return err
}

// GetJob 按 job_id 查询实体对齐 job；不存在时返回 nil, nil。
func (r *EntityAlignmentRepo) GetJob(jobID string) (*Job, error) {
	job := &Job{}
	var reuseMapping int
	var onlyWaitingTargetReview int
	var fieldsJSON string
	var startID, endID sql.NullInt64
	var startedAt, finishedAt, errorMessage sql.NullString
	err := r.db.QueryRow(`SELECT
		job_id,
		llm_resource_id,
		database_resource_id,
		source_table,
		output_table,
		status,
		reuse_mapping,
		only_waiting_target_review,
		key_field,
		start_id,
		end_id,
		fields_json,
		created_at,
		started_at,
		finished_at,
		error_message
		FROM entity_alignment_jobs WHERE job_id = ?`, jobID).
		Scan(
			&job.JobID,
			&job.LLMResourceID,
			&job.DatabaseResourceID,
			&job.SourceTable,
			&job.OutputTable,
			&job.Status,
			&reuseMapping,
			&onlyWaitingTargetReview,
			&job.KeyField,
			&startID,
			&endID,
			&fieldsJSON,
			&job.CreatedAt,
			&startedAt,
			&finishedAt,
			&errorMessage,
		)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	job.ReuseMapping = reuseMapping != 0
	job.OnlyWaitingTargetReview = onlyWaitingTargetReview != 0
	if startID.Valid {
		job.StartID = &startID.Int64
	}
	if endID.Valid {
		job.EndID = &endID.Int64
	}
	if err := json.Unmarshal([]byte(fieldsJSON), &job.Fields); err != nil {
		return nil, fmt.Errorf("unmarshal entity alignment fields: %w", err)
	}
	job.StartedAt = startedAt.String
	job.FinishedAt = finishedAt.String
	job.ErrorMessage = errorMessage.String
	return job, nil
}

// MarkRunning 将 job 标记为 running。
func (r *EntityAlignmentRepo) MarkRunning(jobID string) error {
	_, err := r.db.Exec(`UPDATE entity_alignment_jobs
		SET status = ?, started_at = ?, error_message = NULL
		WHERE job_id = ?`,
		StatusRunning,
		nowString(),
		jobID,
	)
	return err
}

// MarkSucceeded 将 job 标记为 succeeded。
func (r *EntityAlignmentRepo) MarkSucceeded(jobID string) error {
	_, err := r.db.Exec(`UPDATE entity_alignment_jobs
		SET status = ?, finished_at = ?, error_message = NULL
		WHERE job_id = ?`,
		StatusSucceeded,
		nowString(),
		jobID,
	)
	return err
}

// MarkFailed 将 job 标记为 failed 并保存错误信息。
func (r *EntityAlignmentRepo) MarkFailed(jobID string, errorMessage string) error {
	_, err := r.db.Exec(`UPDATE entity_alignment_jobs
		SET status = ?, finished_at = ?, error_message = ?
		WHERE job_id = ?`,
		StatusFailed,
		nowString(),
		errorMessage,
		jobID,
	)
	return err
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

func ensureColumn(db *sql.DB, tableName, columnName, columnType string) error {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(1) FROM pragma_table_info(?) WHERE name = ?`,
		tableName,
		columnName,
	).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err = db.Exec(
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, columnName, columnType),
	)
	return err
}

var _ Repository = (*EntityAlignmentRepo)(nil)
