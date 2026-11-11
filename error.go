package goqu

import "github.com/doug-martin/goqu/v9/internal/errors"

type (
	InvalidExpressionError   = errors.InvalidExpressionError
	UnsupportedError         = errors.UnsupportedError
	UnsupportedScanTypeError = errors.UnsupportedScanTypeError
	ColumnMismatchError      = errors.ColumnMismatchError
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

func IsColumnMismatch(err error) bool {
	return errors.IsColumnMismatch(err)
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

var (
	errEmptyDistinctOn    = errors.NewInvalidExpressionError("DistinctOn requires at least one column")
	errEmptySelect        = errors.NewInvalidExpressionError("Select requires at least one column; use ClearSelect for SELECT *")
	errEmptyGroupBy       = errors.NewInvalidExpressionError("GroupBy requires at least one column; use ClearGroupBy to drop GROUP BY")
	errEmptyOrder         = errors.NewInvalidExpressionError("Order requires at least one expression; use ClearOrder to drop ORDER BY")
	errEmptySelectAppend  = errors.NewInvalidExpressionError("SelectAppend requires at least one column")
	errEmptyGroupByAppend = errors.NewInvalidExpressionError("GroupByAppend requires at least one column")
	errEmptyOrderAppend   = errors.NewInvalidExpressionError("OrderAppend requires at least one expression")
	errEmptyOrderPrepend  = errors.NewInvalidExpressionError("OrderPrepend requires at least one expression")
	errNilOrder           = errors.NewInvalidExpressionError("Order expressions must not be nil")
)

func errUnsupportedFromArgument(from interface{}) error {
	return errors.NewInvalidExpressionError(
		"unsupported DeleteDataset#From argument %T, a string or identifier expression is required", from,
	)
}

func errUnsupportedTableType(table interface{}) error {
	return errors.NewInvalidExpressionError("unsupported table type %T, a string or expression is required", table)
}

func errUnknownDialect(dialect string) error {
	return errors.NewUnknownDialectError(
		"dialect %q is not registered; blank-import its dialect package or call RegisterDialect", dialect,
	)
}

func errNilInsertSource(from interface{}) error {
	return errors.NewInvalidExpressionError("insert source must not be nil, got %T", from)
}

func errIncompatibleInsertDialects(insertDialect, selectDialect string) error {
	return errors.NewInvalidExpressionError("incompatible dialects for INSERT (%q) and SELECT (%q)", insertDialect, selectDialect)
}
