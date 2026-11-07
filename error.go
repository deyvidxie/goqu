package goqu

import (
	"errors"
	"fmt"
)

type (
	baseError struct {
		msg string
	}
	UnknownDialectError struct {
		baseError
	}
	NoQueryExecutorError struct {
		baseError
	}
)

func (e baseError) Error() string {
	return "goqu: " + e.msg
}

func newUnknownDialectError(format string, args ...interface{}) error {
	return UnknownDialectError{baseError{msg: fmt.Sprintf(format, args...)}}
}

func IsUnknownDialect(err error) bool {
	var target UnknownDialectError
	return errors.As(err, &target)
}

func newNoQueryExecutorError(format string, args ...interface{}) error {
	return NoQueryExecutorError{baseError{msg: fmt.Sprintf(format, args...)}}
}

func IsNoQueryExecutor(err error) bool {
	var target NoQueryExecutorError
	return errors.As(err, &target)
}
