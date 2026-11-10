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

		unexportedPointerEmbeds bool
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

func (s *scanner) ScanStruct(i interface{}) error {
	if util.IsNilOrNilPointer(i) {
		return errNilScanTarget(i)
	}
	if !util.IsPointer(reflect.TypeOf(i).Kind()) {
		return errScanStructTarget(i)
	}
	return s.scanStruct(i, "embed it by value, set it before ScanStruct or tag it")
}

func (s *scanner) scanStruct(i interface{}, nilEmbedRemedy string) error {
	if s.columnMap == nil || s.columns == nil {
		cm, unexportedPointerEmbeds, err := scantype.ScanRowStruct(reflect.TypeOf(i), s.tagName)
		if err != nil {
			return err
		}
		cols, err := s.rows.Columns()
		if err != nil {
			return err
		}
		s.columnMap = cm
		s.columns = cols
		s.unexportedPointerEmbeds = unexportedPointerEmbeds
	}

	scans := make([]interface{}, 0, len(s.columns))
	var scannedInPlace []bool
	for index, col := range s.columns {
		data, ok := s.columnMap[col]
		if !ok {
			return errNoFieldForColumn(col)
		}
		if s.unexportedPointerEmbeds {
			if field, embedded, found := util.NilUnexportedEmbed(reflect.ValueOf(i), data.FieldIndex); found {
				return errNilUnexportedEmbed(col, field, embedded, nilEmbedRemedy, s.tagName)
			}
		}
		if data.GoType.Kind() == reflect.Interface && data.GoType.NumMethod() > 0 {
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
		scans = append(scans, reflect.New(util.ScanType(data.GoType)).Interface())
	}

	if err := s.rows.Scan(scans...); err != nil {
		return err
	}

	record := exp.Record{}
	for index, col := range s.columns {
		if len(scannedInPlace) == 0 || !scannedInPlace[index] {
			record[col] = fieldValue(scans[index], s.columnMap[col].GoType)
		}
	}

	util.AssignStructVals(i, record, s.columnMap)

	return s.Err()
}

func (s *scanner) ScanStructs(i interface{}) error {
	val, err := checkScanStructsTarget(i, s.tagName)
	if err != nil {
		return err
	}
	elemType := util.GetSliceElementType(val)
	for s.Next() {
		row := reflect.New(elemType)
		if err := s.scanStruct(row.Interface(), "embed it by value or tag it"); err != nil {
			return err
		}
		util.AppendSliceElement(val, row)
	}
	return s.Err()
}

func (s *scanner) ScanVal(i interface{}) error {
	if util.IsNilOrNilPointer(i) {
		return errNilScanTarget(i)
	}
	t := reflect.TypeOf(i)
	if !util.IsPointer(t.Kind()) {
		return errScanValTarget(i)
	}
	if scanType := util.ScanType(t); scanType != t {
		row := reflect.New(scanType.Elem())
		if err := s.rows.Scan(row.Interface()); err != nil {
			return err
		}
		util.AssignScanned(reflect.ValueOf(i).Elem(), row.Elem())
		return s.Err()
	}
	if err := s.rows.Scan(i); err != nil {
		return err
	}

	return s.Err()
}

func (s *scanner) ScanVals(i interface{}) error {
	val, err := checkScanValsTarget(i, s.tagName)
	if err != nil {
		return err
	}
	elemType := val.Type().Elem()
	for s.Next() {
		row := reflect.New(elemType)
		if err := s.ScanVal(row.Interface()); err != nil {
			return err
		}
		val.Set(reflect.Append(val, row.Elem()))
	}
	return s.Err()
}

// Close closes the Rows, preventing further enumeration. See sql.Rows#Close
// for more info.
func (s *scanner) Close() error {
	return s.rows.Close()
}

func fieldValue(scanned interface{}, fieldType reflect.Type) interface{} {
	v := reflect.ValueOf(scanned)
	if v.Type().Elem() == fieldType {
		return scanned
	}
	field := reflect.New(fieldType)
	util.AssignScanned(field.Elem(), v.Elem())
	return field.Interface()
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
