package connect

import (
	"connectrpc.com/grpchealth"
)

// NewHealthChecker 返回框架阶段的占位健康检查.
//
// 当前实现: grpchealth.StaticChecker, 对进程级 (service="") 健康请求
// 永远返回 StatusServing, 对未注册的具体 service 返回 CodeNotFound.
//
// 后续接入 DB / Redis 真实探活时, 在此处实现 grpchealth.Checker 接口即可,
// 不需改动 connect.Server 装配代码.
//
// 详细 TODO 见 TODO.md "grpchealth 接入真实探活".
func NewHealthChecker() grpchealth.Checker {
	return grpchealth.NewStaticChecker()
}

// ensureChecker 编译期校验: HealthChecker 别名与 grpchealth.Checker 接口兼容.
var _ HealthChecker = (grpchealth.Checker)(nil)
