package resource

import (
	"errors"
	"strings"
)

const DatabaseTypePostgres = "postgres"

type LLMResource struct {
	ID             string
	Name           string
	BaseURL        string
	APIKey         string
	Model          string
	TimeoutSeconds int
	Temperature    *float64
	MaxConcurrency *int
	CreatedAt      string
	UpdatedAt      string
}

func (r *LLMResource) Validate() error {
	if err := validateModelResource(
		r.ID,
		r.BaseURL,
		r.APIKey,
		r.Model,
		r.MaxConcurrency,
	); err != nil {
		return err
	}
	return nil
}

type EmbeddingResource struct {
	ID             string
	Name           string
	BaseURL        string
	APIKey         string
	Model          string
	TimeoutSeconds int
	MaxConcurrency *int
	CreatedAt      string
	UpdatedAt      string
}

func (r *EmbeddingResource) Validate() error {
	if err := validateModelResource(
		r.ID,
		r.BaseURL,
		r.APIKey,
		r.Model,
		r.MaxConcurrency,
	); err != nil {
		return err
	}
	return nil
}

func validateModelResource(
	id string,
	baseURL string,
	apiKey string,
	model string,
	maxConcurrency *int,
) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("resource id is required")
	}
	if strings.TrimSpace(baseURL) == "" {
		return errors.New("base_url is required")
	}
	if strings.TrimSpace(apiKey) == "" {
		return errors.New("api_key is required")
	}
	if strings.TrimSpace(model) == "" {
		return errors.New("model is required")
	}
	if maxConcurrency != nil && *maxConcurrency <= 0 {
		return errors.New("max_concurrency must be greater than 0")
	}
	return nil
}

type DatabaseResource struct {
	ID        string
	Name      string
	Type      string
	Host      string
	Port      int
	Database  string
	User      string
	Password  string
	SSLMode   string
	CreatedAt string
	UpdatedAt string
}

func (r *DatabaseResource) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return errors.New("resource id is required")
	}
	if strings.TrimSpace(r.Type) == "" {
		return errors.New("database type is required")
	}
	if r.Type != DatabaseTypePostgres {
		return errors.New("unsupported database type")
	}
	if strings.TrimSpace(r.Host) == "" {
		return errors.New("host is required")
	}
	if r.Port <= 0 {
		return errors.New("port must be greater than 0")
	}
	if strings.TrimSpace(r.Database) == "" {
		return errors.New("database is required")
	}
	if strings.TrimSpace(r.User) == "" {
		return errors.New("user is required")
	}
	return nil
}
