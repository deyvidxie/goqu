package goqu

import "github.com/doug-martin/goqu/v9/internal/errors"

type (
	UnknownDialectError  = errors.UnknownDialectError
	NoQueryExecutorError = errors.NoQueryExecutorError
)

func newUnknownDialectError(format string, args ...interface{}) error {
	return errors.NewUnknownDialectError(format, args...)
}

func IsUnknownDialect(err error) bool {
	return errors.IsUnknownDialect(err)
}

func IsNoQueryExecutor(err error) bool {
	return errors.IsNoQueryExecutor(err)
}
