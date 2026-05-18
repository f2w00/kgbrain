package rpc

import (
	"encoding/json"

	"kgbrain/pkg/jsonrpc"
)

type MethodHandler func(id string, params json.RawMessage) jsonrpc.Response
