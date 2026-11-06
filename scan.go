package goqu

import "context"

type (
	StructScanner interface {
		ScanStructsContext(ctx context.Context, i interface{}) error
		ScanStructContext(ctx context.Context, i interface{}) (bool, error)
	}
	ValScanner interface {
		ScanValsContext(ctx context.Context, i interface{}) error
		ScanValContext(ctx context.Context, i interface{}) (bool, error)
	}
)

func ScanAll[T any](ctx context.Context, s StructScanner) ([]T, error) {
	var rows []T
	if err := s.ScanStructsContext(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func ScanOne[T any](ctx context.Context, s StructScanner) (row T, found bool, err error) {
	found, err = s.ScanStructContext(ctx, &row)
	return row, found, err
}

func ScanVals[T any](ctx context.Context, s ValScanner) ([]T, error) {
	var vals []T
	if err := s.ScanValsContext(ctx, &vals); err != nil {
		return nil, err
	}
	return vals, nil
}

func ScanVal[T any](ctx context.Context, s ValScanner) (val T, found bool, err error) {
	found, err = s.ScanValContext(ctx, &val)
	return val, found, err
}
