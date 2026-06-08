package repo

import (
	"database/sql"
	"testing"

	"kgbrain/internal/domain/resource"

	_ "modernc.org/sqlite"
)

func newTestResourceRepo(t *testing.T) *ResourceRepo {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	repo, err := NewResourceRepo(db)
	if err != nil {
		t.Fatalf("new resource repo: %v", err)
	}
	return repo
}

func TestResourceRepoLLMCRUD(t *testing.T) {
	repo := newTestResourceRepo(t)
	temp := 0.7

	if err := repo.SaveLLM(&resource.LLMResource{
		ID:             "llm_1",
		Name:           "LLM One",
		BaseURL:        "http://localhost:8000/v1",
		APIKey:         "sk-one",
		Model:          "qwen",
		TimeoutSeconds: 180,
		Temperature:    &temp,
	}); err != nil {
		t.Fatalf("save llm: %v", err)
	}

	got, err := repo.GetLLM("llm_1")
	if err != nil {
		t.Fatalf("get llm: %v", err)
	}
	if got == nil || got.APIKey != "sk-one" || got.Temperature == nil || *got.Temperature != temp {
		t.Fatalf("unexpected llm: %#v", got)
	}
	createdAt := got.CreatedAt

	if err := repo.SaveLLM(&resource.LLMResource{
		ID:             "llm_1",
		Name:           "LLM Updated",
		BaseURL:        "http://localhost:9000/v1",
		APIKey:         "sk-two",
		Model:          "deepseek",
		TimeoutSeconds: 60,
	}); err != nil {
		t.Fatalf("update llm: %v", err)
	}

	got, err = repo.GetLLM("llm_1")
	if err != nil {
		t.Fatalf("get updated llm: %v", err)
	}
	if got.APIKey != "sk-two" || got.Model != "deepseek" || got.CreatedAt != createdAt || got.Temperature != nil {
		t.Fatalf("unexpected updated llm: %#v", got)
	}

	deleted, err := repo.DeleteLLM("llm_1")
	if err != nil || !deleted {
		t.Fatalf("delete llm: deleted=%v err=%v", deleted, err)
	}
	deleted, err = repo.DeleteLLM("llm_1")
	if err != nil || deleted {
		t.Fatalf("delete missing llm: deleted=%v err=%v", deleted, err)
	}
}

func TestResourceRepoDatabaseCRUD(t *testing.T) {
	repo := newTestResourceRepo(t)

	if err := repo.SaveDatabase(&resource.DatabaseResource{
		ID:       "db_1",
		Name:     "DB One",
		Type:     resource.DatabaseTypePostgres,
		Host:     "127.0.0.1",
		Port:     5432,
		Database: "museum",
		User:     "kgbrain",
		Password: "secret",
		SSLMode:  "disable",
	}); err != nil {
		t.Fatalf("save database: %v", err)
	}

	got, err := repo.GetDatabase("db_1")
	if err != nil {
		t.Fatalf("get database: %v", err)
	}
	if got == nil || got.Password != "secret" || got.SSLMode != "disable" {
		t.Fatalf("unexpected database: %#v", got)
	}
	createdAt := got.CreatedAt

	if err := repo.SaveDatabase(&resource.DatabaseResource{
		ID:       "db_1",
		Name:     "DB Updated",
		Type:     resource.DatabaseTypePostgres,
		Host:     "localhost",
		Port:     15432,
		Database: "museum2",
		User:     "kgbrain2",
	}); err != nil {
		t.Fatalf("update database: %v", err)
	}

	got, err = repo.GetDatabase("db_1")
	if err != nil {
		t.Fatalf("get updated database: %v", err)
	}
	if got.Password != "" || got.Database != "museum2" || got.CreatedAt != createdAt {
		t.Fatalf("unexpected updated database: %#v", got)
	}

	deleted, err := repo.DeleteDatabase("db_1")
	if err != nil || !deleted {
		t.Fatalf("delete database: deleted=%v err=%v", deleted, err)
	}
	deleted, err = repo.DeleteDatabase("db_1")
	if err != nil || deleted {
		t.Fatalf("delete missing database: deleted=%v err=%v", deleted, err)
	}
}
