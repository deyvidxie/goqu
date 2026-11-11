package exp

import (
	"reflect"

	"github.com/doug-martin/goqu/v9/internal/errors"
	"github.com/doug-martin/goqu/v9/internal/util"
)

func errNilInsertRow(row interface{}) error {
	return errors.NewInvalidExpressionError("insert rows must not be nil, got %T", row)
}

func errMixedRowTypes(expected, actual reflect.Type) error {
	return errors.NewInvalidExpressionError("rows must all be the same type, expected %s got %s", expected, actual)
}

func errRowLengthMismatch(expected, actual int) error {
	return errors.NewInvalidExpressionError("rows with different value length, expected %d got %d", expected, actual)
}

var errNilOrder = errors.NewInvalidExpressionError("Order expressions must not be nil")

var errRowsWithoutColumns = errors.NewInvalidExpressionError("insert rows have no columns")

func errEmptyAppend(method, noun string) error {
	return errors.NewInvalidExpressionError("%s requires at least one %s", method, noun)
}

func errRowKeysMismatch(expected, actual string) error {
	return errors.NewInvalidExpressionError("rows with different keys, expected %s got %s", expected, actual)
}

func errUnsupportedInsertRowType(row interface{}) error {
	return errors.NewInvalidExpressionError("unsupported insert row type %T, a map, goqu.Record or struct is required", row)
}

func errNilUpdateValues(update interface{}) error {
	return errors.NewInvalidExpressionError("update values must not be nil, got %T", update)
}

func errUnsupportedUpdateValueType(update interface{}) error {
	return errors.NewInvalidExpressionError("unsupported update value type %T, a map, goqu.Record or struct is required", update)
}

func errNilRecordSource(source interface{}) error {
	return errors.NewInvalidExpressionError("record source must not be nil, got %T", source)
}

func errRecordNotStruct(source interface{}) error {
	return errors.NewInvalidExpressionError("cannot create a record from %T, a struct is required", source)
}

func errUnsupportedOpKey(key string) error {
	return errors.NewInvalidExpressionError("unsupported Op key %q", key)
}

func errEmptyOp(key string) error {
	return errors.NewInvalidExpressionError("Op for %q has no operators", key)
}

func errColumnSource(t reflect.Type, problem util.ColumnMapProblem, tagName string) error {
	if problem.Issue == util.ColumnMapCycle {
		return errors.NewInvalidExpressionError(
			"cannot derive columns from %s: field %s leads back to %s; tag it `%s:\"-\"`", t, problem.Path, problem.Type, tagName,
		)
	}
	return errors.NewInvalidExpressionError(
		"cannot derive columns from %s: embedded field %s is a pointer to non-struct %s; tag it `%s:\"-\"`",
		t, problem.Path, problem.Type, tagName,
	)
}

func errRangeOpValue(key string, val interface{}) error {
	return errors.NewInvalidExpressionError("Op %q takes a goqu.Range value, got %T", key, val)
}
