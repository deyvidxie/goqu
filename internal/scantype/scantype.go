package scantype

import (
	"database/sql"
	"reflect"
	"time"

	"github.com/doug-martin/goqu/v9/internal/util"
)

var (
	timeType     = reflect.TypeFor[time.Time]()
	byteType     = reflect.TypeFor[byte]()
	bytesType    = reflect.TypeFor[[]byte]()
	rawBytesType = reflect.TypeFor[sql.RawBytes]()
)

func Struct[T any](tagName string) error {
	return StrictRowStruct(reflect.TypeFor[T](), tagName)
}

func StrictRowStruct(t reflect.Type, tagName string) error {
	return rowStruct(t, tagName, false)
}

func RowStruct(t reflect.Type, tagName string) error {
	return rowStruct(t, tagName, true)
}

func rowStruct(t reflect.Type, tagName string, allowUnexportedPointerEmbeds bool) error {
	elem := util.Deref(t)
	if !util.IsStruct(elem.Kind()) {
		return errNotStructTarget(t)
	}
	switch problem := util.CheckColumnMap(elem, tagName); problem.Issue {
	case util.ColumnMapNonStructEmbed:
		return errNonStructEmbed(t, problem, tagName)
	case util.ColumnMapUnexportedPointerEmbed:
		if !allowUnexportedPointerEmbeds {
			return errUnexportedPointerEmbed(t, problem, tagName)
		}
	case util.ColumnMapCycle:
		return errFieldCycle(t, problem, tagName)
	case util.ColumnMapOK:
	}
	oneColumn := isTime(elem) || util.IsScanner(elem)
	sqlValue := oneColumn && elem.PkgPath() == "database/sql"
	columns := util.StructColumnMap(elem, tagName)
	switch {
	case len(columns) > 0 && !sqlValue:
		return nil
	case oneColumn:
		return errOneColumnStruct(t, elem)
	default:
		return errNoScannableFields(t)
	}
}

func AllVals[T any](tagName string) error {
	t := reflect.TypeFor[T]()
	if err := scannableVal("ScanAllVals", "ScanAllStructs", t, tagName); err != nil {
		return err
	}
	if !util.IsPointer(t.Kind()) {
		return nil
	}
	switch {
	case acceptsNull(t.Elem()):
		return nil
	case isPlainValue(t.Elem()):
		return errElementNotNullable(t)
	default:
		return errNotScannable("ScanAllVals", t)
	}
}

func OneVal[T any](tagName string) error {
	t := reflect.TypeFor[T]()
	if err := scannableVal("ScanOneVal", "ScanOneStruct", t, tagName); err != nil {
		return err
	}
	if isByteSlice(t) && t != bytesType && !util.IsScanner(t) {
		return errNamedByteSliceValue(t)
	}
	return nil
}

func IterVal[T any](tagName string) error {
	return scannableVal("IterVals", "IterStructs", reflect.TypeFor[T](), tagName)
}

func scannableVal(fn, structsFn string, t reflect.Type, tagName string) error {
	if err := rejectRawBytes(fn, t); err != nil {
		return err
	}
	if isUnscannableSlice(util.DerefAll(t)) {
		return errUnscannableSlice(fn, t)
	}
	return driverValueTarget(fn, structsFn, t, tagName)
}

func driverValueTarget(fn, structsFn string, t reflect.Type, tagName string) error {
	inner := util.DerefAll(t)
	switch {
	case util.IsScanner(inner):
		return nil
	case isRowStruct(inner, tagName):
		return errRowStructValue(fn, structsFn, t, inner)
	case util.IsScannerInterface(inner), isNeverADriverValue(inner.Kind()):
		return errNotScannable(fn, t)
	default:
		return nil
	}
}

func isNeverADriverValue(k reflect.Kind) bool {
	return k == reflect.Map || k == reflect.Chan || k == reflect.Func || k == reflect.UnsafePointer
}

func acceptsNull(t reflect.Type) bool {
	return util.IsPointer(t.Kind()) || t == bytesType || (t.Kind() == reflect.Interface && t.NumMethod() == 0) ||
		util.IsScanner(t)
}

func isPlainValue(t reflect.Type) bool {
	k := t.Kind()
	return util.IsBool(k) || util.IsInt(k) || util.IsUint(k) || util.IsFloat(k) || util.IsString(k) ||
		isTime(t) || isByteSlice(t)
}

func isTime(t reflect.Type) bool {
	return util.IsStruct(t.Kind()) && t.ConvertibleTo(timeType)
}

func isRowStruct(t reflect.Type, tagName string) bool {
	return util.IsStruct(t.Kind()) &&
		(util.CheckColumnMap(t, tagName).Issue != util.ColumnMapOK || len(util.StructColumnMap(t, tagName)) > 0)
}

func isUnscannableSlice(t reflect.Type) bool {
	return util.IsSlice(t.Kind()) && !isByteSlice(t) && !util.IsScanner(t)
}

func isByteSlice(t reflect.Type) bool {
	return util.IsSlice(t.Kind()) && t.Elem() == byteType
}

func rejectRawBytes(fn string, t reflect.Type) error {
	if util.DerefAll(t) == rawBytesType {
		return errRawBytes(fn)
	}
	return nil
}
