package exec

import (
	"reflect"

	"github.com/doug-martin/goqu/v9/internal/errors"
)

func errScanStructTarget(i interface{}) error {
	return errors.NewUnsupportedScanTypeError("cannot scan into %T, a pointer to a struct is required", i)
}

func errScanSliceTarget(i interface{}) error {
	return errors.NewUnsupportedScanTypeError("cannot scan into %T, a pointer to a slice is required", i)
}

func errScanValTarget(i interface{}) error {
	return errors.NewUnsupportedScanTypeError("cannot scan into %T, a pointer is required", i)
}

func errScanValSliceTarget(i interface{}) error {
	return errors.NewUnsupportedScanTypeError("cannot scan one value into %T, use ScanVals for a slice", i)
}

func errScannerFieldNeedsValue(col string, t reflect.Type) error {
	return errors.NewUnsupportedScanTypeError(
		`unable to scan column %q into %s field: it must hold a non-nil pointer to a value that `+
			`implements sql.Scanner (ScanStructs cannot do this)`,
		col, t,
	)
}

func errNoFieldForColumn(col string) error {
	return errors.NewUnsupportedScanTypeError("unable to find corresponding field to column %q returned by query", col)
}
