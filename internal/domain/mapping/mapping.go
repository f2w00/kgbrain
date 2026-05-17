package mapping

import "fmt"

type Mapping map[string][]string

func (m Mapping) Validate(source, target []string) error {
	srcSet := toSet(source)
	tgtSet := toSet(target)
	used := make(map[string]bool)

	for tgt, srcs := range m {
		if !tgtSet[tgt] {
			return fmt.Errorf("target %q not in target_fields", tgt)
		}
		if len(srcs) == 0 {
			return fmt.Errorf("target %q has empty source list", tgt)
		}
		for _, src := range srcs {
			if !srcSet[src] {
				return fmt.Errorf("source %q in target %q not in source_fields", src, tgt)
			}
			if used[src] {
				return fmt.Errorf("source %q mapped to multiple targets", src)
			}
			used[src] = true
		}
	}
	return nil
}

func CalcUnmappedSource(m Mapping, source []string) []string {
	mapped := make(map[string]bool)
	for _, srcs := range m {
		for _, s := range srcs {
			mapped[s] = true
		}
	}
	var out []string
	for _, s := range source {
		if !mapped[s] {
			out = append(out, s)
		}
	}
	return out
}

func CalcUnfilledTarget(m Mapping, target []string) []string {
	used := make(map[string]bool)
	for tgt := range m {
		used[tgt] = true
	}
	var out []string
	for _, t := range target {
		if !used[t] {
			out = append(out, t)
		}
	}
	return out
}

func toSet(s []string) map[string]bool {
	m := make(map[string]bool, len(s))
	for _, v := range s {
		m[v] = true
	}
	return m
}
