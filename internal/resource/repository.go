package resource

import (
	"database/sql"
	"fmt"
	"time"
)

type Repository interface {
	SaveLLM(*LLMResource) error
	GetLLM(id string) (*LLMResource, error)
	DeleteLLM(id string) (bool, error)

	SaveDatabase(*DatabaseResource) error
	GetDatabase(id string) (*DatabaseResource, error)
	DeleteDatabase(id string) (bool, error)
}

type ResourceRepo struct {
	db *sql.DB
}

func NewResourceRepo(db *sql.DB) (*ResourceRepo, error) {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS llm_resources (
		resource_id     TEXT PRIMARY KEY,
		name            TEXT,
		base_url        TEXT NOT NULL,
		api_key         TEXT NOT NULL,
		model           TEXT NOT NULL,
		timeout_seconds INTEGER NOT NULL,
		temperature     REAL,
		max_concurrency INTEGER,
		created_at      TEXT NOT NULL,
		updated_at      TEXT NOT NULL
	)`); err != nil {
		return nil, fmt.Errorf("create llm_resources table: %w", err)
	}
	if err := ensureColumn(db, "llm_resources", "max_concurrency", "INTEGER"); err != nil {
		return nil, fmt.Errorf("migrate llm_resources table: %w", err)
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS database_resources (
		resource_id TEXT PRIMARY KEY,
		name        TEXT,
		type        TEXT NOT NULL,
		host        TEXT NOT NULL,
		port        INTEGER NOT NULL,
		database    TEXT NOT NULL,
		user        TEXT NOT NULL,
		password    TEXT,
		sslmode     TEXT,
		created_at  TEXT NOT NULL,
		updated_at  TEXT NOT NULL
	)`); err != nil {
		return nil, fmt.Errorf("create database_resources table: %w", err)
	}

	return &ResourceRepo{db: db}, nil
}

func (r *ResourceRepo) SaveLLM(res *LLMResource) error {
	now := time.Now().UTC().Format(time.RFC3339)
	createdAt := res.CreatedAt
	if createdAt == "" {
		if existing, err := r.GetLLM(res.ID); err != nil {
			return err
		} else if existing != nil {
			createdAt = existing.CreatedAt
		}
	}
	if createdAt == "" {
		createdAt = now
	}
	res.CreatedAt = createdAt
	res.UpdatedAt = now

	_, err := r.db.Exec(`INSERT OR REPLACE INTO llm_resources (
		resource_id,
		name,
		base_url,
		api_key,
		model,
		timeout_seconds,
		temperature,
		max_concurrency,
		created_at,
		updated_at
	)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		res.ID,
		nullable(res.Name),
		res.BaseURL,
		res.APIKey,
		res.Model,
		res.TimeoutSeconds,
		nullableFloat(res.Temperature),
		nullableInt(res.MaxConcurrency),
		res.CreatedAt,
		res.UpdatedAt,
	)
	return err
}

func (r *ResourceRepo) GetLLM(id string) (*LLMResource, error) {
	res := &LLMResource{}
	var name sql.NullString
	var temperature sql.NullFloat64
	var maxConcurrency sql.NullInt64
	err := r.db.QueryRow(`SELECT
		resource_id,
		name,
		base_url,
		api_key,
		model,
		timeout_seconds,
		temperature,
		max_concurrency,
		created_at,
		updated_at
		FROM llm_resources WHERE resource_id = ?`, id).
		Scan(
			&res.ID,
			&name,
			&res.BaseURL,
			&res.APIKey,
			&res.Model,
			&res.TimeoutSeconds,
			&temperature,
			&maxConcurrency,
			&res.CreatedAt,
			&res.UpdatedAt,
		)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	res.Name = name.String
	if temperature.Valid {
		res.Temperature = &temperature.Float64
	}
	if maxConcurrency.Valid {
		v := int(maxConcurrency.Int64)
		res.MaxConcurrency = &v
	}
	return res, nil
}

func (r *ResourceRepo) DeleteLLM(id string) (bool, error) {
	res, err := r.db.Exec(`DELETE FROM llm_resources WHERE resource_id = ?`, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *ResourceRepo) SaveDatabase(res *DatabaseResource) error {
	now := time.Now().UTC().Format(time.RFC3339)
	createdAt := res.CreatedAt
	if createdAt == "" {
		if existing, err := r.GetDatabase(res.ID); err != nil {
			return err
		} else if existing != nil {
			createdAt = existing.CreatedAt
		}
	}
	if createdAt == "" {
		createdAt = now
	}
	res.CreatedAt = createdAt
	res.UpdatedAt = now

	_, err := r.db.Exec(`INSERT OR REPLACE INTO database_resources (
		resource_id,
		name,
		type,
		host,
		port,
		database,
		user,
		password,
		sslmode,
		created_at,
		updated_at
	)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		res.ID,
		nullable(res.Name),
		res.Type,
		res.Host,
		res.Port,
		res.Database,
		res.User,
		nullable(res.Password),
		nullable(res.SSLMode),
		res.CreatedAt,
		res.UpdatedAt,
	)
	return err
}

func (r *ResourceRepo) GetDatabase(id string) (*DatabaseResource, error) {
	res := &DatabaseResource{}
	var name, password, sslMode sql.NullString
	err := r.db.QueryRow(`SELECT
		resource_id,
		name,
		type,
		host,
		port,
		database,
		user,
		password,
		sslmode,
		created_at,
		updated_at
		FROM database_resources WHERE resource_id = ?`, id).
		Scan(
			&res.ID,
			&name,
			&res.Type,
			&res.Host,
			&res.Port,
			&res.Database,
			&res.User,
			&password,
			&sslMode,
			&res.CreatedAt,
			&res.UpdatedAt,
		)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	res.Name = name.String
	res.Password = password.String
	res.SSLMode = sslMode.String
	return res, nil
}

func (r *ResourceRepo) DeleteDatabase(id string) (bool, error) {
	res, err := r.db.Exec(`DELETE FROM database_resources WHERE resource_id = ?`, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func nullableFloat(v *float64) *float64 {
	if v == nil {
		return nil
	}
	return v
}

func nullableInt(v *int) any {
	if v == nil {
		return nil
	}
	return int64(*v)
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func ensureColumn(db *sql.DB, tableName, columnName, columnType string) error {
	rows, err := db.Query(`PRAGMA table_info(` + tableName + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == columnName {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = db.Exec(`ALTER TABLE ` + tableName + ` ADD COLUMN ` + columnName + ` ` + columnType)
	return err
}

var _ Repository = (*ResourceRepo)(nil)
