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
	CacheKey     string
	Mapping      string
	SourceFields string
	TargetFields string
	CreatedAt    string
}

func NewCacheRepo(db *sql.DB) (*CacheRepo, error) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS mapping_cache (
		cache_key     TEXT PRIMARY KEY,
		mapping       TEXT NOT NULL,
		source_fields TEXT NOT NULL,
		target_fields TEXT NOT NULL,
		created_at    TEXT NOT NULL
	)`)
	if err != nil {
		return nil, fmt.Errorf("create mapping_cache table: %w", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS content_mapping (
		topic      TEXT NOT NULL,
		mapping    TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY (topic)
	)`)
	if err != nil {
		return nil, fmt.Errorf("create content_mapping table: %w", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS content_mapping_targets (
		topic      TEXT NOT NULL,
		targets    TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY (topic)
	)`)
	if err != nil {
		return nil, fmt.Errorf("create content_mapping_targets table: %w", err)
	}

	return &CacheRepo{db: db}, nil
}

func (r *CacheRepo) Get(cacheKey string) (*mapping.Mapping, error) {
	row := &cacheRow{}
	err := r.db.QueryRow(
		`SELECT cache_key, mapping, source_fields, target_fields, created_at
		FROM mapping_cache WHERE cache_key = ?`, cacheKey).
		Scan(&row.CacheKey, &row.Mapping, &row.SourceFields, &row.TargetFields, &row.CreatedAt)
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

func (r *CacheRepo) Save(cacheKey string, m *mapping.Mapping, source, target []string) error {
	mJSON, _ := json.Marshal(m)
	sJSON, _ := json.Marshal(source)
	tJSON, _ := json.Marshal(target)
	_, err := r.db.Exec(`INSERT OR REPLACE INTO mapping_cache
		(cache_key, mapping, source_fields, target_fields, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		cacheKey, string(mJSON), string(sJSON), string(tJSON),
		time.Now().UTC().Format(time.RFC3339))
	return err
}

func (r *CacheRepo) GetContentMapping(topic string) (mapping.ContentMapping, error) {
	var mappingJSON string
	var updatedAt string
	err := r.db.QueryRow(
		`SELECT mapping, updated_at FROM content_mapping WHERE topic = ?`, topic).
		Scan(&mappingJSON, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var m mapping.ContentMapping
	if err := json.Unmarshal([]byte(mappingJSON), &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (r *CacheRepo) SaveContentMapping(topic string, m mapping.ContentMapping) error {
	mJSON, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal content mapping: %w", err)
	}
	_, err = r.db.Exec(`INSERT OR REPLACE INTO content_mapping
		(topic, mapping, updated_at)
		VALUES (?, ?, ?)`,
		topic, string(mJSON), time.Now().UTC().Format(time.RFC3339))
	return err
}

func (r *CacheRepo) GetTargets(topic string) ([]string, error) {
	var targetsJSON string
	err := r.db.QueryRow(
		`SELECT targets FROM content_mapping_targets WHERE topic = ?`, topic).
		Scan(&targetsJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var targets []string
	if err := json.Unmarshal([]byte(targetsJSON), &targets); err != nil {
		return nil, err
	}
	return targets, nil
}

func (r *CacheRepo) SaveTargets(topic string, targets []string) error {
	tJSON, err := json.Marshal(targets)
	if err != nil {
		return fmt.Errorf("marshal targets: %w", err)
	}
	_, err = r.db.Exec(`INSERT OR REPLACE INTO content_mapping_targets
		(topic, targets, updated_at)
		VALUES (?, ?, ?)`,
		topic, string(tJSON), time.Now().UTC().Format(time.RFC3339))
	return err
}
