package enrichextract

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func newTestEnrichExtractDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newTestEnrichExtractRepo(t *testing.T) *EnrichExtractRepo {
	t.Helper()
	repo, err := NewEnrichExtractRepo(newTestEnrichExtractDB(t))
	if err != nil {
		t.Fatalf("new enrich extract repo: %v", err)
	}
	return repo
}

func TestEnrichExtractRepoPersistsPriorityFieldHints(t *testing.T) {
	repo := newTestEnrichExtractRepo(t)
	job := &Job{
		JobID:              "ee_job_1",
		Status:             StatusPending,
		LLMResourceID:      "llm_1",
		DatabaseResourceID: "db_1",
		SourceTable:        "public.source",
		OutputTable:        "public.output",
		KeyField:           "id",
		SourceJSONField:    "raw_data",
		OutputSchema: []OutputColumn{
			{Name: "dynasty", Type: OutputColumnTypeText},
			{Name: "material", Type: OutputColumnTypeText},
		},
		TargetExample: []map[string]any{{
			"dynasty":  "",
			"material": "",
		}},
		PriorityFieldHints: map[string]string{
			"dynasty":  "朝代信息",
			"material": "材质信息",
		},
		AutoCreateOutputTable: true,
		Overwrite:             false,
		Concurrency:           1,
		PageSize:              100,
		MaxRetries:            2,
		CreatedAt:             "2026-06-11T00:00:00Z",
	}
	if err := repo.CreateJob(job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	got, err := repo.GetJob("ee_job_1")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if got == nil {
		t.Fatal("expected job, got nil")
	}
	if got.PriorityFieldHints["dynasty"] != "朝代信息" {
		t.Fatalf("unexpected dynasty hint: %#v", got.PriorityFieldHints)
	}
	if got.PriorityFieldHints["material"] != "材质信息" {
		t.Fatalf("unexpected material hint: %#v", got.PriorityFieldHints)
	}
	if !got.AutoCreateOutputTable {
		t.Fatal("expected auto_create_output_table to be persisted")
	}
	if len(got.OutputSchema) != 2 || got.OutputSchema[0].Name != "dynasty" {
		t.Fatalf("unexpected output schema: %#v", got.OutputSchema)
	}
}

func TestNewEnrichExtractRepoMigratesPriorityFieldHintsColumn(t *testing.T) {
	db := newTestEnrichExtractDB(t)
	if _, err := db.Exec(`CREATE TABLE enrich_extract_jobs (
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
		t.Fatalf("create legacy jobs table: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE enrich_extract_job_errors (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		job_id        TEXT NOT NULL,
		source_key    INTEGER,
		stage         TEXT NOT NULL,
		attempts      INTEGER NOT NULL DEFAULT 1,
		error_message TEXT NOT NULL,
		created_at    TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create errors table: %v", err)
	}
	if _, err := NewEnrichExtractRepo(db); err != nil {
		t.Fatalf("migrate repo: %v", err)
	}
	rows, err := db.Query(`PRAGMA table_info(enrich_extract_jobs)`)
	if err != nil {
		t.Fatalf("inspect jobs table: %v", err)
	}
	defer rows.Close()
	foundPriorityHints := false
	foundOutputSchema := false
	foundAutoCreateOutputTable := false
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, pk int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("scan table info: %v", err)
		}
		if name == "priority_field_hints_json" {
			foundPriorityHints = true
		}
		if name == "output_schema_json" {
			foundOutputSchema = true
		}
		if name == "auto_create_output_table" {
			foundAutoCreateOutputTable = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table info: %v", err)
	}
	if !foundPriorityHints {
		t.Fatal("expected priority_field_hints_json column to be added")
	}
	if !foundOutputSchema {
		t.Fatal("expected output_schema_json column to be added")
	}
	if !foundAutoCreateOutputTable {
		t.Fatal("expected auto_create_output_table column to be added")
	}
}
