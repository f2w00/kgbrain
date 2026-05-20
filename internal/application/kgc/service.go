package kgc

import (
	"context"

	"kgbrain/internal/domain/kgc"
	"kgbrain/internal/domain/profile"
)

type Service struct {
	profileRepo  profile.ProfileRepository
	enrichSvc    *kgc.EnrichService
	llmFactory   kgc.LLMClientFactory
}

func NewService(
	profileRepo profile.ProfileRepository,
	enrichSvc *kgc.EnrichService,
	llmFactory kgc.LLMClientFactory,
) *Service {
	return &Service{
		profileRepo: profileRepo,
		enrichSvc:   enrichSvc,
		llmFactory:  llmFactory,
	}
}

func (s *Service) Enrich(ctx context.Context, profileID string, req *kgc.Request) (*kgc.Result, error) {
	prof, err := s.profileRepo.Get(profileID)
	if err != nil {
		return nil, err
	}
	if prof == nil {
		return nil, errProfileNotFound
	}

	llmCfg, err := prof.ParseLLMConfig()
	if err != nil {
		return nil, err
	}

	llmClient, err := s.llmFactory(llmCfg.BaseURL, llmCfg.APIKey, llmCfg.Model)
	if err != nil {
		return nil, err
	}

	return s.enrichSvc.Execute(ctx, llmClient, req)
}

type profileNotFoundError struct {
	id string
}

func (e *profileNotFoundError) Error() string {
	return "profile not found"
}

var errProfileNotFound = &profileNotFoundError{id: ""}
