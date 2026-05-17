package profile

import "encoding/json"

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

func (p *Profile) ParseLLMConfig() (*LLMConfig, error) {
	var c LLMConfig
	if err := json.Unmarshal([]byte(p.LLMConfig), &c); err != nil {
		return nil, err
	}
	return &c, nil
}
