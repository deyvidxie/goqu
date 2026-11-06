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
	elem := t
	if elem.Kind() == reflect.Ptr {
		elem = elem.Elem()
	}
	if elem.Kind() != reflect.Struct {
		return NewUnsupportedScanTypeError("cannot scan rows into %s: requires a struct or a pointer to a struct", t)
	}
	switch problem := util.CheckColumnMap(elem, tagName); problem.Issue {
	case util.ColumnMapNonStructEmbed:
		return NewUnsupportedScanTypeError(
			"cannot scan rows into %s: embedded field %s is a pointer to non-struct %s; tag it `%s:\"-\"`",
			t, problem.Field, problem.Type, tagName,
		)
	case util.ColumnMapUnexportedPointerEmbed:
		return NewUnsupportedScanTypeError(
			"cannot scan rows into %s: embedded field %s is a pointer to unexported %s; embed it by value or tag it `%s:\"-\"`",
			t, problem.Field, problem.Type, tagName,
		)
	case util.ColumnMapCycle:
		return NewUnsupportedScanTypeError(
			"cannot scan rows into %s: field %s leads back to %s; tag it `%s:\"-\"`", t, problem.Field, problem.Type, tagName,
		)
	case util.ColumnMapOK:
	}
	oneColumn := elem == timeType || util.IsScanner(elem)
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

func AllVals[T any]() error {
	t := reflect.TypeFor[T]()
	if err := rejectRawBytes("ScanAllVals", t); err != nil {
		return err
	}
	if t.Kind() != reflect.Ptr {
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
	case util.IsStruct(t.Elem().Kind()):
		return NewUnsupportedScanTypeError("ScanAllVals cannot scan into %s, which is a row struct; use ScanAllStructs[%s]", t, t)
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
		return NewUnsupportedScanTypeError("ScanOneVal cannot scan into slice type %s; use []byte or a sql.Scanner", inner)
	case t == inner && isByteSlice(t) && t != bytesType && !util.IsScanner(t):
		return NewUnsupportedScanTypeError("ScanOneVal cannot scan into %s; use *%s", t, t)
	default:
		return nil
	}
}

func acceptsNull(t reflect.Type) bool {
	return t.Kind() == reflect.Ptr || t == bytesType || (t.Kind() == reflect.Interface && t.NumMethod() == 0) ||
		util.IsScanner(t)
}

func isPlainValue(t reflect.Type) bool {
	k := t.Kind()
	return util.IsBool(k) || util.IsInt(k) || util.IsUint(k) || util.IsFloat(k) || util.IsString(k) ||
		(util.IsStruct(k) && t.ConvertibleTo(timeType)) || isByteSlice(t)
}

func isUnscannableSlice(t reflect.Type) bool {
	return t.Kind() == reflect.Slice && !isByteSlice(t) && !util.IsScanner(t)
}

func isByteSlice(t reflect.Type) bool {
	return t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8
}

func derefAll(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

func rejectRawBytes(fn string, t reflect.Type) error {
	if derefAll(t) == rawBytesType {
		return NewUnsupportedScanTypeError("%s cannot return sql.RawBytes, which is only valid until the next row; use []byte", fn)
	}
	return nil
}
