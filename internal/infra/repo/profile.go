package repo

import (
	"database/sql"
	"fmt"
	"time"

	"kgbrain/internal/domain/profile"
)

type ProfileRepo struct {
	db *sql.DB
}

func NewProfileRepo(db *sql.DB) (*ProfileRepo, error) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS profiles (
		id            TEXT PRIMARY KEY,
		llm_config    TEXT NOT NULL,
		notify_config TEXT,
		created_at    TEXT NOT NULL,
		updated_at    TEXT NOT NULL
	)`)
	if err != nil {
		return nil, fmt.Errorf("create profiles table: %w", err)
	}
	return &ProfileRepo{db: db}, nil
}

func (r *ProfileRepo) Get(id string) (*profile.Profile, error) {
	p := &profile.Profile{}
	err := r.db.QueryRow(
		`SELECT id, llm_config, COALESCE(notify_config,''), created_at, updated_at FROM profiles WHERE id = ?`, id).
		Scan(&p.ID, &p.LLMConfig, &p.NotifyConfig, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *ProfileRepo) Save(p *profile.Profile) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if p.CreatedAt == "" {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	_, err := r.db.Exec(
		`INSERT OR REPLACE INTO profiles (id, llm_config, notify_config, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		p.ID, p.LLMConfig, nullable(p.NotifyConfig), p.CreatedAt, p.UpdatedAt)
	return err
}

func (r *ProfileRepo) Delete(id string) (bool, error) {
	res, err := r.db.Exec(`DELETE FROM profiles WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
