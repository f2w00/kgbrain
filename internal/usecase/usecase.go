package usecase

import (
	"context"
	"encoding/json"

	"kgbrain/internal/domain/kgc"
	"kgbrain/internal/domain/mapping"
	"kgbrain/internal/domain/profile"
)

type UseCase struct {
	profileRepo     profile.ProfileRepository
	mappingSvc      *mapping.MappingService
	mappingLLMFactory mapping.LLMClientFactory
	enrichSvc       *kgc.EnrichService
	enrichLLMFactory kgc.LLMClientFactory
}

func New(
	profileRepo profile.ProfileRepository,
	mappingSvc *mapping.MappingService,
	mappingLLMFactory mapping.LLMClientFactory,
	enrichSvc *kgc.EnrichService,
	enrichLLMFactory kgc.LLMClientFactory,
) *UseCase {
	return &UseCase{
		profileRepo:       profileRepo,
		mappingSvc:        mappingSvc,
		mappingLLMFactory: mappingLLMFactory,
		enrichSvc:         enrichSvc,
		enrichLLMFactory:  enrichLLMFactory,
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
	return u.profileRepo.Delete(id)
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

	llmClient, err := u.mappingLLMFactory(llmCfg.BaseURL, llmCfg.APIKey, llmCfg.Model)
	if err != nil {
		return nil, err
	}

	return u.mappingSvc.Execute(ctx, llmClient, &mapping.ExecuteRequest{
		Example:      example,
		TargetFields: targetFields,
		Refresh:      refresh,
	})
}

// Enrich 执行数据补全
// 流程: 获取 profile → 解析 LLM 配置 → 创建 LLM 客户端 → 调用领域层执行补全
func (u *UseCase) Enrich(ctx context.Context, profileID string, req *kgc.Request) (*kgc.Result, error) {
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

	llmClient, err := u.enrichLLMFactory(llmCfg.BaseURL, llmCfg.APIKey, llmCfg.Model)
	if err != nil {
		return nil, err
	}

	return u.enrichSvc.Execute(ctx, llmClient, req)
}

var errProfileNotFound = &profileNotFoundError{id: ""}

type profileNotFoundError struct {
	id string
}

func (e *profileNotFoundError) Error() string {
	return "profile not found"
}
