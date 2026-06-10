package resource

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

type SetResult struct {
	ResourceID string
	Status     string
}

type DeleteResult struct {
	ResourceID string
	Status     string
}

func (s *Service) SetLLM(r *LLMResource) (*SetResult, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.SaveLLM(r); err != nil {
		return nil, err
	}
	return &SetResult{ResourceID: r.ID, Status: "ok"}, nil
}

func (s *Service) GetLLM(id string) (*LLMResource, error) {
	r, err := s.repo.GetLLM(id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, &notFoundError{id: id}
	}
	return r, nil
}

func (s *Service) DeleteLLM(id string) (*DeleteResult, error) {
	deleted, err := s.repo.DeleteLLM(id)
	if err != nil {
		return nil, err
	}
	status := "deleted"
	if !deleted {
		status = "not_found"
	}
	return &DeleteResult{ResourceID: id, Status: status}, nil
}

func (s *Service) SetDatabase(r *DatabaseResource) (*SetResult, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.SaveDatabase(r); err != nil {
		return nil, err
	}
	return &SetResult{ResourceID: r.ID, Status: "ok"}, nil
}

func (s *Service) GetDatabase(id string) (*DatabaseResource, error) {
	r, err := s.repo.GetDatabase(id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, &notFoundError{id: id}
	}
	return r, nil
}

func (s *Service) DeleteDatabase(id string) (*DeleteResult, error) {
	deleted, err := s.repo.DeleteDatabase(id)
	if err != nil {
		return nil, err
	}
	status := "deleted"
	if !deleted {
		status = "not_found"
	}
	return &DeleteResult{ResourceID: id, Status: status}, nil
}
