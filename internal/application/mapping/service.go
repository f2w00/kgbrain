package mapping

import (
	"context"

	"kgbrain/internal/domain/mapping"
	"kgbrain/internal/domain/profile"
)

type Service struct {
	profileRepo   profile.ProfileRepository
	domainSvc     *mapping.MappingService
	contentSvc    *mapping.ContentMappingService
	llmFactory    mapping.LLMClientFactory
}

func NewService(
	profileRepo profile.ProfileRepository,
	domainSvc *mapping.MappingService,
	contentSvc *mapping.ContentMappingService,
	llmFactory mapping.LLMClientFactory,
) *Service {
	return &Service{
		profileRepo: profileRepo,
		domainSvc:   domainSvc,
		contentSvc:  contentSvc,
		llmFactory:  llmFactory,
	}
}

type FieldResult = mapping.Result
type ContentResult = mapping.ContentResult
type TargetsResult = mapping.TargetsResult

func (s *Service) GenerateField(
	ctx context.Context,
	profileID string,
	example map[string]any,
	targetFields []string,
	refresh bool,
) (*FieldResult, error) {
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

	return s.domainSvc.Execute(ctx, llmClient, &mapping.ExecuteRequest{
		Example:      example,
		TargetFields: targetFields,
		Refresh:      refresh,
	})
}

func (s *Service) ApplyContent(
	ctx context.Context,
	profileID string,
	topic string,
	values []string,
	targets []string,
) (*ContentResult, error) {
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

	return s.contentSvc.Execute(ctx, llmClient, &mapping.ContentExecuteRequest{
		Topic:   topic,
		Values:  values,
		Targets: targets,
	})
}

func (s *Service) SetContent(
	ctx context.Context,
	profileID string,
	topic string,
	m mapping.ContentMapping,
) error {
	prof, err := s.profileRepo.Get(profileID)
	if err != nil {
		return err
	}
	if prof == nil {
		return errProfileNotFound
	}

	return s.contentSvc.Set(&mapping.ContentSetRequest{
		Topic:   topic,
		Mapping: m,
	})
}

func (s *Service) SetContentTargets(
	ctx context.Context,
	profileID string,
	topic string,
	targets []string,
) error {
	prof, err := s.profileRepo.Get(profileID)
	if err != nil {
		return err
	}
	if prof == nil {
		return errProfileNotFound
	}

	return s.contentSvc.SetTargets(&mapping.TargetsSetRequest{
		Topic:   topic,
		Targets: targets,
	})
}

func (s *Service) GetContentTargets(
	ctx context.Context,
	profileID string,
	topic string,
) (*TargetsResult, error) {
	prof, err := s.profileRepo.Get(profileID)
	if err != nil {
		return nil, err
	}
	if prof == nil {
		return nil, errProfileNotFound
	}

	return s.contentSvc.GetTargets(topic)
}

var errProfileNotFound = &profileNotFoundError{id: ""}

type profileNotFoundError struct {
	id string
}

func (e *profileNotFoundError) Error() string {
	return "profile not found"
}
