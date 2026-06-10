// business_repository.go 提供业务 Postgres 侧的结构化抽取读写能力。
package enrichextract

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const defaultSchema = "public"

var identPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type qualifiedName struct {
	Schema string
	Name   string
}

type EnrichExtractBusinessRepo struct {
	db *sql.DB
}

func NewEnrichExtractBusinessRepo(db *sql.DB) *EnrichExtractBusinessRepo {
	return &EnrichExtractBusinessRepo{db: db}
}

func (r *EnrichExtractBusinessRepo) EnsureExecutionReady(
	ctx context.Context,
	req ExecuteRequest,
) error {
	sourceTable, err := parseQualifiedName(req.SourceTable)
	if err != nil {
		return fmt.Errorf("parse source_table: %w", err)
	}
	outputTable, err := parseQualifiedName(req.OutputTable)
	if err != nil {
		return fmt.Errorf("parse output_table: %w", err)
	}
	sourceColumns, err := r.loadColumns(ctx, sourceTable)
	if err != nil {
		return err
	}
	if len(sourceColumns) == 0 {
		return fmt.Errorf("source_table does not exist")
	}
	sourceKey, err := requireColumn(sourceColumns, req.KeyField)
	if err != nil {
		return fmt.Errorf("source_table: %w", err)
	}
	if !isIntegerColumn(sourceKey) {
		return fmt.Errorf("source key_field must be smallint, integer, or bigint")
	}
	sourceJSON, err := requireColumn(sourceColumns, req.SourceJSONField)
	if err != nil {
		return fmt.Errorf("source_table: %w", err)
	}
	if sourceJSON.UDTName != "jsonb" {
		return fmt.Errorf("source_json_field must be jsonb")
	}
	if err := r.ensureUniqueKey(ctx, sourceTable, req.KeyField); err != nil {
		return fmt.Errorf("source_table: %w", err)
	}

	outputColumns, err := r.loadColumns(ctx, outputTable)
	if err != nil {
		return err
	}
	if len(outputColumns) == 0 {
		return fmt.Errorf("output_table does not exist")
	}
	outputKey, err := requireColumn(outputColumns, req.KeyField)
	if err != nil {
		return fmt.Errorf("output_table: %w", err)
	}
	if !isIntegerColumn(outputKey) {
		return fmt.Errorf("output key_field must be smallint, integer, or bigint")
	}
	if err := r.ensureUniqueKey(ctx, outputTable, req.KeyField); err != nil {
		return fmt.Errorf("output_table: %w", err)
	}
	for _, field := range req.TargetFields {
		if _, err := requireColumn(outputColumns, field); err != nil {
			return fmt.Errorf("output_table: %w", err)
		}
	}
	return nil
}

func (r *EnrichExtractBusinessRepo) SelectSourcePage(
	ctx context.Context,
	req ExecuteRequest,
	lastKey *int64,
) ([]SourceRow, error) {
	sourceTable, err := parseQualifiedName(req.SourceTable)
	if err != nil {
		return nil, fmt.Errorf("parse source_table: %w", err)
	}
	outputTable, err := parseQualifiedName(req.OutputTable)
	if err != nil {
		return nil, fmt.Errorf("parse output_table: %w", err)
	}
	args := make([]any, 0, 4)
	where := make([]string, 0, 4)
	keyExpr := "s." + quoteIdent(req.KeyField)
	if lastKey != nil {
		args = append(args, *lastKey)
		where = append(where, fmt.Sprintf("%s > $%d", keyExpr, len(args)))
	} else if req.StartID != nil {
		args = append(args, *req.StartID-1)
		where = append(where, fmt.Sprintf("%s > $%d", keyExpr, len(args)))
	}
	if req.EndID != nil {
		args = append(args, *req.EndID)
		where = append(where, fmt.Sprintf("%s <= $%d", keyExpr, len(args)))
	}
	if !req.Overwrite {
		where = append(where, "o."+quoteIdent(req.KeyField)+" IS NULL")
	}
	whereSQL := "TRUE"
	if len(where) > 0 {
		whereSQL = strings.Join(where, " AND ")
	}
	args = append(args, req.PageSize)
	join := ""
	if !req.Overwrite {
		join = fmt.Sprintf(
			"LEFT JOIN %s o ON o.%s = s.%s",
			fullTableName(outputTable),
			quoteIdent(req.KeyField),
			quoteIdent(req.KeyField),
		)
	}
	query := fmt.Sprintf(`
		SELECT s.%s, s.%s
		FROM %s s
		%s
		WHERE %s
		ORDER BY s.%s
		LIMIT $%d`,
		quoteIdent(req.KeyField),
		quoteIdent(req.SourceJSONField),
		fullTableName(sourceTable),
		join,
		whereSQL,
		quoteIdent(req.KeyField),
		len(args),
	)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select source page: %w", err)
	}
	defer rows.Close()
	result := make([]SourceRow, 0, req.PageSize)
	for rows.Next() {
		var row SourceRow
		var raw []byte
		if err := rows.Scan(&row.Key, &raw); err != nil {
			return nil, fmt.Errorf("scan source row: %w", err)
		}
		row.Raw = json.RawMessage(append([]byte(nil), raw...))
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate source rows: %w", err)
	}
	return result, nil
}

func (r *EnrichExtractBusinessRepo) BatchWriteOutputRows(
	ctx context.Context,
	req ExecuteRequest,
	rows []OutputRow,
) error {
	for start := 0; start < len(rows); start += WriteBatchSize {
		end := min(start+WriteBatchSize, len(rows))
		if err := r.writeOutputRows(ctx, req, rows[start:end]); err != nil {
			time.Sleep(500 * time.Millisecond)
			if retryErr := r.writeOutputRows(ctx, req, rows[start:end]); retryErr == nil {
				continue
			}
			return err
		}
	}
	return nil
}

func (r *EnrichExtractBusinessRepo) WriteOutputRow(
	ctx context.Context,
	req ExecuteRequest,
	row OutputRow,
) error {
	return r.writeOutputRows(ctx, req, []OutputRow{row})
}

func (r *EnrichExtractBusinessRepo) writeOutputRows(
	ctx context.Context,
	req ExecuteRequest,
	rows []OutputRow,
) error {
	if len(rows) == 0 {
		return nil
	}
	outputTable, err := parseQualifiedName(req.OutputTable)
	if err != nil {
		return fmt.Errorf("parse output_table: %w", err)
	}
	columns := append([]string{req.KeyField}, req.TargetFields...)
	quotedColumns := make([]string, 0, len(columns))
	for _, column := range columns {
		quotedColumns = append(quotedColumns, quoteIdent(column))
	}
	valueRows := make([]string, 0, len(rows))
	args := make([]any, 0, len(rows)*len(columns))
	argIndex := 1
	for _, row := range rows {
		valueRows = append(valueRows, rowPlaceholders(argIndex, len(columns)))
		args = append(args, row.Key)
		for _, field := range req.TargetFields {
			args = append(args, row.Values[field])
		}
		argIndex += len(columns)
	}
	conflict := "DO NOTHING"
	if req.Overwrite {
		sets := make([]string, 0, len(req.TargetFields))
		for _, field := range req.TargetFields {
			sets = append(sets, fmt.Sprintf("%s = EXCLUDED.%s", quoteIdent(field), quoteIdent(field)))
		}
		conflict = "DO UPDATE SET " + strings.Join(sets, ", ")
	}
	query := fmt.Sprintf(`
		INSERT INTO %s (%s)
		VALUES %s
		ON CONFLICT (%s) %s`,
		fullTableName(outputTable),
		strings.Join(quotedColumns, ", "),
		strings.Join(valueRows, ", "),
		quoteIdent(req.KeyField),
		conflict,
	)
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("write output rows: %w", err)
	}
	return nil
}

func (r *EnrichExtractBusinessRepo) loadColumns(
	ctx context.Context,
	table qualifiedName,
) ([]ColumnMeta, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT column_name, udt_name, format_type(a.atttypid, a.atttypmod)
		FROM information_schema.columns c
		JOIN pg_catalog.pg_class cls ON cls.relname = c.table_name
		JOIN pg_catalog.pg_namespace n ON n.oid = cls.relnamespace
		JOIN pg_catalog.pg_attribute a ON a.attrelid = cls.oid AND a.attname = c.column_name
		WHERE c.table_schema = $1 AND c.table_name = $2 AND n.nspname = $1
		ORDER BY c.ordinal_position`, table.Schema, table.Name)
	if err != nil {
		return nil, fmt.Errorf("load columns for %s: %w", fullTableName(table), err)
	}
	defer rows.Close()
	columns := make([]ColumnMeta, 0)
	for rows.Next() {
		var column ColumnMeta
		if err := rows.Scan(&column.Name, &column.UDTName, &column.FormattedType); err != nil {
			return nil, fmt.Errorf("scan columns for %s: %w", fullTableName(table), err)
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate columns for %s: %w", fullTableName(table), err)
	}
	return columns, nil
}

func (r *EnrichExtractBusinessRepo) ensureUniqueKey(
	ctx context.Context,
	table qualifiedName,
	keyField string,
) error {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM pg_index i
		JOIN pg_class t ON t.oid = i.indrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(i.indkey)
		WHERE n.nspname = $1
		  AND t.relname = $2
		  AND i.indisunique
		  AND i.indnkeyatts = 1
		  AND a.attname = $3`, table.Schema, table.Name, keyField).Scan(&count)
	if err != nil {
		return fmt.Errorf("check unique key: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("key_field must have primary key or unique index")
	}
	return nil
}

func parseQualifiedName(raw string) (qualifiedName, error) {
	parts := strings.Split(raw, ".")
	if len(parts) == 1 {
		if !identPattern.MatchString(parts[0]) {
			return qualifiedName{}, fmt.Errorf("invalid table name %q", raw)
		}
		return qualifiedName{Schema: defaultSchema, Name: parts[0]}, nil
	}
	if len(parts) != 2 || !identPattern.MatchString(parts[0]) || !identPattern.MatchString(parts[1]) {
		return qualifiedName{}, fmt.Errorf("invalid table name %q", raw)
	}
	return qualifiedName{Schema: parts[0], Name: parts[1]}, nil
}

func requireColumn(columns []ColumnMeta, name string) (ColumnMeta, error) {
	for _, column := range columns {
		if column.Name == name {
			return column, nil
		}
	}
	return ColumnMeta{}, fmt.Errorf("column %q does not exist", name)
}

func isIntegerColumn(column ColumnMeta) bool {
	switch column.UDTName {
	case "int2", "int4", "int8":
		return true
	default:
		return false
	}
}

func quoteIdent(ident string) string {
	return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
}

func fullTableName(name qualifiedName) string {
	return quoteIdent(name.Schema) + "." + quoteIdent(name.Name)
}

func rowPlaceholders(start int, count int) string {
	parts := make([]string, count)
	for i := 0; i < count; i++ {
		parts[i] = fmt.Sprintf("$%d", start+i)
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

var _ BusinessRepository = (*EnrichExtractBusinessRepo)(nil)
