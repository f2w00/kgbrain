package alignment

import "testing"

func TestNormalizeTargets(t *testing.T) {
	got := NormalizeTargets([]string{" 宋 ", "唐", "", "宋", "元"})
	want := []string{"元", "唐", "宋"}
	if len(got) != len(want) {
		t.Fatalf("unexpected targets length: %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected targets: %#v", got)
		}
	}
}

func TestChunkStrings(t *testing.T) {
	got := ChunkStrings([]string{"a", "b", "c"}, 2)
	if len(got) != 2 {
		t.Fatalf("unexpected chunks: %#v", got)
	}
	if len(got[0]) != 2 || got[0][0] != "a" || got[0][1] != "b" {
		t.Fatalf("unexpected first chunk: %#v", got)
	}
	if len(got[1]) != 1 || got[1][0] != "c" {
		t.Fatalf("unexpected second chunk: %#v", got)
	}
}

func TestPrepareFieldUsesExplicitTargetSetID(t *testing.T) {
	batchSize := 100
	field, err := PrepareField(
		FieldConfig{Name: "era_name", TargetSetID: "dynasty", BatchSize: &batchSize},
		[]string{" 唐 ", "宋", "唐"},
		DefaultBatchSize,
	)
	if err != nil {
		t.Fatalf("prepare field: %v", err)
	}
	if field.Name != "era_name" || field.TargetSetID != "dynasty" {
		t.Fatalf("unexpected field identity: %#v", field)
	}
	if field.BatchSize != MaxBatchSize {
		t.Fatalf("expected batch size capped to %d, got %d", MaxBatchSize, field.BatchSize)
	}
	if len(field.Targets) != 2 || field.Targets[0] != "唐" || field.Targets[1] != "宋" {
		t.Fatalf("unexpected normalized targets: %#v", field.Targets)
	}
}

func TestPrepareFieldDefaultsTargetSetIDToName(t *testing.T) {
	field, err := PrepareField(
		FieldConfig{Name: "dynasty"},
		[]string{"唐"},
		DefaultBatchSize,
	)
	if err != nil {
		t.Fatalf("prepare field: %v", err)
	}
	if field.TargetSetID != "dynasty" {
		t.Fatalf("unexpected target set id: %q", field.TargetSetID)
	}
}

func TestValidateGeneratedMappingsAcceptsNeedsCandidate(t *testing.T) {
	field := PreparedField{Name: "dynasty", Targets: []string{"唐", "宋"}}
	got, err := ValidateGeneratedMappings(field, []string{"辽"}, []llmMappingRecord{{
		RawValue: "辽",
		Status:   MappingStatusNeedsCandidate,
	}})
	if err != nil {
		t.Fatalf("validate mappings: %v", err)
	}
	if len(got) != 1 || got[0].Status != MappingStatusNeedsCandidate || got[0].AlignedValue != nil {
		t.Fatalf("unexpected mapping: %#v", got)
	}
}

func TestValidateGeneratedMappingsRejectsFallbackOriginal(t *testing.T) {
	field := PreparedField{Name: "dynasty", Targets: []string{"唐", "宋"}}
	aligned := "辽"
	_, err := ValidateGeneratedMappings(field, []string{"辽"}, []llmMappingRecord{{
		RawValue:     "辽",
		Status:       "fallback_original",
		AlignedValue: &aligned,
	}})
	if err == nil {
		t.Fatal("expected fallback_original to be rejected")
	}
}
