package agents

import "context"

type Input struct {
	Data    map[string]any
	Options map[string]any
}

type Output struct {
	Status     string
	Data       map[string]any
	DurationMs int64
	Error      string
}

type Agent interface {
	Name() string
	DisplayName() string
	Description() string
	Execute(ctx context.Context, input Input) Output
}
