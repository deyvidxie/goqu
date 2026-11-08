package exp

import (
	"reflect"
	"sort"

	"github.com/doug-martin/goqu/v9/internal/errors"
	"github.com/doug-martin/goqu/v9/internal/util"
)

type (
	insert struct {
		from AppendableExpression
		cols ColumnListExpression
		vals [][]interface{}
	}
)

func NewInsertExpression(tagName string, rows ...interface{}) (insertExpression InsertExpression, err error) {
	switch len(rows) {
	case 0:
		return new(insert), nil
	case 1:
		val := reflect.ValueOf(rows[0])
		if val.Kind() == reflect.Slice {
			vals := make([]interface{}, 0, val.Len())
			for i := 0; i < val.Len(); i++ {
				vals = append(vals, val.Index(i).Interface())
			}
			return NewInsertExpression(tagName, vals...)
		}
		if ae, ok := rows[0].(AppendableExpression); ok && !util.IsNilOrNilPointer(ae) {
			return &insert{from: ae}, nil
		}
	}
	return newInsert(tagName, rows...)
}

func (i *insert) Expression() Expression {
	return i
}

func (i *insert) Clone() Expression {
	return i.clone()
}

func (i *insert) clone() *insert {
	return &insert{from: i.from, cols: i.cols, vals: i.vals}
}

func (i *insert) IsEmpty() bool {
	return i.from == nil && (i.cols == nil || i.cols.IsEmpty())
}

func (i *insert) IsInsertFrom() bool {
	return i.from != nil
}

func (i *insert) From() AppendableExpression {
	return i.from
}

func (i *insert) Cols() ColumnListExpression {
	return i.cols
}

func (i *insert) SetCols(cols ColumnListExpression) InsertExpression {
	ci := i.clone()
	ci.cols = cols
	return ci
}

func (i *insert) Vals() [][]interface{} {
	return i.vals
}

func (i *insert) SetVals(vals [][]interface{}) InsertExpression {
	ci := i.clone()
	ci.vals = cloneRows(vals)
	return ci
}

func newInsert(tagName string, rows ...interface{}) (insertExp InsertExpression, err error) {
	for _, row := range rows {
		if util.IsNilOrNilPointer(row) {
			return nil, errors.NewInvalidExpressionError("insert rows must not be nil, got %T", row)
		}
	}
	var mapKeys util.ValueSlice
	rowValue := reflect.Indirect(reflect.ValueOf(rows[0]))
	rowType := rowValue.Type()
	rowKind := rowValue.Kind()
	if rowKind == reflect.Struct {
		return createStructSliceInsert(tagName, rows...)
	}
	vals := make([][]interface{}, 0, len(rows))
	var columns ColumnListExpression
	for _, row := range rows {
		if rowType != reflect.Indirect(reflect.ValueOf(row)).Type() {
			return nil, errors.NewInvalidExpressionError(
				"rows must all be the same type, expected %+v got %+v",
				rowType,
				reflect.Indirect(reflect.ValueOf(row)).Type(),
			)
		}
		newRowValue := reflect.Indirect(reflect.ValueOf(row))
		switch rowKind {
		case reflect.Map:
			if columns == nil {
				mapKeys = util.ValueSlice(newRowValue.MapKeys())
				sort.Sort(mapKeys)
				colKeys := make([]interface{}, 0, len(mapKeys))
				for _, key := range mapKeys {
					colKeys = append(colKeys, key.Interface())
				}
				columns = NewColumnListExpression(nil, tagName, colKeys...)
			}
			newMapKeys := util.ValueSlice(newRowValue.MapKeys())
			if len(newMapKeys) != len(mapKeys) {
				return nil, errors.NewInvalidExpressionError("rows with different value length, expected %d got %d", len(mapKeys), len(newMapKeys))
			}
			if !mapKeys.Equal(newMapKeys) {
				return nil, errors.NewInvalidExpressionError("rows with different keys, expected %s got %s", mapKeys.String(), newMapKeys.String())
			}
			rowVals := make([]interface{}, 0, len(mapKeys))
			for _, key := range mapKeys {
				rowVals = append(rowVals, newRowValue.MapIndex(key).Interface())
			}
			vals = append(vals, rowVals)
		default:
			return nil, errors.NewInvalidExpressionError(
				"unsupported insert row type %T, a map, goqu.Record or struct is required",
				row,
			)
		}
	}
	return &insert{cols: columns, vals: vals}, nil
}

func createStructSliceInsert(tagName string, rows ...interface{}) (insertExp InsertExpression, err error) {
	rowValue := reflect.Indirect(reflect.ValueOf(rows[0]))
	rowType := rowValue.Type()
	recordRows := make([]interface{}, 0, len(rows))
	for _, row := range rows {
		if rowType != reflect.Indirect(reflect.ValueOf(row)).Type() {
			return nil, errors.NewInvalidExpressionError(
				"rows must all be the same type, expected %+v got %+v",
				rowType,
				reflect.Indirect(reflect.ValueOf(row)).Type(),
			)
		}
		record, err := NewRecordFromStruct(row, true, false, tagName)
		if err != nil {
			return nil, err
		}
		recordRows = append(recordRows, record)
	}
	return newInsert(tagName, recordRows...)
}
