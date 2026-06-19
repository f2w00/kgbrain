package processrecord

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

const writeBatchSize = 100

var identPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Repository 在业务库中维护 data_process_records 表。
type Repository struct {
	db *sql.DB
}

// NewRepository 创建处理状态仓储，并确保业务库中存在状态表。
func NewRepository(db *sql.DB) (*Repository, error) {
	r := &Repository{db: db}
	if err := r.ensureTable(context.Background()); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Repository) ensureTable(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS data_process_records (
		source_key BIGINT PRIMARY KEY,
		process_type TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("create data_process_records table: %w", err)
	}
	return nil
}

// UpsertMany 批量登记处理状态；相同业务数据会覆盖为最新整体状态。
func (r *Repository) UpsertMany(ctx context.Context, records []Record) error {
	if len(records) == 0 {
		return nil
	}
	for start := 0; start < len(records); start += writeBatchSize {
		end := min(start+writeBatchSize, len(records))
		if err := r.upsertBatch(ctx, records[start:end]); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) upsertBatch(ctx context.Context, records []Record) error {
	valueRows := make([]string, 0, len(records))
	args := make([]any, 0, len(records)*3)
	argIndex := 1
	for _, record := range records {
		valueRows = append(valueRows, rowPlaceholders(argIndex, 3))
		args = append(args, record.SourceKey, record.ProcessType, record.Status)
		argIndex += 3
	}
	query := fmt.Sprintf(`INSERT INTO data_process_records (
		source_key, process_type, status
	) VALUES %s
	ON CONFLICT (source_key)
	DO UPDATE SET process_type = EXCLUDED.process_type,
		status = EXCLUDED.status,
		updated_at = CURRENT_TIMESTAMP`,
		strings.Join(valueRows, ", "),
	)
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("upsert data process records: %w", err)
	}
	return nil
}

func rowPlaceholders(start int, count int) string {
	parts := make([]string, count)
	for i := 0; i < count; i++ {
		parts[i] = fmt.Sprintf("$%d", start+i)
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// UpsertSourceRange 将源表范围内的所有主键统一登记为指定处理状态。
func (r *Repository) UpsertSourceRange(ctx context.Context, record SourceRangeRecord) error {
	tableName, err := parseQualifiedName(record.SourceTable)
	if err != nil {
		return fmt.Errorf("parse source_table: %w", err)
	}
	if !identPattern.MatchString(record.KeyField) {
		return fmt.Errorf("invalid key_field %q", record.KeyField)
	}
	args := []any{record.ProcessType, record.Status}
	where := []string{fmt.Sprintf("s.%s IS NOT NULL", quoteIdent(record.KeyField))}
	if record.StartID != nil {
		args = append(args, *record.StartID)
		where = append(where, fmt.Sprintf("s.%s >= $%d", quoteIdent(record.KeyField), len(args)))
	}
	if record.EndID != nil {
		args = append(args, *record.EndID)
		where = append(where, fmt.Sprintf("s.%s <= $%d", quoteIdent(record.KeyField), len(args)))
	}
	query := fmt.Sprintf(`INSERT INTO data_process_records (
		source_key, process_type, status
	)
	SELECT s.%s, $1, $2
	FROM %s s
	WHERE %s
	ON CONFLICT (source_key)
	DO UPDATE SET process_type = EXCLUDED.process_type,
		status = EXCLUDED.status,
		updated_at = CURRENT_TIMESTAMP`,
		quoteIdent(record.KeyField),
		fullTableName(tableName),
		strings.Join(where, " AND "),
	)
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("upsert data process records by source range: %w", err)
	}
	return nil
}

type qualifiedName struct {
	Schema string
	Name   string
}

func parseQualifiedName(raw string) (qualifiedName, error) {
	parts := strings.Split(raw, ".")
	if len(parts) == 1 {
		if !identPattern.MatchString(parts[0]) {
			return qualifiedName{}, fmt.Errorf("invalid table name %q", raw)
		}
		return qualifiedName{Name: parts[0]}, nil
	}
	if len(parts) != 2 || !identPattern.MatchString(parts[0]) || !identPattern.MatchString(parts[1]) {
		return qualifiedName{}, fmt.Errorf("invalid table name %q", raw)
	}
	return qualifiedName{Schema: parts[0], Name: parts[1]}, nil
}

func quoteIdent(ident string) string {
	return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
}

func fullTableName(name qualifiedName) string {
	if name.Schema == "" {
		return quoteIdent(name.Name)
	}
	return quoteIdent(name.Schema) + "." + quoteIdent(name.Name)
}
