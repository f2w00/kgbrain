package handlers

import (
	"encoding/json"

	"kgbrain/internal/delivery/rpc"
	"kgbrain/pkg/jsonrpc"
)

func RegisterSystemMethods(s *rpc.Server, doc json.RawMessage) {
	s.Register("rpc.discover", func(id string, _ json.RawMessage) jsonrpc.Response {
		return jsonrpc.NewResponse(id, doc)
	})
}
