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
	if err := requireValType[T]("ScanAllVals"); err != nil {
		return nil, err
	}
	var vals []T
	if err := s.ScanValsContext(ctx, &vals); err != nil {
		return nil, err
	}
	return vals, nil
}

func ScanOneVal[T any](ctx context.Context, s ValSource) (val T, found bool, err error) {
	if typeErr := requireValType[T]("ScanOneVal"); typeErr != nil {
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
		return NewUnsupportedScanTypeError("%s requires a struct or a pointer to a struct, got %s", fn, t)
	}
	if elem == timeType || reflect.PointerTo(elem).Implements(sqlScannerType) {
		return NewUnsupportedScanTypeError("%s requires a row struct, got %s, which scans as one column; use %s", fn, t, valFn)
	}
	return nil
}

func requireValType[T any](fn string) error {
	t := reflect.TypeFor[T]()
	if t.Kind() == reflect.Slice && t != bytesType && t != rawBytesType && !reflect.PointerTo(t).Implements(sqlScannerType) {
		return NewUnsupportedScanTypeError("%s requires a single-column value type, got %s", fn, t)
	}
	return nil
}
