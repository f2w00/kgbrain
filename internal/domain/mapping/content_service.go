package mapping

import (
	"context"
	"encoding/json"
	"fmt"

	"kgbrain/internal/logger"
	"kgbrain/pkg/extract"

	"go.uber.org/zap"
)

type ContentExecuteRequest struct {
	Topic  string
	Values []string
}

type ContentSetRequest struct {
	Topic   string
	Mapping ContentMapping
}

type ContentResult struct {
	Topic   string
	Mapping ContentMapping
}

type ContentMappingService struct {
	repo ContentRepository
}

func NewContentService(repo ContentRepository) *ContentMappingService {
	return &ContentMappingService{repo: repo}
}

func (s *ContentMappingService) Execute(ctx context.Context, llm LLMClient, req *ContentExecuteRequest) (*ContentResult, error) {
	existing, err := s.repo.GetContentMapping(req.Topic)
	if err != nil {
		return nil, fmt.Errorf("get content mapping: %w", err)
	}

	if existing == nil {
		existing = make(ContentMapping)
	}

	var unmapped []string
	for _, v := range req.Values {
		if _, ok := existing[v]; !ok {
			unmapped = append(unmapped, v)
		}
	}

	if len(unmapped) > 0 {
		targets := extractTargets(existing)
		prompt := BuildContentMappingPrompt(req.Topic, unmapped, targets)
		resp, err := llm.Generate(ctx, prompt)
		if err != nil {
			return nil, fmt.Errorf("llm generate: %w", err)
		}

		content := extract.JSON(resp)
		var newMapping ContentMapping
		if err := json.Unmarshal([]byte(content), &newMapping); err != nil {
			return nil, fmt.Errorf("parse llm output: %w", err)
		}

		existing = existing.Merge(newMapping)

		if err := s.repo.SaveContentMapping(req.Topic, existing); err != nil {
			logger.L().Warn("save content mapping failed", zap.Error(err))
		}

		logger.L().Info("content mapping updated", zap.String("topic", req.Topic), zap.Int("new_mappings", len(newMapping)))
	}

	return &ContentResult{
		Topic:   req.Topic,
		Mapping: existing,
	}, nil
}

func (s *ContentMappingService) Set(req *ContentSetRequest) error {
	if err := s.repo.SaveContentMapping(req.Topic, req.Mapping); err != nil {
		return fmt.Errorf("save content mapping: %w", err)
	}
	logger.L().Info("content mapping set", zap.String("topic", req.Topic), zap.Int("mappings", len(req.Mapping)))
	return nil
}

func extractTargets(m ContentMapping) []string {
	seen := make(map[string]bool)
	var targets []string
	for _, v := range m {
		if !seen[v] {
			seen[v] = true
			targets = append(targets, v)
		}
	}
	return targets
}
