package mapping

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"kgbrain/internal/logger"
	"kgbrain/pkg/extract"
	"kgbrain/pkg/hash"

	"go.uber.org/zap"
)

type LLMClient interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

type LLMClientFactory func(baseURL, apiKey, model string) (LLMClient, error)

type ExecuteRequest struct {
	ProfileID    string
	Example      map[string]any
	TargetFields []string
	Refresh      bool
}

type Result struct {
	Mapping        Mapping
	UnmappedSource []string
	UnfilledTarget []string
	Cached         bool
}

type MappingService struct {
	cacheRepo CacheRepository
}

func NewService(cacheRepo CacheRepository) *MappingService {
	return &MappingService{cacheRepo: cacheRepo}
}

func (s *MappingService) Execute(ctx context.Context, llm LLMClient, req *ExecuteRequest) (*Result, error) {
	source := make([]string, 0, len(req.Example))
	for k := range req.Example {
		source = append(source, k)
	}

	h := cacheKey(source, req.TargetFields)

	if !req.Refresh {
		cached, err := s.cacheRepo.Get(req.ProfileID, h)
		if err != nil {
			return nil, fmt.Errorf("get cache: %w", err)
		}
		if cached != nil {
			return &Result{
				Mapping:        *cached,
				UnmappedSource: CalcUnmappedSource(*cached, source),
				UnfilledTarget: CalcUnfilledTarget(*cached, req.TargetFields),
				Cached:         true,
			}, nil
		}
	}

	prompt := BuildMappingPrompt(source, req.TargetFields, req.Example)
	resp, err := llm.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	content := extract.JSON(resp)
	var mapping Mapping
	if err := json.Unmarshal([]byte(content), &mapping); err != nil {
		return nil, fmt.Errorf("parse llm output: %w", err)
	}
	if err := mapping.Validate(source, req.TargetFields); err != nil {
		return nil, fmt.Errorf("mapping validation: %w", err)
	}

	if err := s.cacheRepo.Save(req.ProfileID, h, &mapping, source, req.TargetFields); err != nil {
		logger.L().Warn("save mapping cache failed", zap.Error(err))
	}

	logger.L().Info("mapping generated", zap.Int("mappings", len(mapping)))
	return &Result{
		Mapping:        mapping,
		UnmappedSource: CalcUnmappedSource(mapping, source),
		UnfilledTarget: CalcUnfilledTarget(mapping, req.TargetFields),
		Cached:         false,
	}, nil
}

func cacheKey(source, target []string) string {
	sort.Strings(source)
	sort.Strings(target)
	return hash.Key(strings.Join(source, ","), strings.Join(target, ","))
}
