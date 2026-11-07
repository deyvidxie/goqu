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
	NoDatabaseError struct {
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

func newNoDatabaseError(format string, args ...interface{}) error {
	return NoDatabaseError{baseError{msg: fmt.Sprintf(format, args...)}}
}

func IsNoDatabase(err error) bool {
	var target NoDatabaseError
	return errors.As(err, &target)
}
