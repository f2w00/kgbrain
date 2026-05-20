package mapping

type ContentMapping map[string]string

func (m ContentMapping) Apply(values []string) map[string]string {
	result := make(map[string]string, len(values))
	for _, v := range values {
		if target, ok := m[v]; ok {
			result[v] = target
		} else {
			result[v] = v
		}
	}
	return result
}

func (m ContentMapping) Merge(other ContentMapping) ContentMapping {
	merged := make(ContentMapping, len(m)+len(other))
	for k, v := range m {
		merged[k] = v
	}
	for k, v := range other {
		merged[k] = v
	}
	return merged
}
