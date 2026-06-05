package xform

import (
	"encoding/json"
	"fmt"
)

type TaskStatus string

const (
	StatusOpen      TaskStatus = "open"
	StatusProcessing TaskStatus = "processing"
	StatusClosing   TaskStatus = "closing"
	StatusCompleted TaskStatus = "completed"
	StatusFailed    TaskStatus = "failed"
	StatusPartial   TaskStatus = "partial"
	StatusDeleted   TaskStatus = "deleted"
)

type ErrorEntry struct {
	PKValue any    `json:"-"`
	PKField string `json:"-"`
	Error   string `json:"_error"`
}

func (e *ErrorEntry) MarshalJSON() ([]byte, error) {
	type Alias ErrorEntry
	aux := struct {
		PKValue any    `json:"pk_value"`
		PKField string `json:"pk_field"`
		*Alias
	}{
		PKValue: e.PKValue,
		PKField: e.PKField,
		Alias:   (*Alias)(e),
	}
	return json.Marshal(aux)
}

func FormatErrorEntry(rawData map[string]any, pkField string, errMsg string) ErrorEntry {
	return ErrorEntry{
		PKValue: rawData[pkField],
		PKField: pkField,
		Error:   errMsg,
	}
}

func InjectPrimaryKey(result map[string]any, rawData map[string]any, pkField string) {
	if v, ok := rawData[pkField]; ok {
		result[pkField] = v
	}
}

// StripKey 返回一个新 map, 不含指定的 key
func StripKey(m map[string]any, key string) map[string]any {
	r := make(map[string]any, len(m))
	for k, v := range m {
		if k != key {
			r[k] = v
		}
	}
	return r
}

func ExtractFieldKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	simpleSort(keys)
	return keys
}

func simpleSort(s []string) {
	for i := range s {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

func ValidateAppendData(data []map[string]any, pkField string) error {
	if len(data) == 0 {
		return fmt.Errorf("data is required")
	}
	for i, row := range data {
		if _, ok := row[pkField]; !ok {
			return fmt.Errorf("data[%d] missing primary_key field %q", i, pkField)
		}
	}
	return nil
}
