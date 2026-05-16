package profile

import "encoding/json"

// Profile 保存用户的 LLM 配置和通知配置, SQLite 持久化, 不过期.
type Profile struct {
	ID           string
	LLMConfig    string // JSON
	NotifyConfig string // JSON (channels 数组), 可为空
	CreatedAt    string
	UpdatedAt    string
}

// LLMConfig 是 Profile.LLMConfig 的结构化表示.
type LLMConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

// ParseLLMConfig 将 LLMConfig (JSON string) 解析为结构体.
func (p *Profile) ParseLLMConfig() (*LLMConfig, error) {
	var c LLMConfig
	if err := json.Unmarshal([]byte(p.LLMConfig), &c); err != nil {
		return nil, err
	}
	return &c, nil
}
