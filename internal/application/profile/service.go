package profile

import (
	"kgbrain/internal/domain/profile"
)

type Service struct {
	repo profile.ProfileRepository
}

func NewService(repo profile.ProfileRepository) *Service {
	return &Service{repo: repo}
}

type SetResult struct {
	Status    string `json:"status"`
	ProfileID string `json:"profile_id"`
}

type GetResult struct {
	ProfileID    string `json:"profile_id"`
	LLMConfig    string `json:"llm_config"`
	NotifyConfig string `json:"notify_config,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type DeleteResult struct {
	Status    string `json:"status"`
	ProfileID string `json:"profile_id"`
}

func (s *Service) Set(id string, llm, notify []byte) (*SetResult, error) {
	p, err := profile.NewProfile(id, string(llm), string(notify))
	if err != nil {
		return nil, err
	}
	if err := s.repo.Save(p); err != nil {
		return nil, err
	}
	return &SetResult{
		Status:    "ok",
		ProfileID: id,
	}, nil
}

func (s *Service) Get(id string) (*GetResult, error) {
	p, err := s.repo.Get(id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, &profileNotFoundError{id: id}
	}
	return &GetResult{
		ProfileID:    p.ID,
		LLMConfig:    p.LLMConfig,
		NotifyConfig: p.NotifyConfig,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}, nil
}

func (s *Service) Delete(id string) (*DeleteResult, error) {
	deleted, err := s.repo.Delete(id)
	if err != nil {
		return nil, err
	}
	status := "deleted"
	if !deleted {
		status = "not_found"
	}
	return &DeleteResult{
		Status:    status,
		ProfileID: id,
	}, nil
}

type profileNotFoundError struct {
	id string
}

func (e *profileNotFoundError) Error() string {
	return "profile not found"
}

func (e *profileNotFoundError) Is(target error) bool {
	_, ok := target.(*profileNotFoundError)
	return ok
}
