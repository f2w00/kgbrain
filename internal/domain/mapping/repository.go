package mapping

type CacheRepository interface {
	Get(cacheKey string) (*Mapping, error)
	Save(cacheKey string, m *Mapping, source, target []string) error
}

type ContentRepository interface {
	GetContentMapping(topic string) (ContentMapping, error)
	SaveContentMapping(topic string, m ContentMapping) error
}
