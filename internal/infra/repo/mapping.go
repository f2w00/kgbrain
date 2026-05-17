package repo

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"kgbrain/internal/domain/mapping"
)

type CacheRepo struct {
	db *sql.DB
}

type cacheRow struct {
	ProfileID    string
	CacheKey     string
	Mapping      string
	SourceFields string
	TargetFields string
	CreatedAt    string
}

func NewCacheRepo(db *sql.DB) (*CacheRepo, error) {
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
	return &CacheRepo{db: db}, nil
}

func (r *CacheRepo) Get(profileID, cacheKey string) (*mapping.Mapping, error) {
	row := &cacheRow{}
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

	var m mapping.Mapping
	if err := json.Unmarshal([]byte(row.Mapping), &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *CacheRepo) Save(profileID, cacheKey string, m *mapping.Mapping, source, target []string) error {
	mJSON, _ := json.Marshal(m)
	sJSON, _ := json.Marshal(source)
	tJSON, _ := json.Marshal(target)
	_, err := r.db.Exec(`INSERT OR REPLACE INTO mapping_cache
		(profile_id, cache_key, mapping, source_fields, target_fields, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		profileID, cacheKey, string(mJSON), string(sJSON), string(tJSON),
		time.Now().UTC().Format(time.RFC3339))
	return err
}

func (r *CacheRepo) ClearByProfile(profileID string) error {
	_, err := r.db.Exec(`DELETE FROM mapping_cache WHERE profile_id = ?`, profileID)
	return err
}
