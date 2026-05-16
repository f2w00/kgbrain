package mapping_test

import (
	"testing"

	"kgbrain/internal/operation/mapping"
)

func TestMappingValidate(t *testing.T) {
	m := mapping.Mapping{"product_name": {"name", "title"}}
	err := m.Validate([]string{"name", "title"}, []string{"product_name"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestMappingValidateBadTarget(t *testing.T) {
	m := mapping.Mapping{"not_there": {"name"}}
	err := m.Validate([]string{"name"}, []string{"other"})
	if err == nil {
		t.Error("expected error for missing target")
	}
}

func TestMappingValidateBadSource(t *testing.T) {
	m := mapping.Mapping{"target": {"not_there"}}
	err := m.Validate([]string{"other"}, []string{"target"})
	if err == nil {
		t.Error("expected error for missing source")
	}
}

func TestMappingValidateSourceReused(t *testing.T) {
	m := mapping.Mapping{"a": {"x"}, "b": {"x"}}
	err := m.Validate([]string{"x"}, []string{"a", "b"})
	if err == nil {
		t.Error("expected error for source reused across targets")
	}
}

func TestMappingValidateEmptyList(t *testing.T) {
	m := mapping.Mapping{"target": {}}
	err := m.Validate([]string{}, []string{"target"})
	if err == nil {
		t.Error("expected error for empty source list")
	}
}

func TestCalcUnmappedSource(t *testing.T) {
	m := mapping.Mapping{"a": {"x", "y"}}
	u := mapping.CalcUnmappedSource(m, []string{"x", "y", "z"})
	if len(u) != 1 || u[0] != "z" {
		t.Errorf("unmapped = %v, want [z]", u)
	}
}

func TestCalcUnfilledTarget(t *testing.T) {
	m := mapping.Mapping{"a": {"x"}}
	u := mapping.CalcUnfilledTarget(m, []string{"a", "b", "c"})
	if len(u) != 2 || u[0] != "b" || u[1] != "c" {
		t.Errorf("unfilled = %v, want [b c]", u)
	}
}
