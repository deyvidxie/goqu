package goqu

import (
	"slices"
	"strings"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/errors"
)

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
	errEmptySelect         = errors.NewInvalidExpressionError("Select requires at least one column; use ClearSelect for SELECT *")
	errEmptySelectDistinct = errors.NewInvalidExpressionError("SelectDistinct requires at least one column")
	errEmptyDistinctOn     = errors.NewInvalidExpressionError("DistinctOn requires at least one column; use Distinct() for a plain DISTINCT")
	errEmptyGroupBy        = errors.NewInvalidExpressionError("GroupBy requires at least one column; use ClearGroupBy to drop GROUP BY")
	errEmptyOrder          = errors.NewInvalidExpressionError("Order requires at least one expression; use ClearOrder to drop ORDER BY")
	errEmptyOrderAppend    = errors.NewInvalidExpressionError("OrderAppend requires at least one expression")
	errEmptyOrderPrepend   = errors.NewInvalidExpressionError("OrderPrepend requires at least one expression")
	errNilOrder            = errors.NewInvalidExpressionError("Order expressions must not be nil")
	errNilOrderAppend      = errors.NewInvalidExpressionError("OrderAppend expressions must not be nil")
	errNilOrderPrepend     = errors.NewInvalidExpressionError("OrderPrepend expressions must not be nil")
	errEmptyWhere          = errors.NewInvalidExpressionError("Where requires at least one expression; use ClearWhere to drop WHERE")
	errEmptyHaving         = errors.NewInvalidExpressionError("Having requires at least one expression; use ClearHaving to drop HAVING")
	errEmptyQualify        = errors.NewInvalidExpressionError("Qualify requires at least one expression; use ClearQualify to drop QUALIFY")
	errNilAlign            = errors.NewInvalidExpressionError("Align requires an AlignExpression; use ClearAlign to drop ALIGN")
	errEmptyCols           = errors.NewInvalidExpressionError("Cols requires at least one column; use ClearCols to drop Cols")
	errEmptyColsAppend     = errors.NewInvalidExpressionError("ColsAppend requires at least one column")
	errEmptySelectAppend   = errors.NewInvalidExpressionError("SelectAppend requires at least one column")
	errNilOnConflict       = errors.NewInvalidExpressionError("OnConflict requires a conflict expression; use ClearOnConflict to drop it")
	errEmptyFrom           = errors.NewInvalidExpressionError("From requires at least one table; use ClearFrom to drop it")
	errEmptyReturning      = errors.NewInvalidExpressionError("Returning requires at least one column; use ClearReturning to drop it")
	errEmptyTruncateTable  = errors.NewInvalidExpressionError("Truncate requires at least one table")
	errEmptyWindow         = errors.NewInvalidExpressionError("Window requires at least one window; use ClearWindow to drop it")
	errEmptyCTEName        = errors.NewInvalidExpressionError("With requires a name")
	errEmptyWindowAppend   = errors.NewInvalidExpressionError("WindowAppend requires at least one window")
	errEmptyGroupByAppend  = errors.NewInvalidExpressionError("GroupByAppend requires at least one column")
	errEmptyRows           = errors.NewInvalidExpressionError("Rows requires at least one row")
	errEmptyVals           = errors.NewInvalidExpressionError("Vals requires at least one row")
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
	return errors.NewInvalidExpressionError("FromQuery requires a query, got %T; use ClearFromQuery to drop it", from)
}

func errIncompatibleInsertDialects(insertDialect, selectDialect string) error {
	return errors.NewInvalidExpressionError("incompatible dialects for INSERT (%q) and SELECT (%q)", insertDialect, selectDialect)
}

func checkOrder(order []exp.OrderedExpression, errEmpty, errNil error) error {
	switch {
	case len(order) == 0:
		return errEmpty
	case slices.Contains(order, nil):
		return errNil
	default:
		return nil
	}
}

func errNilSubquery(source interface{}) error {
	return errors.NewInvalidExpressionError("subquery must not be nil, got %T", source)
}

func errEmptyLockOf(method string) error {
	return errors.NewInvalidExpressionError(
		"%s requires at least one table; use %s without Of to lock every table", method, strings.TrimSuffix(method, "Of"),
	)
}

func errUncarriedClause(method, clause string) error {
	return errors.NewInvalidExpressionError("SelectDataset.%s cannot carry the select's %s clause", method, clause)
}
