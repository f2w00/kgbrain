package processrecord

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestRepositoryUpsertMany(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	repo, err := NewRepository(db)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}
	ctx := context.Background()
	if err := repo.UpsertMany(ctx, []Record{
		{
			SourceTable: "papers",
			SourceKey:   1,
			ProcessType: "enrich_extract",
			Status:      StatusFailed,
		},
		{
			SourceTable: "papers",
			SourceKey:   2,
			ProcessType: "enrich_extract",
			Status:      StatusSucceeded,
		},
	}); err != nil {
		t.Fatalf("upsert records: %v", err)
	}
	if err := repo.UpsertMany(ctx, []Record{
		{
			SourceTable: "papers",
			SourceKey:   1,
			ProcessType: "enrich_extract",
			Status:      StatusSucceeded,
		},
	}); err != nil {
		t.Fatalf("update record: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(1) FROM data_process_records`).Scan(&count); err != nil {
		t.Fatalf("count records: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 records, got %d", count)
	}

	var status string
	err = db.QueryRow(`SELECT status FROM data_process_records
		WHERE source_table = ? AND source_key = ? AND process_type = ?`,
		"papers", 1, "enrich_extract",
	).Scan(&status)
	if err != nil {
		t.Fatalf("query updated status: %v", err)
	}
	if status != StatusSucceeded {
		t.Fatalf("expected status %q, got %q", StatusSucceeded, status)
	}
}

func TestRepositoryUpsertManyEmpty(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	repo, err := NewRepository(db)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}
	if err := repo.UpsertMany(context.Background(), nil); err != nil {
		t.Fatalf("upsert empty records: %v", err)
	}
}

func TestRepositoryUpsertSourceRange(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE papers (id INTEGER PRIMARY KEY, title TEXT)`); err != nil {
		t.Fatalf("create source table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO papers (id, title) VALUES (1, 'a'), (2, 'b'), (3, 'c')`); err != nil {
		t.Fatalf("seed source table: %v", err)
	}
	repo, err := NewRepository(db)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}
	startID := int64(2)
	if err := repo.UpsertSourceRange(context.Background(), SourceRangeRecord{
		SourceTable: "papers",
		KeyField:    "id",
		StartID:     &startID,
		ProcessType: "entity_alignment",
		Status:      StatusSucceeded,
	}); err != nil {
		t.Fatalf("upsert source range: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(1) FROM data_process_records`).Scan(&count); err != nil {
		t.Fatalf("count records: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 records, got %d", count)
	}

	var status string
	err = db.QueryRow(`SELECT status FROM data_process_records
		WHERE source_table = ? AND source_key = ? AND process_type = ?`,
		"papers", 3, "entity_alignment",
	).Scan(&status)
	if err != nil {
		t.Fatalf("query range status: %v", err)
	}
	if status != StatusSucceeded {
		t.Fatalf("expected status %q, got %q", StatusSucceeded, status)
	}
}
