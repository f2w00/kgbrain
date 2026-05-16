package feishu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	webhookURL string
	httpClient *http.Client
}

func NewClient(webhookURL string) *Client {
	return &Client{
		webhookURL: webhookURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type cardContent struct {
	Config configBlock `json:"config"`
	Header headerBlock  `json:"header"`
	Elements []elementBlock `json:"elements"`
}

type configBlock struct {
	WideScreenMode bool `json:"wide_screen_mode"`
	EnableForward  bool `json:"enable_forward"`
}

type headerBlock struct {
	Title   titleBlock `json:"title"`
	Template string    `json:"template"`
}

type titleBlock struct {
	Content string `json:"content"`
	Tag     string `json:"tag"`
}

type elementBlock struct {
	Tag     string      `json:"tag"`
	Text    *textContent `json:"text,omitempty"`
	Content string      `json:"content,omitempty"`
	Mode    string      `json:"mode,omitempty"`
}

type textContent struct {
	Content string `json:"content"`
	Tag     string `json:"tag"`
}

func (c *Client) SendCard(title, content string, color string) error {
	if c.webhookURL == "" {
		return nil
	}

	card := cardContent{
		Config: configBlock{
			WideScreenMode: true,
			EnableForward:  true,
		},
		Header: headerBlock{
			Title: titleBlock{
				Content: title,
				Tag:     "plain_text",
			},
			Template: color,
		},
		Elements: []elementBlock{
			{
				Tag: "markdown",
				Content: content,
			},
		},
	}

	payload := map[string]any{
		"msg_type": "interactive",
		"card":     card,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	resp, err := c.httpClient.Post(c.webhookURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}
