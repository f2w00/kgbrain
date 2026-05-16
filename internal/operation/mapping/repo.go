package mapping

import (
	"database/sql"
	"fmt"
)

// MappingRepo 管理 mapping_cache 表的持久化.
type MappingRepo struct {
	db *sql.DB
}

// NewMappingRepo 创建 mapping repository, 同时确保 mapping_cache 表存在.
// mapping_cache 按 (profile_id, cache_key) 联合主键, 支持同一用户的多组字段组合缓存.
func NewMappingRepo(db *sql.DB) (*MappingRepo, error) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS mapping_cache (
		profile_id    TEXT NOT NULL,
		cache_key     TEXT NOT NULL,
		mapping       TEXT NOT NULL,
		source_fields TEXT NOT NULL,
		target_fields TEXT NOT NULL,
		created_at    TEXT NOT NULL,
		PRIMARY KEY (profile_id, cache_key)
	)`)
	if err != nil {
		return nil, fmt.Errorf("create mapping_cache table: %w", err)
	}
	return &MappingRepo{db: db}, nil
}

// Get 查询缓存的 mapping 结果. 不存在时返回 (nil, nil), 调用方通过判 nil 区分"未命中"和"查询错误".
func (r *MappingRepo) Get(profileID, cacheKey string) (*CacheRow, error) {
	row := &CacheRow{}
	err := r.db.QueryRow(
		`SELECT profile_id, cache_key, mapping, source_fields, target_fields, created_at
		FROM mapping_cache WHERE profile_id = ? AND cache_key = ?`, profileID, cacheKey).
		Scan(&row.ProfileID, &row.CacheKey, &row.Mapping, &row.SourceFields, &row.TargetFields, &row.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return row, nil
}

// Save 缓存 mapping 生成结果.
func (r *MappingRepo) Save(row *CacheRow) error {
	_, err := r.db.Exec(`INSERT OR REPLACE INTO mapping_cache
		(profile_id, cache_key, mapping, source_fields, target_fields, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		row.ProfileID, row.CacheKey, row.Mapping, row.SourceFields, row.TargetFields, row.CreatedAt)
	return err
}

// ClearByProfile 清空某个 profile 的所有 mapping 缓存.
func (r *MappingRepo) ClearByProfile(profileID string) error {
	_, err := r.db.Exec(`DELETE FROM mapping_cache WHERE profile_id = ?`, profileID)
	return err
}
