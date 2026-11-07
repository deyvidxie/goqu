package errors

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
		cause error
	}
	UnsupportedScanTypeError struct {
		baseError
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

func NewUnsupportedError(format string, args ...interface{}) error {
	return UnsupportedError{baseError{msg: fmt.Sprintf(format, args...)}}
}

func IsUnsupported(err error) bool {
	var target UnsupportedError
	return errors.As(err, &target)
}

func NewInvalidExpressionError(format string, args ...interface{}) error {
	return InvalidExpressionError{baseError: baseError{msg: fmt.Sprintf(format, args...)}}
}

func NewInvalidExpressionErrorWithCause(cause error, format string, args ...interface{}) error {
	return InvalidExpressionError{baseError: baseError{msg: fmt.Sprintf(format, args...)}, cause: cause}
}

func (e InvalidExpressionError) Error() string {
	if e.cause == nil {
		return e.baseError.Error()
	}
	return e.baseError.Error() + ": " + e.cause.Error()
}

func (e InvalidExpressionError) Unwrap() error {
	return e.cause
}

func IsInvalidExpression(err error) bool {
	var target InvalidExpressionError
	return errors.As(err, &target)
}

func NewUnsupportedScanTypeError(format string, args ...interface{}) error {
	return UnsupportedScanTypeError{baseError{msg: fmt.Sprintf(format, args...)}}
}

func IsUnsupportedScanType(err error) bool {
	var target UnsupportedScanTypeError
	return errors.As(err, &target)
}

func NewUnknownDialectError(format string, args ...interface{}) error {
	return UnknownDialectError{baseError{msg: fmt.Sprintf(format, args...)}}
}

func IsUnknownDialect(err error) bool {
	var target UnknownDialectError
	return errors.As(err, &target)
}

func NewNoQueryExecutorError(format string, args ...interface{}) error {
	return NoQueryExecutorError{baseError{msg: fmt.Sprintf(format, args...)}}
}

func IsNoQueryExecutor(err error) bool {
	var target NoQueryExecutorError
	return errors.As(err, &target)
}
