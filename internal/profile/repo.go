package profile

import (
	"database/sql"
	"fmt"
	"time"
)

// Repo 管理 profiles 表的持久化.
type Repo struct {
	db *sql.DB
}

// NewRepo 创建 profile repository, 同时确保 profiles 表存在.
func NewRepo(db *sql.DB) (*Repo, error) {
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
	return &Repo{db: db}, nil
}

// Get 查询 profile. 不存在时返回 (nil, nil).
func (r *Repo) Get(id string) (*Profile, error) {
	p := &Profile{}
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

// Set 创建或更新 profile. 使用 INSERT OR REPLACE, 已存在时覆盖.
func (r *Repo) Set(p *Profile) error {
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

// Delete 删除 profile.
func (r *Repo) Delete(id string) (bool, error) {
	res, err := r.db.Exec(`DELETE FROM profiles WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// nullable 将空字符串转为 nil, 用于 SQLite 可空列写入.
func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
