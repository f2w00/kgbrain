// business_repository.go 提供业务 Postgres 侧的实体对齐数据读写能力。
package alignment

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	defaultSchema         = "public"
	mappingTableName      = "entity_alignment_mapping"
	outputTableAlias      = "s"
	mappingTableAlias     = "m_"
	defaultOutputPageSize = int64(10000)
)

var identPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type qualifiedName struct {
	Schema string
	Name   string
}

type outputKeyRange struct {
	Start int64
	End   int64
}

type outputPage struct {
	Start int64
	End   int64
}

// EntityAlignmentBusinessRepo 实现业务 Postgres 上的实体对齐读写能力。
type EntityAlignmentBusinessRepo struct {
	db *sql.DB
}

// NewEntityAlignmentBusinessRepo 创建业务库侧实体对齐仓储。
func NewEntityAlignmentBusinessRepo(db *sql.DB) *EntityAlignmentBusinessRepo {
	return &EntityAlignmentBusinessRepo{db: db}
}

// EnsureExecutionReady 校验源表和输出表结构，确保 mapping 表和输出表已就绪。
func (r *EntityAlignmentBusinessRepo) EnsureExecutionReady(
	ctx context.Context,
	req ExecuteRequest,
	fields []PreparedField,
) ([]ColumnMeta, error) {
	sourceTable, err := parseQualifiedName(req.SourceTable)
	if err != nil {
		return nil, fmt.Errorf("parse source_table: %w", err)
	}
	outputTable, err := parseQualifiedName(req.OutputTable)
	if err != nil {
		return nil, fmt.Errorf("parse output_table: %w", err)
	}
	mappingTable := qualifiedName{Schema: sourceTable.Schema, Name: mappingTableName}

	sourceColumns, err := r.loadColumns(ctx, sourceTable)
	if err != nil {
		return nil, err
	}
	if len(sourceColumns) == 0 {
		return nil, fmt.Errorf("source_table does not exist")
	}
	keyColumn, err := requireColumn(sourceColumns, req.KeyField)
	if err != nil {
		return nil, fmt.Errorf("source_table: %w", err)
	}
	if !isIntegerColumn(keyColumn) {
		return nil, fmt.Errorf("key_field must be smallint, integer, or bigint")
	}
	for _, field := range fields {
		column, err := requireColumn(sourceColumns, field.Name)
		if err != nil {
			return nil, fmt.Errorf("source_table: %w", err)
		}
		if !isTextCompatibleColumn(column) {
			return nil, fmt.Errorf(
				"alignment field %q must be text, varchar, or char compatible",
				field.Name,
			)
		}
	}
	if err := r.ensureMappingTable(ctx, mappingTable); err != nil {
		return nil, err
	}
	outputColumns, err := r.ensureOutputTable(ctx, outputTable, sourceColumns)
	if err != nil {
		return nil, err
	}
	if _, err := requireColumn(outputColumns, req.KeyField); err != nil {
		return nil, fmt.Errorf("output_table: %w", err)
	}
	if err := r.ensureOutputUniqueKey(ctx, outputTable, req.KeyField); err != nil {
		return nil, err
	}
	if err := r.ensureSourceKeyNotNull(
		ctx,
		sourceTable,
		req.KeyField,
		req.StartID,
		req.EndID,
	); err != nil {
		return nil, err
	}
	return sourceColumns, nil
}

// SelectDistinctRawValues 查询指定字段在范围内的非空不重复原始值。
func (r *EntityAlignmentBusinessRepo) SelectDistinctRawValues(
	ctx context.Context,
	req ExecuteRequest,
	fieldName string,
) ([]string, error) {
	sourceTable, err := parseQualifiedName(req.SourceTable)
	if err != nil {
		return nil, fmt.Errorf("parse source_table: %w", err)
	}
	where, args := buildRangeClause(
		outputTableAlias+"."+quoteIdent(req.KeyField),
		req.StartID,
		req.EndID,
		1,
	)
	query := fmt.Sprintf(`
		SELECT DISTINCT %s.%s::text
		FROM %s %s
		WHERE %s.%s IS NOT NULL
		  AND btrim(%s.%s::text) <> ''%s`,
		outputTableAlias,
		quoteIdent(fieldName),
		fullTableName(sourceTable),
		outputTableAlias,
		outputTableAlias,
		quoteIdent(fieldName),
		outputTableAlias,
		quoteIdent(fieldName),
		where,
	)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf(
			"select distinct raw values for %q: %w",
			fieldName,
			err,
		)
	}
	defer rows.Close()
	values := make([]string, 0)
	for rows.Next() {
		var rawValue string
		if err := rows.Scan(&rawValue); err != nil {
			return nil, fmt.Errorf("scan raw value for %q: %w", fieldName, err)
		}
		values = append(values, rawValue)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate raw values for %q: %w", fieldName, err)
	}
	return values, nil
}

// LoadExistingMappings 加载指定字段和原始值集合对应的已有 mapping 记录。
func (r *EntityAlignmentBusinessRepo) LoadExistingMappings(
	ctx context.Context,
	req ExecuteRequest,
	field PreparedField,
	rawValues []string,
) (map[string]MappingRecord, error) {
	result := make(map[string]MappingRecord, len(rawValues))
	if len(rawValues) == 0 {
		return result, nil
	}
	placeholders := dollarList(3, len(rawValues))
	args := make([]any, 0, len(rawValues)+2)
	args = append(args, field.Name, field.TargetHash)
	for _, rawValue := range rawValues {
		args = append(args, rawValue)
	}
	mappingTable, err := mappingTableNameFor(req.SourceTable)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		SELECT raw_value, aligned_value, status
		FROM %s
		WHERE field_name = $1
		  AND target_hash = $2
		  AND raw_value IN (%s)`,
		fullTableName(mappingTable),
		placeholders,
	)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("load existing mappings for %q: %w", field.Name, err)
	}
	defer rows.Close()
	for rows.Next() {
		var rawValue string
		var alignedValue sql.NullString
		var status string
		if err := rows.Scan(&rawValue, &alignedValue, &status); err != nil {
			return nil, fmt.Errorf("scan existing mapping for %q: %w", field.Name, err)
		}
		result[rawValue] = MappingRecord{
			RawValue:     rawValue,
			AlignedValue: nullableString(alignedValue),
			Status:       status,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate existing mappings for %q: %w", field.Name, err)
	}
	return result, nil
}

// TouchMappings 批量更新已有 mapping 的使用计数和最后访问时间。
func (r *EntityAlignmentBusinessRepo) TouchMappings(
	ctx context.Context,
	req ExecuteRequest,
	field PreparedField,
	records map[string]MappingRecord,
) error {
	rawValues := make([]string, 0, len(records))
	for rawValue := range records {
		rawValues = append(rawValues, rawValue)
	}
	if len(rawValues) == 0 {
		return nil
	}
	placeholders := dollarList(4, len(rawValues))
	now := time.Now().UTC()
	args := []any{now, field.Name, field.TargetHash}
	for _, rawValue := range rawValues {
		args = append(args, rawValue)
	}
	mappingTable, err := mappingTableNameFor(req.SourceTable)
	if err != nil {
		return err
	}
	query := fmt.Sprintf(`
		UPDATE %s
		SET last_used_at = $1, use_count = use_count + 1
		WHERE field_name = $2
		  AND target_hash = $3
		  AND raw_value IN (%s)`,
		fullTableName(mappingTable),
		placeholders,
	)
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("touch existing mappings for %q: %w", field.Name, err)
	}
	return nil
}

// UpsertMappings 批量写入 mapping 结果，支持追加和覆盖两种模式。
func (r *EntityAlignmentBusinessRepo) UpsertMappings(
	ctx context.Context,
	req ExecuteRequest,
	field PreparedField,
	records []MappingRecord,
	overwrite bool,
) error {
	if len(records) == 0 {
		return nil
	}
	now := time.Now().UTC()
	valueRows := make([]string, 0, len(records))
	args := make([]any, 0, len(records)*9)
	argIndex := 1
	for _, record := range records {
		valueRows = append(valueRows, rowPlaceholders(argIndex, 9))
		args = append(
			args,
			field.Name,
			field.TargetHash,
			field.TargetsJSON,
			record.RawValue,
			record.AlignedValue,
			record.Status,
			now,
			now,
			now,
		)
		argIndex += 9
	}
	mappingTable, err := mappingTableNameFor(req.SourceTable)
	if err != nil {
		return err
	}
	conflictSet := `
		last_used_at = EXCLUDED.last_used_at,
		updated_at = EXCLUDED.updated_at,
		use_count = ` + fullTableName(mappingTable) + `.use_count + 1`
	if overwrite {
		conflictSet = `
			targets_json = EXCLUDED.targets_json,
			aligned_value = EXCLUDED.aligned_value,
			status = EXCLUDED.status,
			last_used_at = EXCLUDED.last_used_at,
			updated_at = EXCLUDED.updated_at,
			use_count = ` + fullTableName(mappingTable) + `.use_count + 1`
	}
	query := fmt.Sprintf(`
		INSERT INTO %s (
			field_name,
			target_hash,
			targets_json,
			raw_value,
			aligned_value,
			status,
			created_at,
			updated_at,
			last_used_at
		)
		VALUES %s
		ON CONFLICT (field_name, target_hash, raw_value) DO UPDATE
		SET %s`,
		fullTableName(mappingTable),
		strings.Join(valueRows, ", "),
		conflictSet,
	)
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("upsert mappings for field %q: %w", field.Name, err)
	}
	return nil
}

// WriteOutputRows 将对齐结果分页合并写入输出表，避免单条大 SQL 长时间持锁。
func (r *EntityAlignmentBusinessRepo) WriteOutputRows(
	ctx context.Context,
	req ExecuteRequest,
	sourceColumns []ColumnMeta,
	fields []PreparedField,
) error {
	sourceTable, err := parseQualifiedName(req.SourceTable)
	if err != nil {
		return fmt.Errorf("parse source_table: %w", err)
	}
	outputTable, err := parseQualifiedName(req.OutputTable)
	if err != nil {
		return fmt.Errorf("parse output_table: %w", err)
	}
	mappingTable := qualifiedName{Schema: sourceTable.Schema, Name: mappingTableName}
	keyRange, ok, err := r.selectOutputKeyRange(ctx, sourceTable, req)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	for _, page := range buildOutputPages(
		keyRange.Start,
		keyRange.End,
		defaultOutputPageSize,
	) {
		if err := r.writeOutputRowsRange(
			ctx,
			req,
			sourceTable,
			outputTable,
			mappingTable,
			sourceColumns,
			fields,
			page,
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *EntityAlignmentBusinessRepo) selectOutputKeyRange(
	ctx context.Context,
	sourceTable qualifiedName,
	req ExecuteRequest,
) (outputKeyRange, bool, error) {
	where, args := buildRangeClause(
		outputTableAlias+"."+quoteIdent(req.KeyField),
		req.StartID,
		req.EndID,
		1,
	)
	query := fmt.Sprintf(
		`SELECT MIN(%s.%s), MAX(%s.%s) FROM %s %s WHERE %s.%s IS NOT NULL%s`,
		outputTableAlias,
		quoteIdent(req.KeyField),
		outputTableAlias,
		quoteIdent(req.KeyField),
		fullTableName(sourceTable),
		outputTableAlias,
		outputTableAlias,
		quoteIdent(req.KeyField),
		where,
	)
	var start sql.NullInt64
	var end sql.NullInt64
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&start, &end); err != nil {
		return outputKeyRange{}, false, fmt.Errorf("select output key range: %w", err)
	}
	if !start.Valid || !end.Valid {
		return outputKeyRange{}, false, nil
	}
	return outputKeyRange{Start: start.Int64, End: end.Int64}, true, nil
}

func (r *EntityAlignmentBusinessRepo) writeOutputRowsRange(
	ctx context.Context,
	req ExecuteRequest,
	sourceTable qualifiedName,
	outputTable qualifiedName,
	mappingTable qualifiedName,
	sourceColumns []ColumnMeta,
	fields []PreparedField,
	page outputPage,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin output transaction: %w", err)
	}
	defer tx.Rollback()
	insertColumns := make([]string, 0, len(sourceColumns))
	selectExprs := make([]string, 0, len(sourceColumns))
	updates := make([]string, 0, len(fields))
	fieldMap := make(map[string]PreparedField, len(fields))
	for _, field := range fields {
		fieldMap[field.Name] = field
		updates = append(
			updates,
			fmt.Sprintf("%s = EXCLUDED.%s", quoteIdent(field.Name), quoteIdent(field.Name)),
		)
	}
	joins := make([]string, 0, len(fields))
	for _, field := range fields {
		alias := mappingTableAlias + field.Name
		joins = append(joins, fmt.Sprintf(
			`LEFT JOIN %s %s ON %s.field_name = %s AND %s.target_hash = %s AND %s.raw_value = %s.%s::text`,
			fullTableName(mappingTable),
			alias,
			alias,
			sqlStringLiteral(field.Name),
			alias,
			sqlStringLiteral(field.TargetHash),
			alias,
			outputTableAlias,
			quoteIdent(field.Name),
		))
	}
	for _, column := range sourceColumns {
		insertColumns = append(insertColumns, quoteIdent(column.Name))
		field, ok := fieldMap[column.Name]
		if !ok {
			selectExprs = append(selectExprs, fmt.Sprintf(
				"%s.%s",
				outputTableAlias,
				quoteIdent(column.Name),
			))
			continue
		}
		alias := mappingTableAlias + field.Name
		selectExprs = append(selectExprs, fmt.Sprintf(`
			CASE
				WHEN %s.%s IS NULL THEN NULL
				WHEN btrim(%s.%s::text) = '' THEN NULL
				WHEN %s.raw_value IS NOT NULL THEN
					CASE %s.status
						WHEN '%s' THEN %s.aligned_value
						WHEN '%s' THEN NULL
						WHEN '%s' THEN %s.%s
						ELSE %s.%s
					END
				ELSE %s.%s
			END AS %s`,
			outputTableAlias,
			quoteIdent(column.Name),
			outputTableAlias,
			quoteIdent(column.Name),
			alias,
			alias,
			MappingStatusMatched,
			alias,
			MappingStatusUnknownToNull,
			MappingStatusFallbackOriginal,
			outputTableAlias,
			quoteIdent(column.Name),
			outputTableAlias,
			quoteIdent(column.Name),
			outputTableAlias,
			quoteIdent(column.Name),
			quoteIdent(column.Name),
		))
	}
	where, args := buildRangeClause(
		outputTableAlias+"."+quoteIdent(req.KeyField),
		&page.Start,
		&page.End,
		1,
	)
	query := fmt.Sprintf(`
		INSERT INTO %s (%s)
		SELECT %s
		FROM %s %s
		%s
		WHERE TRUE%s
		ON CONFLICT (%s) DO UPDATE
		SET %s`,
		fullTableName(outputTable),
		strings.Join(insertColumns, ", "),
		strings.Join(selectExprs, ", "),
		fullTableName(sourceTable),
		outputTableAlias,
		strings.Join(joins, "\n"),
		where,
		quoteIdent(req.KeyField),
		strings.Join(updates, ", "),
	)
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("upsert output_table: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit output transaction: %w", err)
	}
	return nil
}

func buildOutputPages(start int64, end int64, pageSize int64) []outputPage {
	if start > end {
		return nil
	}
	if pageSize <= 0 {
		pageSize = defaultOutputPageSize
	}
	pages := make([]outputPage, 0, (end-start)/pageSize+1)
	for pageStart := start; pageStart <= end; {
		pageEnd := pageStart + pageSize - 1
		if end-pageStart < pageSize {
			pageEnd = end
		}
		pages = append(pages, outputPage{Start: pageStart, End: pageEnd})
		if pageEnd == end {
			break
		}
		pageStart = pageEnd + 1
	}
	return pages
}

func (r *EntityAlignmentBusinessRepo) loadColumns(
	ctx context.Context,
	name qualifiedName,
) ([]ColumnMeta, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.column_name, c.udt_name,
		       pg_catalog.format_type(a.atttypid, a.atttypmod) AS formatted_type
		FROM information_schema.columns c
		JOIN pg_catalog.pg_class cls
		  ON cls.relname = c.table_name
		JOIN pg_catalog.pg_namespace ns
		  ON ns.nspname = c.table_schema AND ns.oid = cls.relnamespace
		JOIN pg_catalog.pg_attribute a
		  ON a.attrelid = cls.oid AND a.attname = c.column_name
		WHERE c.table_schema = $1 AND c.table_name = $2
		ORDER BY c.ordinal_position`, name.Schema, name.Name)
	if err != nil {
		return nil, fmt.Errorf("load table columns: %w", err)
	}
	defer rows.Close()
	columns := make([]ColumnMeta, 0)
	for rows.Next() {
		var column ColumnMeta
		if err := rows.Scan(&column.Name, &column.UDTName, &column.FormattedType); err != nil {
			return nil, fmt.Errorf("scan table column: %w", err)
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate table columns: %w", err)
	}
	return columns, nil
}

func (r *EntityAlignmentBusinessRepo) ensureMappingTable(
	ctx context.Context,
	name qualifiedName,
) error {
	createSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id BIGSERIAL PRIMARY KEY,
			field_name TEXT NOT NULL,
			target_hash TEXT NOT NULL,
			targets_json JSONB NOT NULL,
			raw_value TEXT NOT NULL,
			aligned_value TEXT,
			status TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			last_used_at TIMESTAMPTZ NOT NULL,
			use_count BIGINT NOT NULL DEFAULT 1,
			CHECK (btrim(raw_value) <> ''),
			CHECK (status IN ('%s', '%s', '%s'))
		)`,
		fullTableName(name),
		MappingStatusMatched,
		MappingStatusUnknownToNull,
		MappingStatusFallbackOriginal,
	)
	if _, err := r.db.ExecContext(ctx, createSQL); err != nil {
		return fmt.Errorf("create mapping table: %w", err)
	}
	indexSQL := fmt.Sprintf(
		`CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s (field_name, target_hash, raw_value)`,
		quoteIdent(name.Name+"_uniq"),
		fullTableName(name),
	)
	if _, err := r.db.ExecContext(ctx, indexSQL); err != nil {
		return fmt.Errorf("create mapping index: %w", err)
	}
	return nil
}

func (r *EntityAlignmentBusinessRepo) ensureOutputTable(
	ctx context.Context,
	outputTable qualifiedName,
	sourceColumns []ColumnMeta,
) ([]ColumnMeta, error) {
	outputColumns, err := r.loadColumns(ctx, outputTable)
	if err != nil {
		return nil, err
	}
	if len(outputColumns) == 0 {
		definitions := make([]string, 0, len(sourceColumns))
		for _, column := range sourceColumns {
			definitions = append(definitions, fmt.Sprintf(
				"%s %s",
				quoteIdent(column.Name),
				column.FormattedType,
			))
		}
		createSQL := fmt.Sprintf(
			"CREATE TABLE %s (%s)",
			fullTableName(outputTable),
			strings.Join(definitions, ", "),
		)
		if _, err := r.db.ExecContext(ctx, createSQL); err != nil {
			return nil, fmt.Errorf("create output_table: %w", err)
		}
		return sourceColumns, nil
	}
	for _, sourceColumn := range sourceColumns {
		outputColumn, err := requireColumn(outputColumns, sourceColumn.Name)
		if err != nil {
			return nil, fmt.Errorf("output_table: %w", err)
		}
		if outputColumn.FormattedType != sourceColumn.FormattedType {
			return nil, fmt.Errorf(
				"output_table column %q type mismatch: expected %s, got %s",
				sourceColumn.Name,
				sourceColumn.FormattedType,
				outputColumn.FormattedType,
			)
		}
	}
	return outputColumns, nil
}

func (r *EntityAlignmentBusinessRepo) ensureOutputUniqueKey(
	ctx context.Context,
	outputTable qualifiedName,
	keyField string,
) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT pg_get_indexdef(i.indexrelid)
		FROM pg_index i
		JOIN pg_class t ON t.oid = i.indrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = $1
		  AND t.relname = $2
		  AND i.indisunique`, outputTable.Schema, outputTable.Name)
	if err != nil {
		return fmt.Errorf("load output_table indexes: %w", err)
	}
	defer rows.Close()
	expected := "(" + quoteIdent(keyField) + ")"
	for rows.Next() {
		var indexDef string
		if err := rows.Scan(&indexDef); err != nil {
			return fmt.Errorf("scan output_table index: %w", err)
		}
		if strings.Contains(indexDef, expected) {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate output_table indexes: %w", err)
	}
	indexName := quoteIdent(outputTable.Name + "_" + keyField + "_uniq")
	createSQL := fmt.Sprintf(
		"CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s (%s)",
		indexName,
		fullTableName(outputTable),
		quoteIdent(keyField),
	)
	if _, err := r.db.ExecContext(ctx, createSQL); err != nil {
		return fmt.Errorf("create output_table unique index: %w", err)
	}
	return nil
}

func (r *EntityAlignmentBusinessRepo) ensureSourceKeyNotNull(
	ctx context.Context,
	sourceTable qualifiedName,
	keyField string,
	startID *int64,
	endID *int64,
) error {
	where, args := buildRangeClause(
		outputTableAlias+"."+quoteIdent(keyField),
		startID,
		endID,
		1,
	)
	query := fmt.Sprintf(
		"SELECT COUNT(1) FROM %s %s WHERE %s.%s IS NULL%s",
		fullTableName(sourceTable),
		outputTableAlias,
		outputTableAlias,
		quoteIdent(keyField),
		where,
	)
	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return fmt.Errorf("validate key_field nulls: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("source_table key_field contains null values in selected range")
	}
	return nil
}

// parseQualifiedName 解析 "schema.table" 或 "table" 格式的表名。
func parseQualifiedName(raw string) (qualifiedName, error) {
	parts := strings.Split(strings.TrimSpace(raw), ".")
	if len(parts) == 1 {
		if !identPattern.MatchString(parts[0]) {
			return qualifiedName{}, fmt.Errorf("invalid identifier %q", raw)
		}
		return qualifiedName{Schema: defaultSchema, Name: parts[0]}, nil
	}
	if len(parts) == 2 {
		if !identPattern.MatchString(parts[0]) ||
			!identPattern.MatchString(parts[1]) {
			return qualifiedName{}, fmt.Errorf("invalid identifier %q", raw)
		}
		return qualifiedName{Schema: parts[0], Name: parts[1]}, nil
	}
	return qualifiedName{}, fmt.Errorf(
		"table name %q must be schema.table or table",
		raw,
	)
}

// requireColumn 按列名查找列元信息，不存在时返回错误。
func requireColumn(columns []ColumnMeta, name string) (ColumnMeta, error) {
	for _, column := range columns {
		if column.Name == name {
			return column, nil
		}
	}
	return ColumnMeta{}, fmt.Errorf("column %q does not exist", name)
}

// isIntegerColumn 判断列类型是否为 smallint/integer/bigint。
func isIntegerColumn(column ColumnMeta) bool {
	return column.UDTName == "int2" ||
		column.UDTName == "int4" ||
		column.UDTName == "int8"
}

// isTextCompatibleColumn 判断列类型是否为 text/varchar/char。
func isTextCompatibleColumn(column ColumnMeta) bool {
	return column.UDTName == "text" ||
		column.UDTName == "varchar" ||
		column.UDTName == "bpchar"
}

// buildRangeClause 构造 start_id/end_id 的 WHERE 范围子句及参数，startIndex 指定参数编号起点。
func buildRangeClause(
	columnExpr string,
	startID *int64,
	endID *int64,
	startIndex int,
) (string, []any) {
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 2)
	index := startIndex
	if startID != nil {
		clauses = append(clauses, fmt.Sprintf("%s >= $%d", columnExpr, index))
		args = append(args, *startID)
		index++
	}
	if endID != nil {
		clauses = append(clauses, fmt.Sprintf("%s <= $%d", columnExpr, index))
		args = append(args, *endID)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " AND " + strings.Join(clauses, " AND "), args
}

// mappingTableNameFor 从源表名推导 mapping 表的 schema 和固定名称。
func mappingTableNameFor(sourceTable string) (qualifiedName, error) {
	sourceName, err := parseQualifiedName(sourceTable)
	if err != nil {
		return qualifiedName{}, fmt.Errorf("parse source_table: %w", err)
	}
	return qualifiedName{Schema: sourceName.Schema, Name: mappingTableName}, nil
}

// fullTableName 将 schema 和表名拼接为 "schema"."table" 的完整引用。
func fullTableName(name qualifiedName) string {
	return quoteIdent(name.Schema) + "." + quoteIdent(name.Name)
}

// quoteIdent 对 Postgres 标识符加双引号，防 SQL 注入。
func quoteIdent(ident string) string {
	return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
}

// sqlStringLiteral 对字符串值加单引号并转义其中的单引号。
func sqlStringLiteral(value string) string {
	return `'` + strings.ReplaceAll(value, `'`, `''`) + `'`
}

// rowPlaceholders 构造 Postgres VALUES 行占位符，例如 ($1,$2,$3)。
func rowPlaceholders(start, count int) string {
	parts := make([]string, 0, count)
	for i := 0; i < count; i++ {
		parts = append(parts, fmt.Sprintf("$%d", start+i))
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// dollarList 构造 Postgres IN 子句占位符序列，例如 $1,$2,$3。
func dollarList(start, count int) string {
	parts := make([]string, 0, count)
	for i := 0; i < count; i++ {
		parts = append(parts, fmt.Sprintf("$%d", start+i))
	}
	return strings.Join(parts, ", ")
}

// nullableString 将数据库可空字符串转为 *string，未设置时返回 nil。
func nullableString(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

var _ BusinessRepository = (*EntityAlignmentBusinessRepo)(nil)
