package notify

import (
	"encoding/json"

	"kgbrain/internal/logger"
	"kgbrain/internal/notify/feishu"
	"kgbrain/internal/orchestrator/task"
	"kgbrain/internal/profile"

	"go.uber.org/zap"
)

// Service 从 profile.Repo 读取通知配置, 在任务状态变更时发送通知.
type Service struct {
	repo *profile.Repo
}

func NewService(repo *profile.Repo) *Service {
	return &Service{repo: repo}
}

type Config struct {
	Channels []Channel `json:"channels"`
}

type Channel struct {
	Type   string         `json:"type"`
	Config map[string]any `json:"config"`
}

// NewTaskHook 返回一个 TaskHook, 根据 profile 的 NotifyConfig 发送通知.
func (s *Service) NewTaskHook() func(t *task.Task) {
	return func(t *task.Task) {
		snap := t.Snapshot()
		if snap.SessionID == "" {
			return
		}

		p, err := s.repo.Get(snap.SessionID)
		if err != nil || p == nil || p.NotifyConfig == "" {
			return
		}

		var cfg Config
		if err := json.Unmarshal([]byte(p.NotifyConfig), &cfg); err != nil {
			return
		}

		for _, ch := range cfg.Channels {
			s.sendByChannel(ch, snap)
		}
	}
}

func (s *Service) sendByChannel(ch Channel, snap task.TaskSnapshot) {
	switch ch.Type {
	case "feishu_webhook":
		s.sendFeishu(ch.Config, snap)
	default:
		logger.L().Warn("unknown notify channel type", zap.String("type", ch.Type))
	}
}

func (s *Service) sendFeishu(cfg map[string]any, snap task.TaskSnapshot) {
	webhookURL, _ := cfg["webhook_url"].(string)
	if webhookURL == "" {
		return
	}

	onStart, _ := cfg["on_start"].(bool)
	onEnd, _ := cfg["on_end"].(bool)
	onError, _ := cfg["on_error"].(bool)

	client := feishu.NewClient(webhookURL)

	switch snap.Status {
	case task.StatusRunning:
		if !onStart {
			return
		}
		title, content, color := feishu.BuildStartMessage(&snap)
		if err := client.SendCard(title, content, color); err != nil {
			logger.L().Warn("notify start failed", zap.String("task_id", snap.ID), zap.Error(err))
		}

	case task.StatusCompleted:
		if !onEnd {
			return
		}
		title, content, color := feishu.BuildCompleteMessage(&snap)
		if err := client.SendCard(title, content, color); err != nil {
			logger.L().Warn("notify complete failed", zap.String("task_id", snap.ID), zap.Error(err))
		}

	case task.StatusFailed:
		if !onError {
			return
		}
		title, content, color := feishu.BuildErrorMessage(&snap)
		if err := client.SendCard(title, content, color); err != nil {
			logger.L().Warn("notify error failed", zap.String("task_id", snap.ID), zap.Error(err))
		}
	}
}
