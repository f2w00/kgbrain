package resource

import (
	"database/sql"
	"testing"

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
	maxConcurrency := 2

	if err := repo.SaveLLM(&LLMResource{
		ID:             "llm_1",
		Name:           "LLM One",
		BaseURL:        "http://localhost:8000/v1",
		APIKey:         "sk-one",
		Model:          "qwen",
		TimeoutSeconds: 180,
		Temperature:    &temp,
		MaxConcurrency: &maxConcurrency,
	}); err != nil {
		t.Fatalf("save llm: %v", err)
	}

	got, err := repo.GetLLM("llm_1")
	if err != nil {
		t.Fatalf("get llm: %v", err)
	}
	if got == nil || got.APIKey != "sk-one" || got.Temperature == nil ||
		*got.Temperature != temp || got.MaxConcurrency == nil ||
		*got.MaxConcurrency != maxConcurrency {
		t.Fatalf("unexpected llm: %#v", got)
	}
	createdAt := got.CreatedAt

	if err := repo.SaveLLM(&LLMResource{
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
	if got.APIKey != "sk-two" || got.Model != "deepseek" ||
		got.CreatedAt != createdAt || got.Temperature != nil ||
		got.MaxConcurrency != nil {
		t.Fatalf("unexpected updated llm: %#v", got)
	}

	deleted, err := repo.DeleteLLM("llm_1")
	if err != nil || !deleted {
		t.Fatalf("delete llm: deleted=%v err=%v", deleted, err)
	}

	got, err = repo.GetLLM("llm_1")
	if err != nil {
		t.Fatalf("get deleted llm: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil after deletion")
	}
}

func TestResourceRepoDatabaseCRUD(t *testing.T) {
	repo := newTestResourceRepo(t)

	if err := repo.SaveDatabase(&DatabaseResource{
		ID:       "db_1",
		Name:     "Postgres One",
		Type:     DatabaseTypePostgres,
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		User:     "testuser",
		Password: "secret",
		SSLMode:  "disable",
	}); err != nil {
		t.Fatalf("save database: %v", err)
	}

	got, err := repo.GetDatabase("db_1")
	if err != nil {
		t.Fatalf("get database: %v", err)
	}
	if got == nil || got.Host != "localhost" || got.Port != 5432 ||
		got.Database != "testdb" || got.User != "testuser" ||
		got.Password != "secret" || got.SSLMode != "disable" {
		t.Fatalf("unexpected database: %#v", got)
	}
	createdAt := got.CreatedAt

	if err := repo.SaveDatabase(&DatabaseResource{
		ID:       "db_1",
		Name:     "Postgres Updated",
		Type:     DatabaseTypePostgres,
		Host:     "db.internal",
		Port:     5433,
		Database: "newdb",
		User:     "newuser",
		Password: "newpass",
		SSLMode:  "require",
	}); err != nil {
		t.Fatalf("update database: %v", err)
	}

	got, err = repo.GetDatabase("db_1")
	if err != nil {
		t.Fatalf("get updated database: %v", err)
	}
	if got.Host != "db.internal" || got.CreatedAt != createdAt {
		t.Fatalf("unexpected updated database: %#v", got)
	}

	deleted, err := repo.DeleteDatabase("db_1")
	if err != nil || !deleted {
		t.Fatalf("delete database: deleted=%v err=%v", deleted, err)
	}

	got, err = repo.GetDatabase("db_1")
	if err != nil {
		t.Fatalf("get deleted database: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil after deletion")
	}
}

func TestResourceRepoEmbeddingCRUD(t *testing.T) {
	repo := newTestResourceRepo(t)
	maxConcurrency := 4

	if err := repo.SaveEmbedding(&EmbeddingResource{
		ID:             "emb_1",
		Name:           "Embedding One",
		BaseURL:        "http://localhost:8000/v1",
		APIKey:         "sk-emb",
		Model:          "text-embedding-3-small",
		TimeoutSeconds: 30,
		MaxConcurrency: &maxConcurrency,
	}); err != nil {
		t.Fatalf("save embedding: %v", err)
	}

	got, err := repo.GetEmbedding("emb_1")
	if err != nil {
		t.Fatalf("get embedding: %v", err)
	}
	if got == nil || got.APIKey != "sk-emb" || got.Model != "text-embedding-3-small" ||
		got.MaxConcurrency == nil || *got.MaxConcurrency != maxConcurrency {
		t.Fatalf("unexpected embedding: %#v", got)
	}
	createdAt := got.CreatedAt

	if err := repo.SaveEmbedding(&EmbeddingResource{
		ID:             "emb_1",
		Name:           "Embedding Updated",
		BaseURL:        "http://localhost:9000/v1",
		APIKey:         "sk-emb-2",
		Model:          "bge-m3",
		TimeoutSeconds: 45,
	}); err != nil {
		t.Fatalf("update embedding: %v", err)
	}

	got, err = repo.GetEmbedding("emb_1")
	if err != nil {
		t.Fatalf("get updated embedding: %v", err)
	}
	if got.APIKey != "sk-emb-2" || got.Model != "bge-m3" ||
		got.CreatedAt != createdAt || got.MaxConcurrency != nil {
		t.Fatalf("unexpected updated embedding: %#v", got)
	}

	deleted, err := repo.DeleteEmbedding("emb_1")
	if err != nil || !deleted {
		t.Fatalf("delete embedding: deleted=%v err=%v", deleted, err)
	}

	got, err = repo.GetEmbedding("emb_1")
	if err != nil {
		t.Fatalf("get deleted embedding: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil after deletion")
	}
}
