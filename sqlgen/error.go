package sqlgen

import (
	"database/sql/driver"
	"strconv"
	"strings"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/errors"
)

var errNoUpdatedValuesProvided = errors.NewInvalidExpressionError("no update values provided")

func errInvalidCollation(collation string) error {
	return errors.NewInvalidExpressionError("invalid collation name %q", collation)
}

func errCTENotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support CTE WITH clause [dialect=%s]", dialect)
}

func errRecursiveCTENotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support CTE WITH RECURSIVE clause [dialect=%s]", dialect)
}

func errReturnNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support RETURNING clause [dialect=%s]", dialect)
}

func errConflictTargetNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support Conflict Target clause [dialect=%s]", dialect)
}

func errNotSupportedFragment(dialect, sqlType string, f SQLFragmentType) error {
	return errors.NewUnsupportedError("unsupported %s SQL fragment %s [dialect=%s]", sqlType, f, dialect)
}

var errNoSourceForDelete = errors.NewInvalidExpressionError("no source found when generating delete sql")

func errOrderByOnDeleteNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support ORDER BY on DELETE [dialect=%s]", dialect)
}

func errLimitOnDeleteNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support LIMIT on DELETE [dialect=%s]", dialect)
}

var errEmptyIdentifier = errors.NewInvalidExpressionError(
	`an empty identifier was encountered, please specify a "schema", "table" or "column"`,
)

var errUnexpectedNamedWindow = errors.NewInvalidExpressionError(`unexpected named window function`)

var errEmptyCaseWhens = errors.NewInvalidExpressionError(`when conditions not found for case statement`)

var errExcludedOutsideConflictUpdate = errors.NewInvalidExpressionError("Excluded is only valid in an OnConflict DoUpdate")

var errExcludedWithoutColumn = errors.NewInvalidExpressionError("Excluded requires a column name")

var errJSONCandidateInvalid = errors.NewInvalidExpressionError("JSONContains candidate is not valid JSON")

var errJSONCandidateCondition = errors.NewInvalidExpressionError(
	"JSONContains candidate goqu.Ex/ExOr is a condition, not a JSON object; use map[string]any",
)

var errIntervalCountCondition = errors.NewInvalidExpressionError("Interval count goqu.Ex/ExOr is a condition, not a count")

var errExcludedRowAliasUnresolved = errors.NewInvalidExpressionError(
	"Excluded rendered differently on a second pass; Valuers and expressions in DoUpdate must render deterministically",
)

func errUnsupportedExpressionType(e exp.Expression) error {
	return errors.NewInvalidExpressionError("unsupported expression type %T", e)
}

func errUnsupportedIdentifierExpression(t interface{}) error {
	return errors.NewInvalidExpressionError("unsupported column type %T, a string or LiteralExpression is required", t)
}

func errUnsupportedBooleanExpressionOperator(dialect string, op exp.BooleanOperation, caseInsensitiveLike bool) error {
	if caseInsensitiveLike && (op == exp.LikeOp || op == exp.NotLikeOp) {
		return errors.NewUnsupportedError("boolean operator '%+v' not supported, use ILike/NotILike [dialect=%s]", op, dialect)
	}
	return errors.NewUnsupportedError("boolean operator '%+v' not supported [dialect=%s]", op, dialect)
}

func errUnsupportedBitwiseExpressionOperator(dialect string, op exp.BitwiseOperation) error {
	return errors.NewUnsupportedError("bitwise operator '%+v' not supported [dialect=%s]", op, dialect)
}

func errUnsupportedArithmeticExpressionOperator(dialect string, op exp.ArithmeticOperation) error {
	return errors.NewUnsupportedError("arithmetic operator '%+v' not supported [dialect=%s]", op, dialect)
}

func errUnsupportedRangeExpressionOperator(dialect string, op exp.RangeOperation) error {
	return errors.NewUnsupportedError("range operator '%+v' not supported [dialect=%s]", op, dialect)
}

func errBooleanDataTypeNotSupported(dialect string) error {
	return errors.NewUnsupportedError("boolean data type is not supported [dialect=%s]", dialect)
}

func errEmptyListFragmentMissing(dialect, fragment, list string) error {
	return errors.NewUnsupportedError("dialect has no %s for an empty %s list [dialect=%s]", fragment, list, dialect)
}

func errExcludedFragmentMissing(dialect, fragment string) error {
	return errors.NewUnsupportedError("dialect has no %s for Excluded [dialect=%s]", fragment, dialect)
}

func errUnknownExcludedStyle(dialect string, style ExcludedStyle) error {
	return errors.NewUnsupportedError("dialect has unknown ExcludedStyle %d [dialect=%s]", style, dialect)
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
	return errors.NewUnsupportedError("dialect does not support fsp %d, want one of %s [dialect=%s]", fsp, strings.Join(names, ", "), dialect)
}

func errUnknownTimeFunction(function exp.TimeFunction) error {
	return errors.NewInvalidExpressionError("unknown TimeFunction %d", function)
}

func errUnknownTimeStyle(dialect string, style TimeStyle) error {
	return errors.NewUnsupportedError("dialect has unknown TimeStyle %d [dialect=%s]", style, dialect)
}

func errIntervalUnit(unit exp.IntervalUnit) error {
	return errors.NewInvalidExpressionError("unknown Interval unit %q, want one of %s", unit, intervalUnitNames)
}

func errIntervalCount(dialect string, count, value any, unit exp.IntervalUnit, fractionalSeconds bool) error {
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

func errJSONFunctionNotSupported(dialect string, function exp.JSONFunction) error {
	name, ok := jsonFunctionNames[function]
	if !ok {
		return errUnknownJSONFunction(function)
	}
	return errors.NewUnsupportedError("dialect does not support %s [dialect=%s]", name, dialect)
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

func errValuerValue(v driver.Valuer, cause error) error {
	return errors.NewInvalidExpressionErrorWithCause(cause, "driver.Valuer %T", v)
}

func errLateralNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support lateral expressions [dialect=%s]", dialect)
}

var errExcludedRowAliasEmpty = errors.NewInvalidExpressionError("Excluded requires ExcludedRowAlias or an insert alias")

var errExcludedRowAliasQualified = errors.NewInvalidExpressionError("Excluded requires an unqualified insert alias")

var errConflictUpdateValuesRequired = errors.NewInvalidExpressionError("values are required for on conflict update expression")

var errNoSourceForInsert = errors.NewInvalidExpressionError("no source found when generating insert sql")

func errMismatchedRowLength(expectedL, actualL int) error {
	return errors.NewInvalidExpressionError("rows with different value length, expected %d got %d", expectedL, actualL)
}

func errUpsertWithWhereNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support upsert with where clause [dialect=%s]", dialect)
}

func errOnConflictNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support upsert [dialect=%s]", dialect)
}

func errExcludedRequiresInsertCols(dialect string) error {
	return errors.NewUnsupportedError("dialect requires Cols(...).FromQuery(...) to use Excluded with INSERT ... SELECT [dialect=%s]", dialect)
}

func errJoinTypeNotSupported(dialect string, j exp.JoinExpression) error {
	return errors.NewUnsupportedError("dialect does not support %v [dialect=%s]", j.JoinType(), dialect)
}

func errJoinConditionRequired(j exp.JoinExpression) error {
	return errors.NewInvalidExpressionError("join condition required for conditioned join %v", j.JoinType())
}

func errJoinUsingNotSupported(dialect string, j exp.JoinExpression) error {
	return errors.NewUnsupportedError("USING is not supported for %v, use an ON condition [dialect=%s]", j.JoinType(), dialect)
}

func errDistinctOnNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support DISTINCT ON clause [dialect=%s]", dialect)
}

func errWindowNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support WINDOW clause [dialect=%s]", dialect)
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

func errQualifyNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support QUALIFY clause [dialect=%s]", dialect)
}

var errNoWindowName = errors.NewInvalidExpressionError("window expression has no valid name")

var errNoSourceForTruncate = errors.NewInvalidExpressionError("no source found when generating truncate sql")

var errNoSourceForUpdate = errors.NewInvalidExpressionError("no source found when generating update sql")

var errNoSetValuesForUpdate = errors.NewInvalidExpressionError("no set values found when generating update sql")

func errOrderByOnUpdateNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support ORDER BY on UPDATE [dialect=%s]", dialect)
}

func errLimitOnUpdateNotSupported(dialect string) error {
	return errors.NewUnsupportedError("dialect does not support LIMIT on UPDATE [dialect=%s]", dialect)
}
