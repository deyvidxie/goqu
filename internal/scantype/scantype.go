package scantype

import (
	"database/sql"
	"reflect"
	"time"

	"github.com/doug-martin/goqu/v9/internal/util"
)

var (
	sqlScannerType = reflect.TypeFor[sql.Scanner]()
	timeType       = reflect.TypeFor[time.Time]()
	bytesType      = reflect.TypeFor[[]byte]()
	rawBytesType   = reflect.TypeFor[sql.RawBytes]()
)

func Struct[T any](tagName string) error {
	t := reflect.TypeFor[T]()
	elem := t
	if elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	if elem.Kind() != reflect.Struct {
		return NewUnsupportedScanTypeError("cannot scan rows into %s: requires a struct or a pointer to a struct", t)
	}
	switch issue, culprit := util.CheckColumnMap(elem, tagName); issue {
	case util.ColumnMapNonStructEmbed, util.ColumnMapUnexportedPointerEmbed:
		return NewUnsupportedScanTypeError("cannot scan rows into %s, which embeds %s", t, culprit)
	case util.ColumnMapCycle:
		return NewUnsupportedScanTypeError("cannot scan rows into %s, which contains itself through %s", t, culprit)
	case util.ColumnMapOK:
	}
	oneColumn := elem == timeType || reflect.PointerTo(elem).Implements(sqlScannerType)
	sqlValue := oneColumn && elem.PkgPath() == "database/sql"
	columns, err := util.GetColumnMap(reflect.New(elem).Interface(), tagName)
	if err != nil {
		return NewUnsupportedScanTypeError("cannot scan rows into %s: %s", t, err)
	}
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
	if t.Kind() != reflect.Pointer {
		return nil
	}
	switch {
	case isUnscannableSlice(derefAll(t)):
		return NewUnsupportedScanTypeError("ScanAllVals cannot scan into %s", t)
	case acceptsNull(t.Elem()):
		return nil
	case isPlainValue(t.Elem()):
		return NewUnsupportedScanTypeError("ScanAllVals cannot scan into %s, which cannot hold NULL; use sql.Null[%s]", t, t.Elem())
	default:
		return NewUnsupportedScanTypeError("ScanAllVals cannot scan into %s", t)
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
	case t == inner && isByteSlice(t) && t != bytesType && !reflect.PointerTo(t).Implements(sqlScannerType):
		return NewUnsupportedScanTypeError("ScanOneVal cannot scan into %s; use *%s", t, t)
	default:
		return nil
	}
}

func acceptsNull(t reflect.Type) bool {
	return t.Kind() == reflect.Pointer || t == bytesType || (t.Kind() == reflect.Interface && t.NumMethod() == 0) ||
		reflect.PointerTo(t).Implements(sqlScannerType)
}

func isPlainValue(t reflect.Type) bool {
	k := t.Kind()
	return (k >= reflect.Bool && k <= reflect.Float64 && k != reflect.Uintptr) || k == reflect.String ||
		(k == reflect.Struct && t.ConvertibleTo(timeType)) || isByteSlice(t)
}

func isUnscannableSlice(t reflect.Type) bool {
	return t.Kind() == reflect.Slice && !isByteSlice(t) && !reflect.PointerTo(t).Implements(sqlScannerType)
}

func isByteSlice(t reflect.Type) bool {
	return t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8
}

func derefAll(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
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
