package exec

import (
	"database/sql"
	"reflect"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/scantype"
	"github.com/doug-martin/goqu/v9/internal/util"
)

type (
	// Scanner knows how to scan sql.Rows into structs.
	Scanner interface {
		Next() bool
		ScanStruct(i interface{}) error
		ScanStructs(i interface{}) error
		ScanVal(i interface{}) error
		ScanVals(i interface{}) error
		Close() error
		Err() error
	}

	scanner struct {
		rows      *sql.Rows
		columnMap util.ColumnMap
		columns   []string
		tagName   string
	}
)

// NewScanner returns a scanner that can be used for scanning rows into structs.
func NewScanner(rows *sql.Rows, tagName string) Scanner {
	return &scanner{rows: rows, tagName: tagName}
}

// Next prepares the next row for Scanning. See sql.Rows#Next for more
// information.
func (s *scanner) Next() bool {
	return s.rows.Next()
}

// Err returns the error, if any that was encountered during iteration. See
// sql.Rows#Err for more information.
func (s *scanner) Err() error {
	return s.rows.Err()
}

// ScanStruct will scan the current row into i.
func (s *scanner) ScanStruct(i interface{}) error {
	// Setup columnMap and columns, but only once.
	if s.columnMap == nil || s.columns == nil {
		if t := reflect.TypeOf(i); t != nil {
			if err := scantype.RowStruct(t, s.tagName); err != nil {
				return err
			}
		}
		cm, err := util.GetColumnMap(i, s.tagName)
		if err != nil {
			return err
		}

		cols, err := s.rows.Columns()
		if err != nil {
			return err
		}

		s.columnMap = cm
		s.columns = cols
	}

	scans := make([]interface{}, 0, len(s.columns))
	// columns that were scanned straight into a value that is already set on the struct (allocated when needed)
	var scannedInPlace []bool
	for index, col := range s.columns {
		data, ok := s.columnMap[col]
		if !ok {
			return errNoFieldForColumn(col)
		}
		if data.GoType.Kind() == reflect.Interface && data.GoType.NumMethod() > 0 {
			// The column map is cached per struct type so it only knows the interface, not the concrete type.
			// Scan into the value the caller already put in the field (e.g. &sql.NullString{}). Empty interfaces
			// (interface{}) keep receiving the driver value.
			if target, ok := util.InterfaceScanTarget(i, data.FieldIndex); ok {
				if scannedInPlace == nil {
					scannedInPlace = make([]bool, len(s.columns))
				}
				scans = append(scans, target)
				scannedInPlace[index] = true
				continue
			}
			if util.IsScannerInterface(data.GoType) {
				return errScannerFieldNeedsValue(col, data.GoType)
			}
		}
		scans = append(scans, reflect.New(data.GoType).Interface())
	}

	if err := s.rows.Scan(scans...); err != nil {
		return err
	}

	record := exp.Record{}
	for index, col := range s.columns {
		if len(scannedInPlace) == 0 || !scannedInPlace[index] {
			record[col] = scans[index]
		}
	}

	util.AssignStructVals(i, record, s.columnMap)

	return s.Err()
}

// ScanStructs scans results in slice of structs
func (s *scanner) ScanStructs(i interface{}) error {
	val, err := checkScanStructsTarget(i, s.tagName)
	if err != nil {
		return err
	}
	return s.scanIntoSlice(val, func(i interface{}) error {
		return s.ScanStruct(i)
	})
}

// ScanVal will scan the current row and column into i.
func (s *scanner) ScanVal(i interface{}) error {
	if err := s.rows.Scan(i); err != nil {
		return err
	}

	return s.Err()
}

// ScanStructs scans results in slice of values
func (s *scanner) ScanVals(i interface{}) error {
	val, err := checkScanValsTarget(i, s.tagName)
	if err != nil {
		return err
	}
	elemType := val.Type().Elem()
	scanType := elemType
	if namedPointer := util.IsPointer(elemType.Kind()) && elemType.Name() != ""; namedPointer {
		scanType = elemType.Elem()
	}
	for s.Next() {
		row := reflect.New(scanType)
		if err := s.ScanVal(row.Interface()); err != nil {
			return err
		}
		if scanType == elemType {
			row = row.Elem()
		}
		val.Set(reflect.Append(val, row.Convert(elemType)))
	}
	return s.Err()
}

// Close closes the Rows, preventing further enumeration. See sql.Rows#Close
// for more info.
func (s *scanner) Close() error {
	return s.rows.Close()
}

func (s *scanner) scanIntoSlice(val reflect.Value, it func(i interface{}) error) error {
	elemType := util.GetSliceElementType(val)

	for s.Next() {
		row := reflect.New(elemType)
		if rowErr := it(row.Interface()); rowErr != nil {
			return rowErr
		}
		util.AppendSliceElement(val, row)
	}

	return s.Err()
}

func checkScanStructsTarget(i interface{}, tagName string) (reflect.Value, error) {
	val := reflect.ValueOf(i)
	if util.IsNilOrNilPointer(i) {
		return val, errNilScanTarget(i)
	}
	if !util.IsPointer(val.Kind()) {
		return val, errScanSliceTarget(i)
	}
	val = reflect.Indirect(val)
	if !util.IsSlice(val.Kind()) {
		return val, errScanSliceTarget(i)
	}
	return val, scantype.RowStruct(val.Type().Elem(), tagName)
}

func checkScanValsTarget(i interface{}, tagName string) (reflect.Value, error) {
	val := reflect.ValueOf(i)
	if util.IsNilOrNilPointer(i) {
		return val, errNilScanTarget(i)
	}
	if !util.IsPointer(val.Kind()) {
		return val, errScanSliceTarget(i)
	}
	val = reflect.Indirect(val)
	if !util.IsSlice(val.Kind()) {
		return val, errScanSliceTarget(i)
	}
	return val, scantype.Val("ScanVals", "ScanStructs", val.Type().Elem(), tagName)
}
