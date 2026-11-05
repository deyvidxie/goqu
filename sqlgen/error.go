package sqlgen

import (
	"errors"
	"fmt"
)

type (
	baseError struct {
		msg string
	}
	UnsupportedError struct {
		baseError
	}
)

func (e baseError) Error() string {
	return "goqu: " + e.msg
}

func NewUnsupportedError(format string, args ...interface{}) error {
	return UnsupportedError{baseError{msg: fmt.Sprintf(format, args...)}}
}

func IsUnsupported(err error) bool {
	var target UnsupportedError
	return errors.As(err, &target)
}
