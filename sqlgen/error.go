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
	InvalidExpressionError struct {
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

func NewInvalidExpressionError(format string, args ...interface{}) error {
	return InvalidExpressionError{baseError{msg: fmt.Sprintf(format, args...)}}
}

func IsInvalidExpression(err error) bool {
	var target InvalidExpressionError
	return errors.As(err, &target)
}
