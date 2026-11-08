package scantype

import (
	"reflect"

	"github.com/doug-martin/goqu/v9/internal/errors"
	"github.com/doug-martin/goqu/v9/internal/util"
)

func errNotStructTarget(t reflect.Type) error {
	return errors.NewUnsupportedScanTypeError("cannot scan rows into %s, a struct or a pointer to a struct is required", t)
}

func errNonStructEmbed(t reflect.Type, problem util.ColumnMapProblem, tagName string) error {
	return errors.NewUnsupportedScanTypeError(
		"cannot scan rows into %s: embedded field %s is a pointer to non-struct %s; tag it `%s:\"-\"`",
		t, problem.Path, problem.Type, tagName,
	)
}

func errUnexportedPointerEmbed(t reflect.Type, problem util.ColumnMapProblem, tagName string) error {
	return errors.NewUnsupportedScanTypeError(
		"cannot scan rows into %s: embedded field %s is a pointer to unexported %s; embed it by value or tag it `%s:\"-\"`",
		t, problem.Path, problem.Type, tagName,
	)
}

func errFieldCycle(t reflect.Type, problem util.ColumnMapProblem, tagName string) error {
	return errors.NewUnsupportedScanTypeError(
		"cannot scan rows into %s: field %s leads back to %s; tag it `%s:\"-\"`", t, problem.Path, problem.Type, tagName,
	)
}

func errOneColumnStruct(t, elem reflect.Type) error {
	return errors.NewUnsupportedScanTypeError(
		"cannot scan rows into %s, which scans as one column; use ScanVals/ScanVal or ScanAllVals[%s]/ScanOneVal[%s]", t, elem, elem,
	)
}

func errNoScannableFields(t reflect.Type) error {
	return errors.NewUnsupportedScanTypeError("cannot scan rows into %s, which has no fields goqu can scan into", t)
}

func errUnscannableSlice(fn string, t reflect.Type) error {
	return errors.NewUnsupportedScanTypeError("%s cannot scan into %s; use []byte or a sql.Scanner", fn, t)
}

func errElementNotNullable(t reflect.Type) error {
	return errors.NewUnsupportedScanTypeError(
		"ScanAllVals cannot scan into %s, whose element %s cannot hold NULL; use sql.Null[%s]", t, t.Elem(), t.Elem(),
	)
}

func errElementIsRowStruct(t reflect.Type) error {
	return errors.NewUnsupportedScanTypeError(
		"ScanAllVals cannot scan into %s, whose element %s is a row struct; use ScanAllStructs[%s]", t, t.Elem(), t,
	)
}

func errElementNotScannable(t reflect.Type) error {
	return errors.NewUnsupportedScanTypeError("ScanAllVals cannot scan into %s; use a sql.Scanner", t)
}

func errNamedByteSliceValue(t reflect.Type) error {
	return errors.NewUnsupportedScanTypeError("ScanOneVal cannot scan into %s; use *%s", t, t)
}

func errRawBytes(fn string) error {
	return errors.NewUnsupportedScanTypeError(
		"%s cannot return sql.RawBytes, which is only valid until the next row is read or the rows are closed; use []byte", fn,
	)
}
