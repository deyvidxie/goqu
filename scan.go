package goqu

import (
	"context"
	"reflect"

	"github.com/doug-martin/goqu/v9/exec"
)

type (
	structSource interface {
		ScanStructsContext(ctx context.Context, i interface{}) error
		ScanStructContext(ctx context.Context, i interface{}) (bool, error)
	}
	valSource interface {
		ScanValsContext(ctx context.Context, i interface{}) error
		ScanValContext(ctx context.Context, i interface{}) (bool, error)
	}
)

var (
	_ structSource = (*SelectDataset)(nil)
	_ structSource = exec.QueryExecutor{}
	_ valSource    = (*SelectDataset)(nil)
	_ valSource    = exec.QueryExecutor{}
)

func ScanAll[T any](ctx context.Context, s structSource) ([]T, error) {
	if err := requireStructType[T]("ScanAll"); err != nil {
		return nil, err
	}
	var rows []T
	if err := s.ScanStructsContext(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func ScanOne[T any](ctx context.Context, s structSource) (row T, found bool, err error) {
	if typeErr := requireStructType[T]("ScanOne"); typeErr != nil {
		return row, false, typeErr
	}
	t := reflect.TypeFor[T]()
	if t.Kind() != reflect.Pointer {
		found, err = s.ScanStructContext(ctx, &row)
		return row, found, err
	}
	elem := reflect.New(t.Elem())
	if found, err = s.ScanStructContext(ctx, elem.Interface()); found {
		row, _ = elem.Interface().(T)
	}
	return row, found, err
}

func ScanAllVals[T any](ctx context.Context, s valSource) ([]T, error) {
	var vals []T
	if err := s.ScanValsContext(ctx, &vals); err != nil {
		return nil, err
	}
	return vals, nil
}

func ScanOneVal[T any](ctx context.Context, s valSource) (val T, found bool, err error) {
	found, err = s.ScanValContext(ctx, &val)
	return val, found, err
}

func requireStructType[T any](fn string) error {
	t := reflect.TypeFor[T]()
	elem := t
	if elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	if elem.Kind() != reflect.Struct {
		return NewUnsupportedScanTypeError("%s requires a struct or a pointer to a struct, got %s", fn, t)
	}
	return nil
}
