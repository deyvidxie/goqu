package scantype

import (
	"database/sql"
	"reflect"
	"time"

	"github.com/doug-martin/goqu/v9/internal/util"
)

var (
	timeType     = reflect.TypeFor[time.Time]()
	bytesType    = reflect.TypeFor[[]byte]()
	rawBytesType = reflect.TypeFor[sql.RawBytes]()
)

func Struct[T any](tagName string) error {
	t := reflect.TypeFor[T]()
	elem := util.Deref(t)
	if !util.IsStruct(elem.Kind()) {
		return NewUnsupportedScanTypeError("cannot scan rows into %s: requires a struct or a pointer to a struct", t)
	}
	switch problem := util.CheckColumnMap(elem, tagName); problem.Issue {
	case util.ColumnMapNonStructEmbed:
		return NewUnsupportedScanTypeError(
			"cannot scan rows into %s: embedded field %s is a pointer to non-struct %s; tag it `%s:\"-\"`",
			t, problem.Path, problem.Type, tagName,
		)
	case util.ColumnMapUnexportedPointerEmbed:
		return NewUnsupportedScanTypeError(
			"cannot scan rows into %s: embedded field %s is a pointer to unexported %s; embed it by value or tag it `%s:\"-\"`",
			t, problem.Path, problem.Type, tagName,
		)
	case util.ColumnMapCycle:
		return NewUnsupportedScanTypeError(
			"cannot scan rows into %s: field %s leads back to %s; tag it `%s:\"-\"`", t, problem.Path, problem.Type, tagName,
		)
	case util.ColumnMapOK:
	}
	oneColumn := isTime(elem) || util.IsScanner(elem)
	sqlValue := oneColumn && elem.PkgPath() == "database/sql"
	columns := util.StructColumnMap(elem, tagName)
	switch {
	case len(columns) > 0 && !sqlValue:
		return nil
	case oneColumn:
		return NewUnsupportedScanTypeError(
			"cannot scan rows into %s, which scans as one column; use ScanAllVals[%s] or ScanOneVal[%s]", t, elem, elem,
		)
	default:
		return NewUnsupportedScanTypeError("cannot scan rows into %s, which has no fields goqu can scan into", t)
	}
}

func AllVals[T any](tagName string) error {
	t := reflect.TypeFor[T]()
	if err := rejectRawBytes("ScanAllVals", t); err != nil {
		return err
	}
	if !util.IsPointer(t.Kind()) {
		return nil
	}
	switch {
	case isUnscannableSlice(derefAll(t)):
		return NewUnsupportedScanTypeError("ScanAllVals cannot scan into %s; use []byte or a sql.Scanner", t)
	case acceptsNull(t.Elem()):
		return nil
	case isPlainValue(t.Elem()):
		return NewUnsupportedScanTypeError(
			"ScanAllVals cannot scan into %s, whose element %s cannot hold NULL; use sql.Null[%s]", t, t.Elem(), t.Elem(),
		)
	case isRowStruct(t.Elem(), tagName):
		return NewUnsupportedScanTypeError(
			"ScanAllVals cannot scan into %s, whose element %s is a row struct; use ScanAllStructs[%s]", t, t.Elem(), t,
		)
	default:
		return NewUnsupportedScanTypeError("ScanAllVals cannot scan into %s; use a sql.Scanner", t)
	}
}

func OneVal[T any]() error {
	t := reflect.TypeFor[T]()
	if err := rejectRawBytes("ScanOneVal", t); err != nil {
		return err
	}
	inner := derefAll(t)
	switch {
	case isUnscannableSlice(inner):
		return NewUnsupportedScanTypeError("ScanOneVal cannot scan into %s; use []byte or a sql.Scanner", t)
	case t == inner && isByteSlice(t) && t != bytesType && !util.IsScanner(t):
		return NewUnsupportedScanTypeError("ScanOneVal cannot scan into %s; use *%s", t, t)
	default:
		return nil
	}
}

func acceptsNull(t reflect.Type) bool {
	return util.IsPointer(t.Kind()) || t == bytesType || (t.Kind() == reflect.Interface && t.NumMethod() == 0) ||
		util.IsScanner(t)
}

func isPlainValue(t reflect.Type) bool {
	k := t.Kind()
	return util.IsBool(k) || util.IsInt(k) || util.IsUint(k) || util.IsFloat(k) || util.IsString(k) ||
		isTime(t) || isByteSlice(t)
}

func isTime(t reflect.Type) bool {
	return util.IsStruct(t.Kind()) && t.ConvertibleTo(timeType)
}

func isRowStruct(t reflect.Type, tagName string) bool {
	return util.IsStruct(t.Kind()) &&
		(util.CheckColumnMap(t, tagName).Issue != util.ColumnMapOK || len(util.StructColumnMap(t, tagName)) > 0)
}

func isUnscannableSlice(t reflect.Type) bool {
	return util.IsSlice(t.Kind()) && !isByteSlice(t) && !util.IsScanner(t)
}

func isByteSlice(t reflect.Type) bool {
	return util.IsSlice(t.Kind()) && t.Elem().Kind() == reflect.Uint8
}

func derefAll(t reflect.Type) reflect.Type {
	for util.IsPointer(t.Kind()) {
		t = t.Elem()
	}
	return t
}

func rejectRawBytes(fn string, t reflect.Type) error {
	if derefAll(t) == rawBytesType {
		return NewUnsupportedScanTypeError("%s cannot return sql.RawBytes, which does not outlive the query; use []byte", fn)
	}
	return nil
}
