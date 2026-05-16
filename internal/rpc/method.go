package rpc

import (
	"encoding/json"

	"kgbrain/pkg/jsonrpc"
)

// MethodHandler 是 JSON-RPC 方法的处理函数签名.
// id 是请求的 ID, 必须在响应中回显. 强制要求 string 类型.
type MethodHandler func(id string, params json.RawMessage) jsonrpc.Response
