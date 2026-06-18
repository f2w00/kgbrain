// business_repository.go 提供业务 Postgres 侧的实体对齐数据读写能力。
package alignment

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"kgbrain/internal/processrecord"
)

const (
	defaultSchema         = "public"
	mappingTableName      = "entity_alignment_mapping"
	targetTableName       = "alignment_targets"
	candidateTableName    = "target_candidates"
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
	mappingTable := qualifiedName{Schema: defaultSchema, Name: mappingTableName}

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
	if err := r.ensureTargetsTable(ctx, qualifiedName{Schema: defaultSchema, Name: targetTableName}); err != nil {
		return nil, err
	}
	if err := r.ensureCandidatesTable(ctx, qualifiedName{Schema: defaultSchema, Name: candidateTableName}); err != nil {
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

func (r *EntityAlignmentBusinessRepo) LoadTargetLabels(
	ctx context.Context,
	targetSetID string,
) ([]TargetDefinition, error) {
	table := qualifiedName{Schema: defaultSchema, Name: targetTableName}
	if err := r.ensureTargetsTable(ctx, table); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT target_set_id, label, description
		FROM %s
		WHERE target_set_id = $1
		ORDER BY label`, fullTableName(table)), targetSetID)
	if err != nil {
		return nil, fmt.Errorf("list alignment targets: %w", err)
	}
	defer rows.Close()
	targets := make([]TargetDefinition, 0)
	for rows.Next() {
		var target TargetDefinition
		var description sql.NullString
		if err := rows.Scan(&target.TargetSetID, &target.Label, &description); err != nil {
			return nil, fmt.Errorf("scan alignment target: %w", err)
		}
		target.Description = description.String
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alignment targets: %w", err)
	}
	return targets, nil
}

func (r *EntityAlignmentBusinessRepo) UpsertTargets(
	ctx context.Context,
	targetSetID string,
	targets []TargetDefinition,
) error {
	table := qualifiedName{Schema: defaultSchema, Name: targetTableName}
	if err := r.ensureTargetsTable(ctx, table); err != nil {
		return err
	}
	valueRows := make([]string, 0, len(targets))
	args := make([]any, 0, len(targets)*4)
	argIndex := 1
	now := time.Now().UTC()
	for _, target := range targets {
		valueRows = append(valueRows, rowPlaceholders(argIndex, 4))
		args = append(args, targetSetID, target.Label, nullableStringValue(target.Description), now)
		argIndex += 4
	}
	query := fmt.Sprintf(`
		INSERT INTO %s (target_set_id, label, description, updated_at)
		VALUES %s
		ON CONFLICT (target_set_id, label) DO UPDATE
		SET description = EXCLUDED.description,
		    updated_at = EXCLUDED.updated_at`, fullTableName(table), strings.Join(valueRows, ", "))
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("upsert alignment targets: %w", err)
	}
	return nil
}

func (r *EntityAlignmentBusinessRepo) DeleteTarget(
	ctx context.Context,
	targetSetID string,
	label string,
) error {
	table := qualifiedName{Schema: defaultSchema, Name: targetTableName}
	if err := r.ensureTargetsTable(ctx, table); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, fmt.Sprintf(
		`DELETE FROM %s WHERE target_set_id = $1 AND label = $2`,
		fullTableName(table),
	), targetSetID, label); err != nil {
		return fmt.Errorf("delete alignment target: %w", err)
	}
	return nil
}

func (r *EntityAlignmentBusinessRepo) UpsertTargetCandidates(
	ctx context.Context,
	targetSetID string,
	records []MappingRecord,
) error {
	table := qualifiedName{Schema: defaultSchema, Name: candidateTableName}
	if err := r.ensureCandidatesTable(ctx, table); err != nil {
		return err
	}
	pending := make([]MappingRecord, 0, len(records))
	for _, record := range records {
		if record.Status == "needs_candidate" {
			pending = append(pending, record)
		}
	}
	if len(pending) == 0 {
		return nil
	}
	valueRows := make([]string, 0, len(pending))
	args := make([]any, 0, len(pending)*5)
	argIndex := 1
	now := time.Now().UTC()
	for _, record := range pending {
		valueRows = append(valueRows, rowPlaceholders(argIndex, 5))
		args = append(args, generateCandidateID(), targetSetID, record.RawValue, now, now)
		argIndex += 5
	}
	query := fmt.Sprintf(`
			INSERT INTO %s (
				id, target_set_id, raw_value, created_at, updated_at
			) VALUES %s
			ON CONFLICT (target_set_id, raw_value) DO UPDATE
		SET frequency = %s.frequency + 1,
		    updated_at = EXCLUDED.updated_at`, fullTableName(table), strings.Join(valueRows, ", "), fullTableName(table))
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("upsert target candidates: %w", err)
	}
	return nil
}

func (r *EntityAlignmentBusinessRepo) ListTargetCandidates(
	ctx context.Context,
	targetSetID string,
	status string,
) ([]TargetCandidate, error) {
	table := qualifiedName{Schema: defaultSchema, Name: candidateTableName}
	if err := r.ensureCandidatesTable(ctx, table); err != nil {
		return nil, err
	}
	args := []any{targetSetID}
	where := "WHERE target_set_id = $1"
	if status != "" {
		args = append(args, status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, target_set_id, raw_value, frequency, status, resolution,
		       resolved_label, review_reason, created_at, updated_at
		FROM %s
		%s
		ORDER BY frequency DESC, created_at ASC`, fullTableName(table), where), args...)
	if err != nil {
		return nil, fmt.Errorf("list target candidates: %w", err)
	}
	defer rows.Close()
	candidates := make([]TargetCandidate, 0)
	for rows.Next() {
		var candidate TargetCandidate
		var resolution, resolvedLabel, reviewReason sql.NullString
		var createdAt, updatedAt time.Time
		if err := rows.Scan(
			&candidate.ID,
			&candidate.TargetSetID,
			&candidate.RawValue,
			&candidate.Frequency,
			&candidate.Status,
			&resolution,
			&resolvedLabel,
			&reviewReason,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan target candidate: %w", err)
		}
		candidate.Resolution = resolution.String
		candidate.ResolvedLabel = resolvedLabel.String
		candidate.ReviewReason = reviewReason.String
		candidate.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		candidate.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate target candidates: %w", err)
	}
	return candidates, nil
}

func (r *EntityAlignmentBusinessRepo) ReviewTargetCandidates(
	ctx context.Context,
	sourceTable string,
	targetSetID string,
	actions []ReviewCandidateAction,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin candidate review tx: %w", err)
	}
	defer tx.Rollback()
	targetTable := qualifiedName{Schema: defaultSchema, Name: targetTableName}
	candidateTable := qualifiedName{Schema: defaultSchema, Name: candidateTableName}
	mappingTable, err := mappingTableNameFor(sourceTable)
	if err != nil {
		return err
	}
	if err := r.ensureTargetsTable(ctx, targetTable); err != nil {
		return err
	}
	if err := r.ensureCandidatesTable(ctx, candidateTable); err != nil {
		return err
	}
	if err := r.ensureMappingTable(ctx, mappingTable); err != nil {
		return err
	}
	for _, action := range actions {
		var rawValue string
		if err := tx.QueryRowContext(ctx, fmt.Sprintf(
			`SELECT raw_value FROM %s WHERE id = $1 AND target_set_id = $2`,
			fullTableName(candidateTable),
		), action.CandidateID, targetSetID).Scan(&rawValue); err != nil {
			return fmt.Errorf("load candidate %s: %w", action.CandidateID, err)
		}
		switch action.Resolution {
		case "add_as_label":
			if _, err := tx.ExecContext(ctx, fmt.Sprintf(
				`INSERT INTO %s (target_set_id, label, description, created_at, updated_at)
				 VALUES ($1, $2, NULL, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
				 ON CONFLICT (target_set_id, label) DO UPDATE SET updated_at = CURRENT_TIMESTAMP`,
				fullTableName(targetTable),
			), targetSetID, action.Label); err != nil {
				return fmt.Errorf("upsert target during review: %w", err)
			}
			if err := r.reviewCandidateMapping(
				ctx,
				tx,
				mappingTable,
				targetSetID,
				rawValue,
				action.Label,
				MappingStatusMatched,
			); err != nil {
				return err
			}
		case "map_to_existing":
			var exists int
			if err := tx.QueryRowContext(ctx, fmt.Sprintf(
				`SELECT COUNT(1) FROM %s WHERE target_set_id = $1 AND label = $2`,
				fullTableName(targetTable),
			), targetSetID, action.Label).Scan(&exists); err != nil {
				return fmt.Errorf("check target exists: %w", err)
			}
			if exists == 0 {
				return fmt.Errorf("target label %q not found in target_set %q", action.Label, targetSetID)
			}
			if err := r.reviewCandidateMapping(
				ctx,
				tx,
				mappingTable,
				targetSetID,
				rawValue,
				action.Label,
				MappingStatusMatched,
			); err != nil {
				return err
			}
		case "reject_as_null":
			if err := r.reviewCandidateMapping(
				ctx,
				tx,
				mappingTable,
				targetSetID,
				rawValue,
				"",
				MappingStatusUnknownToNull,
			); err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid candidate resolution %q", action.Resolution)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(
			`UPDATE %s
			 SET status = 'resolved', resolution = $1, resolved_label = NULLIF($2, ''),
			     review_reason = NULLIF($3, ''), resolved_at = CURRENT_TIMESTAMP,
			     updated_at = CURRENT_TIMESTAMP
			 WHERE id = $4`, fullTableName(candidateTable)),
			action.Resolution, action.Label, action.ReviewReason, action.CandidateID); err != nil {
			return fmt.Errorf("update target candidate review result: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit candidate review tx: %w", err)
	}
	return nil
}

func (r *EntityAlignmentBusinessRepo) BuildSourceRangeProcessRecords(
	ctx context.Context,
	req ExecuteRequest,
	fields []PreparedField,
) ([]processrecord.Record, error) {
	sourceTable, err := parseQualifiedName(req.SourceTable)
	if err != nil {
		return nil, fmt.Errorf("parse source_table: %w", err)
	}
	selects := make([]string, 0, len(fields))
	joins := make([]string, 0, len(fields))
	for _, field := range fields {
		alias := mappingTableAlias + field.Name
		joins = append(joins, fmt.Sprintf(
			`LEFT JOIN %s %s ON %s.target_set_id = %s AND %s.raw_value = %s.%s::text`,
			fullTableName(qualifiedName{Schema: defaultSchema, Name: mappingTableName}),
			alias,
			alias,
			sqlStringLiteral(field.TargetSetID),
			alias,
			outputTableAlias,
			quoteIdent(field.Name),
		))
		selects = append(selects, fmt.Sprintf(
			`CASE
				WHEN %s.%s IS NULL THEN FALSE
				WHEN btrim(%s.%s::text) = '' THEN FALSE
				WHEN %s.status = '%s' THEN TRUE
				ELSE FALSE
			 END`,
			outputTableAlias,
			quoteIdent(field.Name),
			outputTableAlias,
			quoteIdent(field.Name),
			alias,
			"needs_candidate",
		))
	}
	where, args := buildRangeClause(
		outputTableAlias+"."+quoteIdent(req.KeyField),
		req.StartID,
		req.EndID,
		1,
	)
	reviewWhere, reviewArgs := buildWaitingTargetReviewClause(
		req,
		outputTableAlias+"."+quoteIdent(req.KeyField),
		len(args)+1,
	)
	args = append(args, reviewArgs...)
	query := fmt.Sprintf(`
			SELECT %s.%s, %s
			FROM %s %s
			%s
			WHERE %s.%s IS NOT NULL%s%s`,
		outputTableAlias,
		quoteIdent(req.KeyField),
		strings.Join(selects, ", "),
		fullTableName(sourceTable),
		outputTableAlias,
		strings.Join(joins, "\n"),
		outputTableAlias,
		quoteIdent(req.KeyField),
		where,
		reviewWhere,
	)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("build source range process records: %w", err)
	}
	defer rows.Close()
	records := make([]processrecord.Record, 0)
	for rows.Next() {
		values := make([]any, 0, 1+len(fields))
		var key int64
		values = append(values, &key)
		flags := make([]bool, len(fields))
		for i := range flags {
			values = append(values, &flags[i])
		}
		if err := rows.Scan(values...); err != nil {
			return nil, fmt.Errorf("scan process record row: %w", err)
		}
		status := processrecord.StatusSucceeded
		for _, flag := range flags {
			if flag {
				status = processrecord.StatusWaitingTargetReview
				break
			}
		}
		records = append(records, processrecord.Record{
			SourceTable: req.SourceTable,
			SourceKey:   key,
			ProcessType: ProcessTypeEntityAlignment,
			Status:      status,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate process record rows: %w", err)
	}
	return records, nil
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
	reviewWhere, reviewArgs := buildWaitingTargetReviewClause(
		req,
		outputTableAlias+"."+quoteIdent(req.KeyField),
		len(args)+1,
	)
	args = append(args, reviewArgs...)
	query := fmt.Sprintf(`
		SELECT DISTINCT %s.%s::text
		FROM %s %s
		WHERE %s.%s IS NOT NULL
		  AND btrim(%s.%s::text) <> ''%s%s`,
		outputTableAlias,
		quoteIdent(fieldName),
		fullTableName(sourceTable),
		outputTableAlias,
		outputTableAlias,
		quoteIdent(fieldName),
		outputTableAlias,
		quoteIdent(fieldName),
		where,
		reviewWhere,
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
	placeholders := dollarList(2, len(rawValues))
	args := make([]any, 0, len(rawValues)+1)
	args = append(args, field.TargetSetID)
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
		WHERE target_set_id = $1
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
	placeholders := dollarList(3, len(rawValues))
	now := time.Now().UTC()
	args := []any{now, field.TargetSetID}
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
		WHERE target_set_id = $2
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
) error {
	if len(records) == 0 {
		return nil
	}
	now := time.Now().UTC()
	valueRows := make([]string, 0, len(records))
	args := make([]any, 0, len(records)*7)
	argIndex := 1
	for _, record := range records {
		valueRows = append(valueRows, rowPlaceholders(argIndex, 7))
		args = append(
			args,
			field.TargetSetID,
			record.RawValue,
			record.AlignedValue,
			record.Status,
			now,
			now,
			now,
		)
		argIndex += 7
	}
	mappingTable, err := mappingTableNameFor(req.SourceTable)
	if err != nil {
		return err
	}
	conflictSet := `
		last_used_at = EXCLUDED.last_used_at,
		updated_at = EXCLUDED.updated_at,
		use_count = ` + fullTableName(mappingTable) + `.use_count + 1`
	query := fmt.Sprintf(`
		INSERT INTO %s (
			target_set_id,
			raw_value,
			aligned_value,
			status,
			created_at,
			updated_at,
			last_used_at
		)
		VALUES %s
		ON CONFLICT (target_set_id, raw_value) DO UPDATE
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
	mappingTable := qualifiedName{Schema: defaultSchema, Name: mappingTableName}
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
	reviewWhere, reviewArgs := buildWaitingTargetReviewClause(
		req,
		outputTableAlias+"."+quoteIdent(req.KeyField),
		len(args)+1,
	)
	args = append(args, reviewArgs...)
	query := fmt.Sprintf(
		`SELECT MIN(%s.%s), MAX(%s.%s) FROM %s %s WHERE %s.%s IS NOT NULL%s%s`,
		outputTableAlias,
		quoteIdent(req.KeyField),
		outputTableAlias,
		quoteIdent(req.KeyField),
		fullTableName(sourceTable),
		outputTableAlias,
		outputTableAlias,
		quoteIdent(req.KeyField),
		where,
		reviewWhere,
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
			`LEFT JOIN %s %s ON %s.target_set_id = %s AND %s.raw_value = %s.%s::text`,
			fullTableName(mappingTable),
			alias,
			alias,
			sqlStringLiteral(field.TargetSetID),
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
						WHEN '%s' THEN NULL
						ELSE NULL
					END
				ELSE NULL
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
			MappingStatusNeedsCandidate,
			quoteIdent(column.Name),
		))
	}
	where, args := buildRangeClause(
		outputTableAlias+"."+quoteIdent(req.KeyField),
		&page.Start,
		&page.End,
		1,
	)
	reviewWhere, reviewArgs := buildWaitingTargetReviewClause(
		req,
		outputTableAlias+"."+quoteIdent(req.KeyField),
		len(args)+1,
	)
	args = append(args, reviewArgs...)
	query := fmt.Sprintf(`
		INSERT INTO %s (%s)
		SELECT %s
		FROM %s %s
		%s
		WHERE TRUE%s%s
		ON CONFLICT (%s) DO UPDATE
		SET %s`,
		fullTableName(outputTable),
		strings.Join(insertColumns, ", "),
		strings.Join(selectExprs, ", "),
		fullTableName(sourceTable),
		outputTableAlias,
		strings.Join(joins, "\n"),
		where,
		reviewWhere,
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
			target_set_id TEXT NOT NULL,
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
		MappingStatusNeedsCandidate,
	)
	if _, err := r.db.ExecContext(ctx, createSQL); err != nil {
		return fmt.Errorf("create mapping table: %w", err)
	}
	if err := r.ensureMappingTargetSetColumn(ctx, name); err != nil {
		return err
	}
	indexSQL := fmt.Sprintf(
		`CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s (target_set_id, raw_value)`,
		quoteIdent(name.Name+"_target_set_raw_value_uniq"),
		fullTableName(name),
	)
	if _, err := r.db.ExecContext(ctx, indexSQL); err != nil {
		return fmt.Errorf("create mapping index: %w", err)
	}
	return nil
}

func (r *EntityAlignmentBusinessRepo) ensureTargetsTable(
	ctx context.Context,
	name qualifiedName,
) error {
	createSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			target_set_id TEXT NOT NULL,
			label TEXT NOT NULL,
			description TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (target_set_id, label)
		)`, fullTableName(name))
	if _, err := r.db.ExecContext(ctx, createSQL); err != nil {
		return fmt.Errorf("create targets table: %w", err)
	}
	return nil
}

func (r *EntityAlignmentBusinessRepo) ensureCandidatesTable(
	ctx context.Context,
	name qualifiedName,
) error {
	createSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			target_set_id TEXT NOT NULL,
			raw_value TEXT NOT NULL,
			frequency BIGINT NOT NULL DEFAULT 1,
			status TEXT NOT NULL DEFAULT 'pending',
			resolution TEXT,
			resolved_label TEXT,
			review_reason TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			resolved_at TIMESTAMPTZ,
			UNIQUE (target_set_id, raw_value)
		)`, fullTableName(name))
	if _, err := r.db.ExecContext(ctx, createSQL); err != nil {
		return fmt.Errorf("create candidates table: %w", err)
	}
	return nil
}

func (r *EntityAlignmentBusinessRepo) ensureMappingTargetSetColumn(
	ctx context.Context,
	name qualifiedName,
) error {
	if _, err := r.db.ExecContext(ctx, fmt.Sprintf(
		`ALTER TABLE %s ADD COLUMN IF NOT EXISTS target_set_id TEXT`,
		fullTableName(name),
	)); err != nil {
		return fmt.Errorf("add mapping target_set_id column: %w", err)
	}
	fieldNameExists, err := r.columnExists(ctx, name, "field_name")
	if err != nil {
		return err
	}
	if fieldNameExists {
		if _, err := r.db.ExecContext(ctx, fmt.Sprintf(
			`UPDATE %s SET target_set_id = field_name WHERE target_set_id IS NULL`,
			fullTableName(name),
		)); err != nil {
			return fmt.Errorf("backfill mapping target_set_id: %w", err)
		}
	}
	if _, err := r.db.ExecContext(ctx, fmt.Sprintf(
		`ALTER TABLE %s ALTER COLUMN target_set_id SET NOT NULL`,
		fullTableName(name),
	)); err != nil {
		return fmt.Errorf("set mapping target_set_id not null: %w", err)
	}
	for _, column := range []string{"field_name", "target_hash", "targets_json"} {
		if _, err := r.db.ExecContext(ctx, fmt.Sprintf(
			`ALTER TABLE %s DROP COLUMN IF EXISTS %s`,
			fullTableName(name),
			quoteIdent(column),
		)); err != nil {
			return fmt.Errorf("drop mapping %s column: %w", column, err)
		}
	}
	return nil
}

func (r *EntityAlignmentBusinessRepo) columnExists(
	ctx context.Context,
	name qualifiedName,
	column string,
) (bool, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2 AND column_name = $3`,
		name.Schema,
		name.Name,
		column,
	).Scan(&count); err != nil {
		return false, fmt.Errorf("check column exists: %w", err)
	}
	return count > 0, nil
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

// buildWaitingTargetReviewClause 只保留上次实体对齐等待 target 审核的源数据。
func buildWaitingTargetReviewClause(
	req ExecuteRequest,
	keyExpr string,
	startIndex int,
) (string, []any) {
	if !req.OnlyWaitingTargetReview {
		return "", nil
	}
	clause := fmt.Sprintf(` AND EXISTS (
		SELECT 1
		FROM data_process_records dpr
		WHERE dpr.source_table = $%d
		  AND dpr.source_key = %s
		  AND dpr.process_type = $%d
		  AND dpr.status = $%d
	)`, startIndex, keyExpr, startIndex+1, startIndex+2)
	return clause, []any{
		req.SourceTable,
		ProcessTypeEntityAlignment,
		processrecord.StatusWaitingTargetReview,
	}
}

// mappingTableNameFor 返回数据库资源内共享的 public mapping 表。
func mappingTableNameFor(sourceTable string) (qualifiedName, error) {
	if strings.TrimSpace(sourceTable) != "" {
		if _, err := parseQualifiedName(sourceTable); err != nil {
			return qualifiedName{}, fmt.Errorf("parse source_table: %w", err)
		}
	}
	return qualifiedName{Schema: defaultSchema, Name: mappingTableName}, nil
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

func nullableStringValue(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func generateCandidateID() string {
	return fmt.Sprintf("candidate_%d", time.Now().UnixNano())
}

func (r *EntityAlignmentBusinessRepo) reviewCandidateMapping(
	ctx context.Context,
	tx *sql.Tx,
	mappingTable qualifiedName,
	targetSetID string,
	rawValue string,
	alignedValue string,
	status string,
) error {
	var nullableAligned any
	if alignedValue != "" {
		nullableAligned = alignedValue
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(
		`INSERT INTO %s (
			target_set_id, raw_value, aligned_value, status,
			created_at, updated_at, last_used_at
		 ) VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		 ON CONFLICT (target_set_id, raw_value) DO UPDATE
		 SET aligned_value = EXCLUDED.aligned_value,
		     status = EXCLUDED.status,
			 updated_at = CURRENT_TIMESTAMP,
			 last_used_at = CURRENT_TIMESTAMP`,
		fullTableName(mappingTable),
	), targetSetID, rawValue, nullableAligned, status); err != nil {
		return fmt.Errorf("review candidate mapping upsert: %w", err)
	}
	return nil
}

var _ BusinessRepository = (*EntityAlignmentBusinessRepo)(nil)
