// errors.go 定义 enrichextract 模块内部错误类型和判别函数。
package enrichextract

import "errors"

type validationError struct {
	message string
}

func (e *validationError) Error() string {
	return e.message
}

// IsValidationError 判断错误是否来自启动请求参数校验。
func IsValidationError(err error) bool {
	var target *validationError
	return errors.As(err, &target)
}

type notFoundError struct {
	message string
}

func (e *notFoundError) Error() string {
	return e.message
}

// IsNotFound 判断错误是否为 job 或依赖 resource 不存在。
func IsNotFound(err error) bool {
	var target *notFoundError
	return errors.As(err, &target)
}
