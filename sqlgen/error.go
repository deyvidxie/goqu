package sqlgen

import (
	"database/sql/driver"
	"strconv"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/errors"
)

var (
	errNoSourceForInsert            = errors.NewInvalidExpressionError("no source found when generating insert sql")
	errNoSourceForUpdate            = errors.NewInvalidExpressionError("no source found when generating update sql")
	errNoSourceForDelete            = errors.NewInvalidExpressionError("no source found when generating delete sql")
	errNoSourceForTruncate          = errors.NewInvalidExpressionError("no source found when generating truncate sql")
	errNoSetValuesForUpdate         = errors.NewInvalidExpressionError("no set values found when generating update sql")
	errNoUpdatedValuesProvided      = errors.NewInvalidExpressionError("no update values provided")
	errConflictUpdateValuesRequired = errors.NewInvalidExpressionError("values are required for on conflict update expression")

	errColsWithoutValsForInsert = errors.NewInvalidExpressionError("Cols without Vals or FromQuery found when generating insert sql")
	errEmptyValsRowForInsert    = errors.NewInvalidExpressionError("empty Vals row without Cols found when generating insert sql")
	errColsWithRows             = errors.NewInvalidExpressionError(
		"Cols cannot be combined with Rows; Rows take their columns from the records",
	)
	errMultipleSourcesForInsert = errors.NewInvalidExpressionError(
		"more than one of Rows, Vals and FromQuery found when generating insert sql",
	)
	errEmptyRows           = errors.NewInvalidExpressionError(`Rows requires at least one row`)
	errEmptyConflictTarget = errors.NewInvalidExpressionError(`DoNothing target requires at least one column`)
	errEmptyVals           = errors.NewInvalidExpressionError(`Vals requires at least one row`)
	errEmptyCols           = errors.NewInvalidExpressionError(`Cols requires at least one column`)

	errEmptyIdentifier = errors.NewInvalidExpressionError(
		`an empty identifier was encountered, please specify a "schema", "table" or "column"`,
	)
	errEmptyCaseWhens        = errors.NewInvalidExpressionError(`when conditions not found for case statement`)
	errEmptyAndCondition     = errors.NewInvalidExpressionError(`empty AND condition`)
	errEmptyOrCondition      = errors.NewInvalidExpressionError(`empty OR condition`)
	errEmptyOrder            = errors.NewInvalidExpressionError(`Order requires at least one expression`)
	errEmptyGroupBy          = errors.NewInvalidExpressionError(`GroupBy requires at least one column`)
	errEmptyFrom             = errors.NewInvalidExpressionError(`From requires at least one table`)
	errEmptyReturning        = errors.NewInvalidExpressionError(`Returning requires at least one column`)
	errEmptyTruncateTable    = errors.NewInvalidExpressionError(`Truncate requires at least one table`)
	errEmptyWindow           = errors.NewInvalidExpressionError(`Window requires at least one window`)
	errEmptyCTEName          = errors.NewInvalidExpressionError(`With requires a name`)
	errEmptyDistinctOn       = errors.NewInvalidExpressionError(`DistinctOn requires at least one column`)
	errEmptyPartitionBy      = errors.NewInvalidExpressionError(`PartitionBy requires at least one column`)
	errEmptyWindowOrderBy    = errors.NewInvalidExpressionError(`OrderBy requires at least one column`)
	errEmptySelect           = errors.NewInvalidExpressionError(`Select requires at least one column`)
	errUnexpectedNamedWindow = errors.NewInvalidExpressionError(`unexpected named window function`)
	errNoWindowName          = errors.NewInvalidExpressionError("window expression has no valid name")

	errExcludedOutsideConflictUpdate = errors.NewInvalidExpressionError("Excluded is only valid in an OnConflict DoUpdate")
	errExcludedWithoutColumn         = errors.NewInvalidExpressionError("Excluded requires a column name")
	errExcludedRowAliasQualified     = errors.NewInvalidExpressionError("Excluded requires an unqualified insert alias")
	errEmptyInsertAlias              = errors.NewInvalidExpressionError("Excluded requires a non-empty insert alias")
	errExcludedRowAliasUnresolved    = errors.NewInvalidExpressionError(
		"Excluded rendered differently on a second pass; Valuers and expressions in DoUpdate must render deterministically",
	)
	errDefaultOutsideValues = errors.NewInvalidExpressionError("Default is only valid as an INSERT or UPDATE SET value")

	errIntervalCountCondition = errors.NewInvalidExpressionError("Interval count goqu.Ex/ExOr is a condition, not a count")
	errJSONCandidateInvalid   = errors.NewInvalidExpressionError("JSONContains candidate is not valid JSON")
	errJSONCandidateCondition = errors.NewInvalidExpressionError(
		"JSONContains candidate goqu.Ex/ExOr is a condition, not a JSON object; use map[string]any",
	)
)

func errCTENotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support CTE WITH clause [dialect=%s]", dialect)
}

func errRecursiveCTENotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support CTE WITH RECURSIVE clause [dialect=%s]", dialect)
}

func errReturnNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support RETURNING clause [dialect=%s]", dialect)
}

func errDistinctOnNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support DISTINCT ON clause [dialect=%s]", dialect)
}

func errAggregateOrderByNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support ORDER BY in function arguments [dialect=%s]", dialect)
}

func errWindowNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support window functions [dialect=%s]", dialect)
}

func errWindowClauseNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support the named WINDOW clause [dialect=%s]", dialect)
}

func errNestedCTENotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support a WITH clause inside a subquery [dialect=%s]", dialect)
}

func errRangeQueryNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support range queries [dialect=%s]", dialect)
}

func errRangeDuration(clause string, d time.Duration) error {
	return errors.NewInvalidExpressionError("%s duration must be a positive whole number of milliseconds, got %s", clause, d)
}

func errAlignOrigin(origin time.Time) error {
	return errors.NewInvalidExpressionError("ALIGN TO origin must be a whole number of milliseconds, got %s", origin.Format(time.RFC3339Nano))
}

func errRangeFillValue(v float64) error {
	return errors.NewInvalidExpressionError("FILL value must be a finite non-negative number, got %v", v)
}

func errUnknownRangeFill(kind exp.RangeFillKind) error {
	return errors.NewInvalidExpressionError("unknown FILL kind %d", kind)
}

func errQualifyNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support QUALIFY clause [dialect=%s]", dialect)
}

func errLateralNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support lateral expressions [dialect=%s]", dialect)
}

func errOrderByOnUpdateNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support ORDER BY on UPDATE [dialect=%s]", dialect)
}

func errLimitOnUpdateNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support LIMIT on UPDATE [dialect=%s]", dialect)
}

func errOrderByOnDeleteNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support ORDER BY on DELETE [dialect=%s]", dialect)
}

func errLimitOnDeleteNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support LIMIT on DELETE [dialect=%s]", dialect)
}

func errMultipleUpdateTablesNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support multiple tables in UPDATE [dialect=%s]", dialect)
}

func errStatementNotSupported(dialect, sqlType string) error {
	return errors.NewUnsupportedError("dialect does not support %s statements [dialect=%s]", sqlType, dialect)
}

func errFragmentNotSupported(dialect, sqlType string, f SQLFragmentType) error {
	return errors.NewUnsupportedError("dialect does not support %s SQL fragment %s [dialect=%s]", sqlType, f, dialect)
}

func errRowLengthMismatch(expected, actual int) error {
	return errors.NewInvalidExpressionError("rows with different value length, expected %d got %d", expected, actual)
}

func errOnConflictNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support upsert [dialect=%s]", dialect)
}

func errConflictTargetNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support conflict target clause [dialect=%s]", dialect)
}

func errNoOpConflictUpdateWithoutColumns(dialect string) error {
	return errors.NewUnsupportedError(
		"dialect renders DoNothing as a no-op ON DUPLICATE KEY UPDATE, which needs an insert column list [dialect=%s]", dialect,
	)
}

func errUnknownConflictExpression(o exp.ConflictExpression) error {
	return errors.NewInvalidExpressionError("unsupported conflict expression %T, use DoNothing or DoUpdate", o)
}

func errConflictWithDefaultValuesNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support upsert with DEFAULT VALUES [dialect=%s]", dialect)
}

func errUpsertWithWhereNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support upsert with where clause [dialect=%s]", dialect)
}

func errInsertRowAliasSelectNotSupported(dialect string) error {
	return errors.NewUnsupportedError(
		"dialect does not support an INSERT row alias with INSERT ... SELECT without Cols(...).FromQuery(...) [dialect=%s]",
		dialect,
	)
}

func errExcludedInsertSelectNotSupported(dialect string) error {
	return errors.NewUnsupportedError(
		"dialect does not support Excluded with INSERT ... SELECT without Cols(...).FromQuery(...) [dialect=%s]", dialect,
	)
}

func errExcludedFragmentMissing(dialect, fragment string) error {
	return errors.NewUnsupportedError("dialect has no %s for Excluded [dialect=%s]", fragment, dialect)
}

func errExcludedRowAliasMissing(dialect string) error {
	return errors.NewUnsupportedError("dialect has no ExcludedRowAlias for Excluded, set an insert alias with As [dialect=%s]", dialect)
}

func errUnknownExcludedStyle(dialect string, style ExcludedStyle) error {
	return errors.NewUnsupportedError("dialect has unknown ExcludedStyle %d [dialect=%s]", style, dialect)
}

func errJoinTypeNotSupported(dialect string, j exp.JoinExpression) error {
	return errors.NewUnsupportedError("dialect does not support %v [dialect=%s]", j.JoinType(), dialect)
}

func errJoinUsingNotSupported(dialect string, j exp.JoinExpression) error {
	return errors.NewUnsupportedError("dialect does not support USING for %v, use an ON condition [dialect=%s]", j.JoinType(), dialect)
}

func errJoinConditionRequired(j exp.JoinExpression) error {
	return errors.NewInvalidExpressionError("join condition required for conditioned join %v", j.JoinType())
}

func errLockStrengthNotSupported(dialect string, strength exp.LockStrength) error {
	return errors.NewUnsupportedError("dialect does not support %s [dialect=%s]", lockStrengthSQL[strength], dialect)
}

func errLockWaitOptionNotSupported(dialect string, option exp.WaitOption) error {
	return errors.NewUnsupportedError("dialect does not support %s [dialect=%s]", lockWaitOptionSQL[option], dialect)
}

func errLockOfNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support OF in a locking clause [dialect=%s]", dialect)
}

func errBooleanDataTypeNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support the boolean data type [dialect=%s]", dialect)
}

func errEmptyListFragmentMissing(dialect, fragment, list string) error {
	return errors.NewUnsupportedError("dialect has no %s for an empty %s list [dialect=%s]", fragment, list, dialect)
}

func errBooleanOperatorNotSupported(dialect string, op exp.BooleanOperation, caseInsensitiveLike bool) error {
	if caseInsensitiveLike && (op == exp.LikeOp || op == exp.NotLikeOp) {
		return errors.NewUnsupportedError("dialect does not support boolean operator %q, use ILike/NotILike [dialect=%s]", op, dialect)
	}
	return errors.NewUnsupportedError("dialect does not support boolean operator %q [dialect=%s]", op, dialect)
}

func errBitwiseOperatorNotSupported(dialect string, op exp.BitwiseOperation) error {
	return errors.NewUnsupportedError("dialect does not support bitwise operator %q [dialect=%s]", op, dialect)
}

func errArithmeticOperatorNotSupported(dialect string, op exp.ArithmeticOperation) error {
	return errors.NewUnsupportedError("dialect does not support arithmetic operator %q [dialect=%s]", op, dialect)
}

func errRangeOperatorNotSupported(dialect string, op exp.RangeOperation) error {
	return errors.NewUnsupportedError("dialect does not support range operator %q [dialect=%s]", op, dialect)
}

func errUnsupportedExpressionType(e exp.Expression) error {
	return errors.NewInvalidExpressionError("unsupported expression type %T", e)
}

func errUnsupportedColumnType(col interface{}) error {
	return errors.NewInvalidExpressionError("unsupported column type %T, a string or LiteralExpression is required", col)
}

func errUnencodableValue(val interface{}) error {
	return errors.NewInvalidExpressionError("unable to encode value %+v", val)
}

func errValuerValue(v driver.Valuer, cause error) error {
	return errors.NewInvalidExpressionErrorWithCause(cause, "driver.Valuer %T", v)
}

func errInvalidCollation(collation string) error {
	return errors.NewInvalidExpressionError("invalid collation name %q", collation)
}

func errTimeFSPCount(n int) error {
	return errors.NewInvalidExpressionError("Now and UTCTimestamp take at most one fsp, got %d", n)
}

func errTimeFSPRange(fsp int) error {
	return errors.NewInvalidExpressionError("Now and UTCTimestamp fsp must be between 0 and %d, got %d", microsecondFSP, fsp)
}

func errTimeFSPNotSupported(dialect string, fsp int, supported ...int) error {
	names := make([]string, len(supported))
	for i, s := range supported {
		names[i] = strconv.Itoa(s)
	}
	if len(names) == 0 {
		return errors.NewUnsupportedError("dialect does not support fsp %d [dialect=%s]", fsp, dialect)
	}
	return errors.NewUnsupportedError("dialect does not support fsp %d, want one of %s [dialect=%s]", fsp, strings.Join(names, ", "), dialect)
}

func errUTCTimestampNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support UTCTimestamp [dialect=%s]", dialect)
}

func errIntervalNotExact(dialect string, count any, unit, target exp.IntervalUnit) error {
	return errors.NewUnsupportedError("dialect cannot represent Interval %v %s exactly as %s [dialect=%s]", count, unit, target, dialect)
}

func errIntervalConversionNeedsCount(dialect string, unit, target exp.IntervalUnit) error {
	return errors.NewUnsupportedError(
		"dialect renders Interval unit %s as %s, which needs an integer count [dialect=%s]", unit, target, dialect,
	)
}

func errUnknownTimeFunction(function exp.TimeFunction) error {
	return errors.NewInvalidExpressionError("unknown TimeFunction %d", function)
}

func errUnknownTimeStyle(dialect string, style TimeStyle) error {
	return errors.NewUnsupportedError("dialect has unknown TimeStyle %d [dialect=%s]", style, dialect)
}

func errUnknownIntervalUnit(unit exp.IntervalUnit) error {
	return errors.NewInvalidExpressionError("unknown Interval unit %q, want one of %s", unit, intervalUnitNames)
}

func errIntervalCount(dialect string, count, value interface{}, unit exp.IntervalUnit, fractionalSeconds bool) error {
	accepted := "an integer or an expression"
	if fractionalSeconds {
		accepted = "an integer, an expression or, with SECOND, a finite number"
	}
	return errors.NewInvalidExpressionError("Interval count must be %s, got %T %v for %s [dialect=%s]", accepted, count, value, unit, dialect)
}

func errIntervalCountValue(cause error) error {
	return errors.NewInvalidExpressionErrorWithCause(cause, "Interval count")
}

func errIntervalNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support Interval [dialect=%s]", dialect)
}

func errJSONFunctionNotSupported(dialect, function string) error {
	return errors.NewUnsupportedError("dialect does not support %s [dialect=%s]", function, dialect)
}

func errJSONContainsPaths(n int) error {
	return errors.NewInvalidExpressionError("JSONContains takes at most one path, got %d", n)
}

func errUnknownJSONFunction(function exp.JSONFunction) error {
	return errors.NewInvalidExpressionError("unknown JSONFunction %d", function)
}

func errJSONCandidate(cause error) error {
	return errors.NewInvalidExpressionErrorWithCause(cause, "JSONContains candidate")
}

func errOffsetWithoutOrder(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support OFFSET without ORDER BY [dialect=%s]", dialect)
}

func errLimitZeroWithOffsetNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support LIMIT 0 with an OFFSET [dialect=%s]", dialect)
}

func errMultipleTruncateTablesNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support multiple tables in TRUNCATE [dialect=%s]", dialect)
}

func errTruncateIdentityNotSupported(dialect, identity string) error {
	return errors.NewUnsupportedError(
		"dialect does not support %s IDENTITY in TRUNCATE [dialect=%s]", strings.ToUpper(identity), dialect,
	)
}

func errTruncateCascadeNotSupported(dialect string, cascade bool) error {
	option := "RESTRICT"
	if cascade {
		option = "CASCADE"
	}
	return errors.NewUnsupportedError("dialect does not support %s in TRUNCATE [dialect=%s]", option, dialect)
}

func errInsertRowAliasNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support an INSERT row alias [dialect=%s]", dialect)
}

func errListAsValue(val interface{}) error {
	return errors.NewInvalidExpressionError(
		"a list (%T) is not an INSERT or UPDATE value; wrap it in a driver.Valuer to store it as one value", val,
	)
}

func errDefaultValueNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support DEFAULT as an INSERT or UPDATE value [dialect=%s]", dialect)
}

func errEmptyListValue(val interface{}) error {
	return errors.NewInvalidExpressionError(
		"an empty list (%T) is only accepted as the whole right side of In or NotIn", val,
	)
}

func errListAsSingleValue(position string, val interface{}) error {
	return errors.NewInvalidExpressionError("%s takes a single value, got a list (%T)", position, val)
}

func errInWithoutList(op exp.BooleanOperation, val interface{}) error {
	return errors.NewInvalidExpressionError("operator %q takes a list or a subquery, got %T; use In or NotIn", op, val)
}

func errListForAnyAll(name string, val interface{}) error {
	return errors.NewInvalidExpressionError(
		"%s takes an array value or a subquery, got a list (%T); use In or NotIn, a driver.Valuer array or a subquery",
		strings.TrimSpace(name), val,
	)
}

func errStructWithoutColumns(v interface{}) error {
	return errors.NewInvalidExpressionError("%T has no columns", v)
}

func errNilSubquery(a exp.AppendableExpression) error {
	return errors.NewInvalidExpressionError("subquery must not be nil, got %T", a)
}

func errInvalidUTF8Literal(v interface{}) error {
	return errors.NewInvalidExpressionError("%T value is not valid UTF-8 and cannot be interpolated; use Prepared(true)", v)
}
