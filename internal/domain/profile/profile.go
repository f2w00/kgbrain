package profile

import (
	"encoding/json"
	"errors"
)

type Profile struct {
	ID           string
	LLMConfig    string
	NotifyConfig string
	CreatedAt    string
	UpdatedAt    string
}

type LLMConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

func NewProfile(id, llmCfg, notifyCfg string) (*Profile, error) {
	if id == "" {
		return nil, errors.New("profile id is required")
	}
	if llmCfg == "" {
		return nil, errors.New("llm config is required")
	}
	return &Profile{
		ID:           id,
		LLMConfig:    llmCfg,
		NotifyConfig: notifyCfg,
	}, nil
}

func (p *Profile) ParseLLMConfig() (*LLMConfig, error) {
	var c LLMConfig
	if err := json.Unmarshal([]byte(p.LLMConfig), &c); err != nil {
		return nil, err
	}
	return &c, nil
}
