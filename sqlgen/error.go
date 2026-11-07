package sqlgen

import "github.com/doug-martin/goqu/v9/internal/errors"

type (
	UnsupportedError       = errors.UnsupportedError
	InvalidExpressionError = errors.InvalidExpressionError
)

func NewUnsupportedError(format string, args ...interface{}) error {
	return errors.NewUnsupportedError(format, args...)
}

func IsUnsupported(err error) bool {
	return errors.IsUnsupported(err)
}

func NewInvalidExpressionError(format string, args ...interface{}) error {
	return errors.NewInvalidExpressionError(format, args...)
}

func NewInvalidExpressionErrorWithCause(cause error, format string, args ...interface{}) error {
	return errors.NewInvalidExpressionErrorWithCause(cause, format, args...)
}

func IsInvalidExpression(err error) bool {
	return errors.IsInvalidExpression(err)
}
