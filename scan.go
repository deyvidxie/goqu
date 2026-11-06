package goqu

import (
	"context"
	"database/sql"
	"reflect"
	"time"

	"github.com/doug-martin/goqu/v9/exec"
)

type (
	StructSource interface {
		ScanStructsContext(ctx context.Context, i interface{}) error
		ScanStructContext(ctx context.Context, i interface{}) (bool, error)
	}
	ValSource interface {
		ScanValsContext(ctx context.Context, i interface{}) error
		ScanValContext(ctx context.Context, i interface{}) (bool, error)
	}
)

var (
	_ StructSource = (*SelectDataset)(nil)
	_ StructSource = exec.QueryExecutor{}
	_ ValSource    = (*SelectDataset)(nil)
	_ ValSource    = exec.QueryExecutor{}

	sqlScannerType = reflect.TypeFor[sql.Scanner]()
	timeType       = reflect.TypeFor[time.Time]()
	bytesType      = reflect.TypeFor[[]byte]()
	rawBytesType   = reflect.TypeFor[sql.RawBytes]()
)

func ScanAllStructs[T any](ctx context.Context, s StructSource) ([]T, error) {
	if err := requireRowType[T]("ScanAllStructs", "ScanAllVals"); err != nil {
		return nil, err
	}
	var rows []T
	if err := s.ScanStructsContext(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func ScanOneStruct[T any](ctx context.Context, s StructSource) (row T, found bool, err error) {
	if typeErr := requireRowType[T]("ScanOneStruct", "ScanOneVal"); typeErr != nil {
		return row, false, typeErr
	}
	t := reflect.TypeFor[T]()
	if t.Kind() != reflect.Pointer {
		found, err = s.ScanStructContext(ctx, &row)
		return row, found, err
	}
	elem := reflect.New(t.Elem())
	if found, err = s.ScanStructContext(ctx, elem.Interface()); found {
		reflect.ValueOf(&row).Elem().Set(elem)
	}
	return row, found, err
}

func ScanAllVals[T any](ctx context.Context, s ValSource) ([]T, error) {
	t := reflect.TypeFor[T]()
	if err := rejectRawBytes("ScanAllVals", t); err != nil {
		return nil, err
	}
	if t.Kind() == reflect.Pointer && !acceptsNull(t.Elem()) {
		return nil, exec.NewUnsupportedScanTypeError("ScanAllVals cannot scan NULL into %s; use sql.Null[%s]", t, t.Elem())
	}
	var vals []T
	if err := s.ScanValsContext(ctx, &vals); err != nil {
		return nil, err
	}
	return vals, nil
}

func ScanOneVal[T any](ctx context.Context, s ValSource) (val T, found bool, err error) {
	if typeErr := requireOneValType[T](); typeErr != nil {
		return val, false, typeErr
	}
	found, err = s.ScanValContext(ctx, &val)
	return val, found, err
}

func requireRowType[T any](fn, valFn string) error {
	t := reflect.TypeFor[T]()
	elem := t
	if elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	if elem.Kind() != reflect.Struct {
		return exec.NewUnsupportedScanTypeError("%s requires a struct or a pointer to a struct, got %s", fn, t)
	}
	isSQLType := elem.PkgPath() == "database/sql"
	if hasRowFields(elem, map[reflect.Type]bool{}) && !isSQLType {
		return nil
	}
	if isSQLType || elem == timeType || reflect.PointerTo(elem).Implements(sqlScannerType) {
		return exec.NewUnsupportedScanTypeError("%s requires a row struct, got %s, which scans as one column; use %s[%s]", fn, t, valFn, elem)
	}
	return exec.NewUnsupportedScanTypeError("%s requires a row struct, got %s, which has no exported fields", fn, t)
}

func acceptsNull(t reflect.Type) bool {
	return t == bytesType || (t.Kind() == reflect.Interface && t.NumMethod() == 0) || reflect.PointerTo(t).Implements(sqlScannerType)
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
			if f.IsExported() && hasRowFields(ft, seen) {
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
	elem := t
	if elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	if elem.Kind() == reflect.Slice && elem != bytesType && !reflect.PointerTo(elem).Implements(sqlScannerType) {
		return exec.NewUnsupportedScanTypeError("ScanOneVal requires a single-column value type, got %s", t)
	}
	return nil
}

func rejectRawBytes(fn string, t reflect.Type) error {
	if t == rawBytesType || t == reflect.PointerTo(rawBytesType) {
		return exec.NewUnsupportedScanTypeError("%s cannot return sql.RawBytes, which is only valid until the next row; use []byte", fn)
	}
	return nil
}
