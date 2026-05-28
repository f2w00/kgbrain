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
	Topic   string
	Values  []string
	Targets []string
}

type ContentSetRequest struct {
	Topic   string
	Mapping ContentMapping
}

type TargetsSetRequest struct {
	Topic   string
	Targets []string
}

type TargetsResult struct {
	Topic   string   `json:"topic"`
	Targets []string `json:"targets"`
}

type ContentResult struct {
	Topic   string         `json:"topic"`
	Mapping ContentMapping `json:"mapping"`
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

	var targets []string
	if len(req.Targets) > 0 {
		targets = req.Targets
		if err := s.repo.SaveTargets(req.Topic, targets); err != nil {
			logger.L().Warn("save targets failed", zap.Error(err))
		}
	} else {
		targets, err = s.repo.GetTargets(req.Topic)
		if err != nil {
			return nil, fmt.Errorf("get targets: %w", err)
		}
		if len(targets) == 0 {
			return nil, fmt.Errorf("targets not set for topic %q. Use mapping.content.targets.set to set targets first", req.Topic)
		}
	}

	var unmapped []string
	for _, v := range req.Values {
		if _, ok := existing[v]; !ok {
			unmapped = append(unmapped, v)
		}
	}

	if len(unmapped) > 0 {
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

		logger.L().Debug("content mapping updated", zap.String("topic", req.Topic), zap.Int("new_mappings", len(newMapping)))
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
	logger.L().Debug("content mapping set", zap.String("topic", req.Topic), zap.Int("mappings", len(req.Mapping)))
	return nil
}

func (s *ContentMappingService) SetTargets(req *TargetsSetRequest) error {
	if err := s.repo.SaveTargets(req.Topic, req.Targets); err != nil {
		return fmt.Errorf("save targets: %w", err)
	}
	logger.L().Debug("targets set", zap.String("topic", req.Topic), zap.Int("targets", len(req.Targets)))
	return nil
}

func (s *ContentMappingService) GetTargets(topic string) (*TargetsResult, error) {
	targets, err := s.repo.GetTargets(topic)
	if err != nil {
		return nil, fmt.Errorf("get targets: %w", err)
	}
	return &TargetsResult{Topic: topic, Targets: targets}, nil
}
