package rpc

import (
	"encoding/json"

	"kgbrain/pkg/jsonrpc"
)

func RegisterRpcDiscover(s *Server, doc json.RawMessage) {
	s.Register("rpc.discover", func(id string, _ json.RawMessage) jsonrpc.Response {
		return jsonrpc.NewResponse(id, doc)
	})
}
