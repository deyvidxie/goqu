package scantype

import (
	"fmt"
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
		"cannot scan rows into %s, which scans as one column; use ScanVals/ScanVal, ScanAllVals[%s]/ScanOneVal[%s] or IterVals[%s]",
		t, elem, elem, elem,
	)
}

func errNoScannableFields(t reflect.Type) error {
	return errors.NewUnsupportedScanTypeError("cannot scan rows into %s, which has no fields goqu can scan into", t)
}

func errUnscannableSlice(fn string, shown, inner reflect.Type) error {
	return errors.NewUnsupportedScanTypeError(
		"%s cannot scan into %s; use []byte or a concrete sql.Scanner", fn, scanTarget(shown, inner),
	)
}

func scanTarget(shown, inner reflect.Type) string {
	if util.DerefAll(shown) == inner {
		return shown.String()
	}
	return fmt.Sprintf("the %s of %s", inner, shown)
}

func errNotScannable(fn string, shown, inner reflect.Type) error {
	return errors.NewUnsupportedScanTypeError("%s cannot scan into %s; use a concrete sql.Scanner", fn, scanTarget(shown, inner))
}

func errRowStructValue(c caller, t, row reflect.Type) error {
	if !c.generic {
		return errors.NewUnsupportedScanTypeError("%s cannot scan into %s: %s is a row struct; use %s", c.fn, t, row, c.structsFn)
	}
	structsT := t
	if util.Deref(t) != row {
		structsT = reflect.PointerTo(row)
	}
	return errors.NewUnsupportedScanTypeError(
		"%s cannot scan into %s: %s is a row struct; use %s[%s]", c.fn, t, row, c.structsFn, structsT,
	)
}

func errRawBytes(fn string) error {
	return errors.NewUnsupportedScanTypeError(
		"%s cannot return sql.RawBytes, which is only valid until the next row is read or the rows are closed; use []byte", fn,
	)
}

func errRawBytesField(t reflect.Type, field string) error {
	return errors.NewUnsupportedScanTypeError(
		"cannot scan rows into %s: field %s is a sql.RawBytes, which is only valid until the next row is read or the rows are closed; use []byte",
		t, field,
	)
}
