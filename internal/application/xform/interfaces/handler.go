package interfaces

import "context"

// MessageHandler 定义了单行数据处理的业务接口.
// 具体实现位于 ../message_handler.go.
type MessageHandler interface {
	Handle(ctx context.Context, taskID string, systemPrompt string, data map[string]any) error
}
