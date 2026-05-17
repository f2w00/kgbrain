package mapping

type CacheRepository interface {
	Get(profileID, cacheKey string) (*Mapping, error)
	Save(profileID, cacheKey string, m *Mapping, source, target []string) error
	ClearByProfile(profileID string) error
}
