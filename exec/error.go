package exec

import (
	"reflect"

	"github.com/doug-martin/goqu/v9/internal/errors"
)

func errNilScanTarget(target interface{}) error {
	return errors.NewUnsupportedScanTypeError("scan target must not be nil, got %T", target)
}

func errScanStructTarget(target interface{}) error {
	return errors.NewUnsupportedScanTypeError("cannot scan into %T, a pointer to a struct is required", target)
}

func errScanSliceTarget(target interface{}) error {
	return errors.NewUnsupportedScanTypeError("cannot scan into %T, a pointer to a slice is required", target)
}

func errScanValTarget(target interface{}) error {
	return errors.NewUnsupportedScanTypeError("cannot scan into %T, a pointer is required", target)
}

func errScanValSliceTarget(target interface{}) error {
	return errors.NewUnsupportedScanTypeError("cannot scan one value into %T, use ScanVals for a slice", target)
}

func errScannerFieldNeedsValue(col string, t reflect.Type) error {
	return errors.NewUnsupportedScanTypeError(
		`unable to scan column %q into %s field: it must hold a non-nil pointer to a value that `+
			`implements sql.Scanner (a row goqu allocates cannot do this)`,
		col, t,
	)
}

func errNoFieldForColumn(col string) error {
	return errors.NewColumnMismatchError("unable to find corresponding field to column %q returned by query", col)
}
