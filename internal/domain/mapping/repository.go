package mapping

type CacheRepository interface {
	Get(cacheKey string) (*Mapping, error)
	Save(cacheKey string, m *Mapping, source, target []string) error
}
