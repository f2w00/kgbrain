package resource

import "errors"

type notFoundError struct {
	id string
}

func (e *notFoundError) Error() string {
	return "resource not found"
}

func IsNotFound(err error) bool {
	var target *notFoundError
	return errors.As(err, &target)
}
