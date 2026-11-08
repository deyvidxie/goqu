package exp

import (
	"reflect"

	"github.com/doug-martin/goqu/v9/internal/errors"
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

func errRowsWithoutColumns(n int) error {
	return errors.NewInvalidExpressionError("%d insert rows have no columns, only one row of defaults can be inserted", n)
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
