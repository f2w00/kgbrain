package usecase

import (
	"context"
	"encoding/json"

	"kgbrain/internal/domain/mapping"
	"kgbrain/internal/domain/profile"
)

type UseCase struct {
	profileRepo profile.ProfileRepository
	cacheRepo   mapping.CacheRepository
	mappingSvc  *mapping.MappingService
	llmFactory  mapping.LLMClientFactory
}

func New(
	profileRepo profile.ProfileRepository,
	cacheRepo mapping.CacheRepository,
	mappingSvc *mapping.MappingService,
	llmFactory mapping.LLMClientFactory,
) *UseCase {
	return &UseCase{
		profileRepo: profileRepo,
		cacheRepo:   cacheRepo,
		mappingSvc:  mappingSvc,
		llmFactory:  llmFactory,
	}
}

func (u *UseCase) SetProfile(id string, llm, notify json.RawMessage) error {
	p, err := profile.NewProfile(id, string(llm), string(notify))
	if err != nil {
		return err
	}
	return u.profileRepo.Save(p)
}

func (u *UseCase) GetProfile(id string) (*profile.Profile, error) {
	p, err := u.profileRepo.Get(id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errProfileNotFound
	}
	return p, nil
}

func (u *UseCase) DeleteProfile(id string) (deleted bool, err error) {
	deleted, err = u.profileRepo.Delete(id)
	if err != nil {
		return false, err
	}
	if err := u.cacheRepo.ClearByProfile(id); err != nil {
		return deleted, err
	}
	return deleted, nil
}

func (u *UseCase) GenerateMapping(
	ctx context.Context,
	profileID string,
	example map[string]any,
	targetFields []string,
	refresh bool,
) (*mapping.Result, error) {
	prof, err := u.profileRepo.Get(profileID)
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

	llmClient, err := u.llmFactory(llmCfg.BaseURL, llmCfg.APIKey, llmCfg.Model)
	if err != nil {
		return nil, err
	}

	return u.mappingSvc.Execute(ctx, llmClient, &mapping.ExecuteRequest{
		ProfileID:    prof.ID,
		Example:      example,
		TargetFields: targetFields,
		Refresh:      refresh,
	})
}

var errProfileNotFound = &profileNotFoundError{id: ""}

type profileNotFoundError struct {
	id string
}

func (e *profileNotFoundError) Error() string {
	return "profile not found"
}
