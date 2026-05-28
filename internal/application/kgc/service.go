package kgc

import (
	"context"
	"time"

	"kgbrain/internal/domain/kgc"
	"kgbrain/internal/domain/profile"
)

type Service struct {
	profileRepo  profile.ProfileRepository
	enrichSvc    *kgc.EnrichService
	autofillSvc  *kgc.AutofillService
	llmFactory   kgc.LLMClientFactory
}

func NewService(
	profileRepo profile.ProfileRepository,
	enrichSvc *kgc.EnrichService,
	autofillSvc *kgc.AutofillService,
	llmFactory kgc.LLMClientFactory,
) *Service {
	return &Service{
		profileRepo: profileRepo,
		enrichSvc:   enrichSvc,
		autofillSvc: autofillSvc,
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

	timeout := llmCfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = 180
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	llmClient, err := s.llmFactory(llmCfg.BaseURL, llmCfg.APIKey, llmCfg.Model)
	if err != nil {
		return nil, err
	}

	return s.enrichSvc.Execute(ctx, llmClient, req)
}

func (s *Service) Autofill(ctx context.Context, profileID string, req *kgc.AutofillRequest) (*kgc.AutofillResult, error) {
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

	timeout := llmCfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = 180
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	llmClient, err := s.llmFactory(llmCfg.BaseURL, llmCfg.APIKey, llmCfg.Model)
	if err != nil {
		return nil, err
	}

	return s.autofillSvc.Execute(ctx, llmClient, req)
}

type profileNotFoundError struct {
	id string
}

func (e *profileNotFoundError) Error() string {
	return "profile not found"
}

var errProfileNotFound = &profileNotFoundError{id: ""}
