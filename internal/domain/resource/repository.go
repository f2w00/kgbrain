package resource

type Repository interface {
	SaveLLM(*LLMResource) error
	GetLLM(id string) (*LLMResource, error)
	DeleteLLM(id string) (bool, error)

	SaveDatabase(*DatabaseResource) error
	GetDatabase(id string) (*DatabaseResource, error)
	DeleteDatabase(id string) (bool, error)
}
