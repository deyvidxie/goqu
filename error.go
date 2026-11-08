package goqu

import "github.com/doug-martin/goqu/v9/internal/errors"

type (
	InvalidExpressionError   = errors.InvalidExpressionError
	UnsupportedError         = errors.UnsupportedError
	UnsupportedScanTypeError = errors.UnsupportedScanTypeError
	UnknownDialectError      = errors.UnknownDialectError
	NoDatabaseError          = errors.NoDatabaseError
)

func IsInvalidExpression(err error) bool {
	return errors.IsInvalidExpression(err)
}

func IsUnsupported(err error) bool {
	return errors.IsUnsupported(err)
}

func IsUnsupportedScanType(err error) bool {
	return errors.IsUnsupportedScanType(err)
}

func IsUnknownDialect(err error) bool {
	return errors.IsUnknownDialect(err)
}

func IsNoDatabase(err error) bool {
	return errors.IsNoDatabase(err)
}

var errNoDatabase = errors.NewNoDatabaseError(
	"no database to execute the query, create the dataset with goqu.Database#From",
)

func errBadFromArgument(from interface{}) error {
	return errors.NewInvalidExpressionError(
		"unsupported DeleteDataset#From argument %T, a string or identifier expression is required", from,
	)
}

func errUnsupportedTableType(table interface{}) error {
	return errors.NewInvalidExpressionError("unsupported table type %T, a string or identifier expression is required", table)
}
