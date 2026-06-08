package repo

import (
	"database/sql"
	"fmt"
	"time"

	"kgbrain/internal/domain/resource"
)

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
		created_at      TEXT NOT NULL,
		updated_at      TEXT NOT NULL
	)`); err != nil {
		return nil, fmt.Errorf("create llm_resources table: %w", err)
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

func (r *ResourceRepo) SaveLLM(res *resource.LLMResource) error {
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

	_, err := r.db.Exec(`INSERT OR REPLACE INTO llm_resources
		(resource_id, name, base_url, api_key, model, timeout_seconds, temperature, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		res.ID,
		nullable(res.Name),
		res.BaseURL,
		res.APIKey,
		res.Model,
		res.TimeoutSeconds,
		nullableFloat(res.Temperature),
		res.CreatedAt,
		res.UpdatedAt,
	)
	return err
}

func (r *ResourceRepo) GetLLM(id string) (*resource.LLMResource, error) {
	res := &resource.LLMResource{}
	var name sql.NullString
	var temperature sql.NullFloat64
	err := r.db.QueryRow(`SELECT resource_id, name, base_url, api_key, model, timeout_seconds, temperature, created_at, updated_at
		FROM llm_resources WHERE resource_id = ?`, id).
		Scan(&res.ID, &name, &res.BaseURL, &res.APIKey, &res.Model, &res.TimeoutSeconds, &temperature, &res.CreatedAt, &res.UpdatedAt)
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

func (r *ResourceRepo) SaveDatabase(res *resource.DatabaseResource) error {
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

	_, err := r.db.Exec(`INSERT OR REPLACE INTO database_resources
		(resource_id, name, type, host, port, database, user, password, sslmode, created_at, updated_at)
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

func (r *ResourceRepo) GetDatabase(id string) (*resource.DatabaseResource, error) {
	res := &resource.DatabaseResource{}
	var name, password, sslMode sql.NullString
	err := r.db.QueryRow(`SELECT resource_id, name, type, host, port, database, user, password, sslmode, created_at, updated_at
		FROM database_resources WHERE resource_id = ?`, id).
		Scan(&res.ID, &name, &res.Type, &res.Host, &res.Port, &res.Database, &res.User, &password, &sslMode, &res.CreatedAt, &res.UpdatedAt)
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
