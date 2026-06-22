package alignment

import (
	"context"
	"strings"
	"testing"

	"kgbrain/internal/processrecord"
)

type recallFlowRepo struct {
	recalledRawValue string
	recalledTopK     int
	upserted         []MappingRecord
}

func (r *recallFlowRepo) EnsureExecutionReady(
	_ context.Context,
	_ ExecuteRequest,
	_ []PreparedField,
) ([]ColumnMeta, error) {
	return []ColumnMeta{{Name: "id", UDTName: "int8", FormattedType: "bigint"}}, nil
}

func (r *recallFlowRepo) SelectDistinctRawValues(
	_ context.Context,
	_ ExecuteRequest,
	_ string,
) ([]string, error) {
	return []string{"西汉"}, nil
}

func (r *recallFlowRepo) LoadExistingMappings(
	_ context.Context,
	_ ExecuteRequest,
	_ PreparedField,
	_ []string,
) (map[string]MappingRecord, error) {
	return map[string]MappingRecord{}, nil
}

func (r *recallFlowRepo) TouchMappings(
	_ context.Context,
	_ ExecuteRequest,
	_ PreparedField,
	_ map[string]MappingRecord,
) error {
	return nil
}

func (r *recallFlowRepo) UpsertMappings(
	_ context.Context,
	_ ExecuteRequest,
	_ PreparedField,
	records []MappingRecord,
) error {
	r.upserted = append(r.upserted, records...)
	return nil
}

func (r *recallFlowRepo) LoadTargetLabels(
	_ context.Context,
	targetSetID string,
) ([]TargetDefinition, error) {
	return []TargetDefinition{
		{TargetSetID: targetSetID, Label: "唐"},
		{TargetSetID: targetSetID, Label: "汉"},
	}, nil
}

func (r *recallFlowRepo) RecallTopKTargets(
	_ context.Context,
	_ string,
	rawValue string,
	topK int,
) ([]string, error) {
	r.recalledRawValue = rawValue
	r.recalledTopK = topK
	return []string{"汉"}, nil
}

func (r *recallFlowRepo) UpsertTargets(
	_ context.Context,
	_ string,
	_ []TargetDefinition,
) error {
	return nil
}

func (r *recallFlowRepo) DeleteTarget(_ context.Context, _ string, _ string) error {
	return nil
}

func (r *recallFlowRepo) UpsertTargetCandidates(
	_ context.Context,
	_ string,
	_ []MappingRecord,
) error {
	return nil
}

func (r *recallFlowRepo) ListTargetCandidates(
	_ context.Context,
	_ string,
	_ string,
) ([]TargetCandidate, error) {
	return nil, nil
}

func (r *recallFlowRepo) ReviewTargetCandidates(
	_ context.Context,
	_ string,
	_ string,
	_ []ReviewCandidateAction,
) error {
	return nil
}

func (r *recallFlowRepo) BuildSourceRangeProcessRecords(
	_ context.Context,
	_ ExecuteRequest,
	_ []PreparedField,
) ([]processrecord.Record, error) {
	return nil, nil
}

func (r *recallFlowRepo) WriteOutputRows(
	_ context.Context,
	_ ExecuteRequest,
	_ []ColumnMeta,
	_ []PreparedField,
) error {
	return nil
}

type recallFlowLLM struct {
	t *testing.T
}

func (l recallFlowLLM) Generate(_ context.Context, prompt string) (string, error) {
	if !strings.Contains(prompt, `候选目标实体: ["汉"]`) {
		l.t.Fatalf("prompt did not use recalled targets: %s", prompt)
	}
	if strings.Contains(prompt, `候选目标实体: ["唐"`) ||
		strings.Contains(prompt, `候选目标实体: ["汉","唐"]`) {
		l.t.Fatalf("prompt unexpectedly contains full target set: %s", prompt)
	}
	return `{"raw_value":"西汉","status":"matched","aligned_value":"汉"}`, nil
}

type noopProcessRecorder struct{}

func (noopProcessRecorder) UpsertMany(_ context.Context, _ []processrecord.Record) error {
	return nil
}

func (noopProcessRecorder) UpsertSourceRange(
	_ context.Context,
	_ processrecord.SourceRangeRecord,
) error {
	return nil
}

func TestDomainServiceUsesRecalledTargetsForLLM(t *testing.T) {
	repo := &recallFlowRepo{}
	svc := NewDomainService(repo, DefaultBatchSize, noopProcessRecorder{})
	err := svc.Execute(context.Background(), recallFlowLLM{t: t}, ExecuteRequest{
		SourceTable: "public.source",
		OutputTable: "public.output",
		KeyField:    "id",
		FuzzyTopK:   7,
		Fields: []FieldConfig{{
			Name:        "dynasty",
			TargetSetID: "dynasty",
		}},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if repo.recalledRawValue != "西汉" || repo.recalledTopK != 7 {
		t.Fatalf("unexpected recall args: raw=%q topK=%d", repo.recalledRawValue, repo.recalledTopK)
	}
	if len(repo.upserted) != 1 || repo.upserted[0].AlignedValue == nil ||
		*repo.upserted[0].AlignedValue != "汉" {
		t.Fatalf("unexpected upserted mappings: %#v", repo.upserted)
	}
}

var _ BusinessRepository = (*recallFlowRepo)(nil)
