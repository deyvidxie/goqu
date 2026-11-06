package exec

import (
	"context"
	"database/sql"
	"reflect"
	"slices"
	"strings"
	"time"
)

var (
	sqlScannerType = reflect.TypeFor[sql.Scanner]()
	timeType       = reflect.TypeFor[time.Time]()
	bytesType      = reflect.TypeFor[[]byte]()
	rawBytesType   = reflect.TypeFor[sql.RawBytes]()
)

func (q QueryExecutor) ScanAllStructs[T any](ctx context.Context) ([]T, error) {
	if err := CheckStructScanType[T](); err != nil {
		return nil, err
	}
	var rows []T
	if err := q.ScanStructsContext(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (q QueryExecutor) ScanOneStruct[T any](ctx context.Context) (row T, found bool, err error) {
	if typeErr := CheckStructScanType[T](); typeErr != nil {
		return row, false, typeErr
	}
	t := reflect.TypeFor[T]()
	if t.Kind() != reflect.Pointer {
		found, err = q.ScanStructContext(ctx, &row)
		return row, found, err
	}
	elem := reflect.New(t.Elem())
	if found, err = q.ScanStructContext(ctx, elem.Interface()); found {
		reflect.ValueOf(&row).Elem().Set(elem)
	}
	return row, found, err
}

func (q QueryExecutor) ScanAllVals[T any](ctx context.Context) ([]T, error) {
	t := reflect.TypeFor[T]()
	if err := rejectRawBytes("ScanAllVals", t); err != nil {
		return nil, err
	}
	if t.Kind() == reflect.Pointer {
		switch {
		case isUnscannableSlice(derefAll(t)):
			return nil, NewUnsupportedScanTypeError("ScanAllVals cannot scan into %s", t)
		case acceptsNull(t.Elem()):
		case isPlainValue(t.Elem()):
			return nil, NewUnsupportedScanTypeError("ScanAllVals cannot scan into %s, which cannot hold NULL; use sql.Null[%s]", t, t.Elem())
		default:
			return nil, NewUnsupportedScanTypeError("ScanAllVals cannot scan into %s", t)
		}
	}
	var vals []T
	if err := q.ScanValsContext(ctx, &vals); err != nil {
		return nil, err
	}
	return vals, nil
}

func (q QueryExecutor) ScanOneVal[T any](ctx context.Context) (val T, found bool, err error) {
	if typeErr := requireOneValType[T](); typeErr != nil {
		return val, false, typeErr
	}
	found, err = q.ScanValContext(ctx, &val)
	return val, found, err
}

func CheckStructScanType[T any]() error {
	t := reflect.TypeFor[T]()
	elem := t
	if elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	if elem.Kind() != reflect.Struct {
		return NewUnsupportedScanTypeError("cannot scan rows into %s: requires a struct or a pointer to a struct", t)
	}
	if embedded := nonStructEmbeddedPointer(elem, map[reflect.Type]bool{}); embedded != nil {
		return NewUnsupportedScanTypeError("cannot scan rows into %s, which embeds %s", t, embedded)
	}
	oneColumn := elem == timeType || reflect.PointerTo(elem).Implements(sqlScannerType)
	sqlValue := oneColumn && elem.PkgPath() == "database/sql"
	if hasRowFields(elem, map[reflect.Type]bool{}) && !sqlValue {
		return nil
	}
	if oneColumn {
		return NewUnsupportedScanTypeError(
			"cannot scan rows into %s, which scans as one column; use ScanAllVals[%s] or ScanOneVal[%s]", t, elem, elem,
		)
	}
	return NewUnsupportedScanTypeError("cannot scan rows into %s, which has no fields goqu can scan into", t)
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

func derefAll(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

func tagSkips(tag reflect.StructTag) bool {
	for _, part := range strings.Fields(string(tag)) {
		if _, value, ok := strings.Cut(part, ":"); ok && slices.Contains(strings.Split(strings.Trim(value, `"`), ","), "-") {
			return true
		}
	}
	return false
}

func isByteSlice(t reflect.Type) bool {
	return t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8
}

func nonStructEmbeddedPointer(t reflect.Type, seen map[reflect.Type]bool) reflect.Type {
	if seen[t] {
		return nil
	}
	seen[t] = true
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.Anonymous || tagSkips(f.Tag) {
			continue
		}
		ft := f.Type
		if ft.Kind() == reflect.Pointer {
			if ft.Elem().Kind() != reflect.Struct {
				return f.Type
			}
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Struct {
			if embedded := nonStructEmbeddedPointer(ft, seen); embedded != nil {
				return embedded
			}
		}
	}
	return nil
}

func hasRowFields(t reflect.Type, seen map[reflect.Type]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		ft := f.Type
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		if f.Anonymous && ft.Kind() == reflect.Struct {
			if (f.Type.Kind() != reflect.Pointer || f.IsExported()) && hasRowFields(ft, seen) {
				return true
			}
			continue
		}
		if f.IsExported() {
			return true
		}
	}
	return false
}

func requireOneValType[T any]() error {
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

func rejectRawBytes(fn string, t reflect.Type) error {
	if derefAll(t) == rawBytesType {
		return NewUnsupportedScanTypeError("%s cannot return sql.RawBytes, which is only valid until the next row; use []byte", fn)
	}
	return nil
}
