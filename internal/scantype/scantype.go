package scantype

import (
	"database/sql"
	"reflect"
	"strings"
	"time"
	"uuid"

	"github.com/doug-martin/goqu/v9/internal/util"
)

var (
	timeType     = reflect.TypeFor[time.Time]()
	byteType     = reflect.TypeFor[byte]()
	rawBytesType = reflect.TypeFor[sql.RawBytes]()
	uuidType     = reflect.TypeFor[uuid.UUID]()
	anyType      = reflect.TypeFor[any]()
	sqlPkgPath   = rawBytesType.PkgPath()
)

func Struct[T any](tagName string) error {
	return StrictRowStruct(reflect.TypeFor[T](), tagName)
}

func StrictRowStruct(t reflect.Type, tagName string) error {
	_, _, err := rowStructColumns(t, tagName, false)
	return err
}

func RowStruct(t reflect.Type, tagName string) error {
	_, _, err := ScanRowStruct(t, tagName)
	return err
}

func ScanRowStruct(t reflect.Type, tagName string) (columns util.ColumnMap, unexportedPointerEmbeds bool, err error) {
	columns, problem, err := rowStructColumns(t, tagName, true)
	return columns, problem.Issue == util.ColumnMapUnexportedPointerEmbed, err
}

func rowStructColumns(
	t reflect.Type, tagName string, allowUnexportedPointerEmbeds bool,
) (util.ColumnMap, util.ColumnMapProblem, error) {
	elem := util.Deref(t)
	if !util.IsStruct(elem.Kind()) {
		return nil, util.ColumnMapProblem{}, errNotStructTarget(t)
	}
	columns, problem := util.CheckScanColumnMap(elem, tagName)
	switch problem.Issue {
	case util.ColumnMapNonStructEmbed:
		return nil, problem, errNonStructEmbed(t, problem, tagName)
	case util.ColumnMapUnexportedPointerEmbed:
		if !allowUnexportedPointerEmbeds {
			return nil, problem, errUnexportedPointerEmbed(t, problem, tagName)
		}
	case util.ColumnMapCycle:
		return nil, problem, errFieldCycle(t, problem, tagName)
	case util.ColumnMapOK:
	}
	oneColumn := isTime(elem) || util.IsScanner(elem)
	sqlValue := oneColumn && elem.PkgPath() == sqlPkgPath
	switch {
	case len(columns) > 0 && !sqlValue:
		if field, ok := rawBytesField(elem, columns); ok {
			return nil, problem, errRawBytesField(t, field)
		}
		return columns, problem, nil
	case oneColumn:
		return nil, problem, errOneColumnStruct(t, elem)
	default:
		return nil, problem, errNoScannableFields(t)
	}
}

type caller struct {
	fn, structsFn string
	generic       bool
}

func AllVals[T any](tagName string) error {
	return scannableVal(caller{"ScanAllVals", "ScanAllStructs", true}, reflect.TypeFor[T](), reflect.TypeFor[T](), tagName)
}

func OneVal[T any](tagName string) error {
	return scannableVal(caller{"ScanOneVal", "ScanOneStruct", true}, reflect.TypeFor[T](), reflect.TypeFor[T](), tagName)
}

func IterVal[T any](tagName string) error {
	return scannableVal(caller{"IterVals", "IterStructs", true}, reflect.TypeFor[T](), reflect.TypeFor[T](), tagName)
}

func Val(fn, structsFn string, t reflect.Type, tagName string) error {
	return scannableVal(caller{fn, structsFn, false}, t, t, tagName)
}

func scannableVal(c caller, shown, check reflect.Type, tagName string) error {
	inner := util.DerefAll(check)
	switch {
	case inner == rawBytesType:
		return errRawBytes(c.fn)
	case IsUnscannableSlice(inner):
		return errUnscannableSlice(c.fn, shown, inner)
	case isSQLNull(inner):
		return scannableVal(c, shown, inner.Field(0).Type, tagName)
	case util.IsScanner(inner):
		return nil
	case isRowStruct(inner, tagName):
		return errRowStructValue(c, shown, inner)
	case isUnassignable(inner):
		return errNotScannable(c.fn, shown, inner)
	default:
		return nil
	}
}

func rawBytesField(t reflect.Type, columns util.ColumnMap) (string, bool) {
	names, _ := columns.Cols()
	for _, name := range names {
		if col := columns[name]; util.DerefAll(col.GoType) == rawBytesType {
			return util.FieldPath(t, col.FieldIndex), true
		}
	}
	return "", false
}

func isUnassignable(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Chan, reflect.Func, reflect.UnsafePointer, reflect.Uintptr, reflect.Complex64, reflect.Complex128:
		return true
	case reflect.Array:
		return t != uuidType
	case reflect.Interface:
		return util.IsScannerInterface(t)
	case reflect.Struct:
		return t.PkgPath() == sqlPkgPath
	default:
		return false
	}
}

func isSQLNull(t reflect.Type) bool {
	return util.IsStruct(t.Kind()) && t.PkgPath() == sqlPkgPath && strings.HasPrefix(t.Name(), "Null[")
}

func isTime(t reflect.Type) bool {
	return util.IsStruct(t.Kind()) && t.ConvertibleTo(timeType)
}

func isRowStruct(t reflect.Type, tagName string) bool {
	return util.IsStruct(t.Kind()) && util.HasTaggedField(t, tagName)
}

func IsUnscannableSlice(t reflect.Type) bool {
	return util.IsSlice(t.Kind()) && t.Elem() != byteType && t.Elem() != anyType && !util.IsScanner(t)
}
