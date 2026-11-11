package util

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
)

const (
	skipUpdateTagName     = "skipupdate"
	skipInsertTagName     = "skipinsert"
	defaultIfEmptyTagName = "defaultifempty"
	omitNilTagName        = "omitnil"
	omitEmptyTagName      = "omitempty"
)

var (
	valuerType     = reflect.TypeFor[driver.Valuer]()
	scannerType    = reflect.TypeFor[sql.Scanner]()
	bytesType      = reflect.TypeFor[[]byte]()
	rawMessageType = reflect.TypeFor[json.RawMessage]()
)

func IsValuerOrBytes(v any) bool {
	_, ok := v.(driver.Valuer)
	return ok || IsBytesValue(v)
}

func IsBytesValue(v any) bool {
	switch v.(type) {
	case []byte, *[]byte, json.RawMessage, *json.RawMessage:
		return true
	default:
		return false
	}
}

func IsListValue(v any) bool {
	rv := reflect.Indirect(reflect.ValueOf(v))
	if !rv.IsValid() {
		return false
	}
	if _, valuer := v.(driver.Valuer); valuer {
		return false
	}
	return IsListType(rv.Type())
}

func IsListType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Slice:
		return t != bytesType && t != rawMessageType
	case reflect.Array:
		return t.Elem().Kind() != reflect.Uint8
	default:
		return false
	}
}

func IsUint(k reflect.Kind) bool {
	return (k == reflect.Uint) ||
		(k == reflect.Uint8) ||
		(k == reflect.Uint16) ||
		(k == reflect.Uint32) ||
		(k == reflect.Uint64)
}

func IsInt(k reflect.Kind) bool {
	return (k == reflect.Int) ||
		(k == reflect.Int8) ||
		(k == reflect.Int16) ||
		(k == reflect.Int32) ||
		(k == reflect.Int64)
}

func IsFloat(k reflect.Kind) bool {
	return (k == reflect.Float32) ||
		(k == reflect.Float64)
}

func IsString(k reflect.Kind) bool {
	return k == reflect.String
}

func IsBool(k reflect.Kind) bool {
	return k == reflect.Bool
}

func IsSlice(k reflect.Kind) bool {
	return k == reflect.Slice
}

func IsStruct(k reflect.Kind) bool {
	return k == reflect.Struct
}

func IsInvalid(k reflect.Kind) bool {
	return k == reflect.Invalid
}

func IsPointer(k reflect.Kind) bool {
	return k == reflect.Ptr
}

func IsNilOrNilPointer(v interface{}) bool {
	rv := reflect.ValueOf(v)
	return !rv.IsValid() || (rv.Kind() == reflect.Ptr && rv.IsNil())
}

func IsNil(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func:
		return v.IsNil()
	default:
		return false
	}
}

func IsEmptyValue(v reflect.Value) bool {
	return !v.IsValid() || v.IsZero()
}

type columnMapKey struct {
	t       reflect.Type
	tagName string
}

var (
	structMapCache     = make(map[columnMapKey]ColumnMap)
	columnSourceCache  = make(map[columnMapKey]ColumnMapProblem)
	structMapCacheLock = sync.Mutex{}
)

var (
	DefaultColumnRenameFunction = strings.ToLower
	columnRenameFunction        = DefaultColumnRenameFunction
	ignoreUntaggedFields        = false
)

func SetIgnoreUntaggedFields(ignore bool) {
	if ignore != ignoreUntaggedFields {
		ignoreUntaggedFields = ignore

		structMapCacheLock.Lock()
		defer structMapCacheLock.Unlock()

		structMapCache = make(map[columnMapKey]ColumnMap)
		columnSourceCache = make(map[columnMapKey]ColumnMapProblem)
	}
}

func SetColumnRenameFunction(newFunction func(string) string) {
	columnRenameFunction = newFunction
}

// GetSliceElementType returns the type for a slices elements.
func GetSliceElementType(val reflect.Value) reflect.Type {
	elemType := val.Type().Elem()
	if elemType.Kind() == reflect.Ptr {
		elemType = elemType.Elem()
	}

	return elemType
}

// AppendSliceElement will append val to slice. Handles slice of pointers and
// not pointers. Val needs to be a pointer.
func AppendSliceElement(slice, val reflect.Value) {
	if slice.Type().Elem().Kind() == reflect.Ptr {
		slice.Set(reflect.Append(slice, val))
	} else {
		slice.Set(reflect.Append(slice, reflect.Indirect(val)))
	}
}

func GetTypeInfo(i interface{}) (reflect.Type, reflect.Kind) {
	t := Deref(reflect.TypeOf(i))
	if kind := t.Kind(); kind == reflect.Slice || kind == reflect.Array {
		t = Deref(t.Elem())
	}
	return t, t.Kind()
}

func SafeGetFieldByIndex(v reflect.Value, fieldIndex []int) (result reflect.Value, isAvailable bool) {
	switch len(fieldIndex) {
	case 0:
		return v, true
	case 1:
		return v.FieldByIndex(fieldIndex), true
	default:
		if f := reflect.Indirect(v.Field(fieldIndex[0])); f.IsValid() {
			return SafeGetFieldByIndex(f, fieldIndex[1:])
		}
	}
	return reflect.ValueOf(nil), false
}

func SafeSetFieldByIndex(v reflect.Value, fieldIndex []int, src interface{}) (result reflect.Value) {
	v = reflect.Indirect(v)
	switch len(fieldIndex) {
	case 0:
		return v
	case 1:
		f := v.FieldByIndex(fieldIndex)
		srcVal := reflect.ValueOf(src)
		f.Set(reflect.Indirect(srcVal))
	default:
		f := v.Field(fieldIndex[0])
		switch f.Kind() {
		case reflect.Ptr:
			s := f
			if f.IsNil() || !f.IsValid() {
				s = reflect.New(f.Type().Elem())
				f.Set(s)
			}
			SafeSetFieldByIndex(reflect.Indirect(s), fieldIndex[1:], src)
		case reflect.Struct:
			SafeSetFieldByIndex(f, fieldIndex[1:], src)
		default: // use the original value
		}
	}
	return v
}

func InterfaceScanTarget(i interface{}, fieldIndex []int) (interface{}, bool) {
	f, err := reflect.Indirect(reflect.ValueOf(i)).FieldByIndexErr(fieldIndex)
	if err != nil || f.Kind() != reflect.Interface || f.IsNil() {
		return nil, false
	}
	elem := f.Elem()
	if elem.Kind() != reflect.Ptr || elem.IsNil() || !elem.Type().Implements(scannerType) {
		return nil, false
	}
	return elem.Interface(), true
}

func IsScannerInterface(t reflect.Type) bool {
	return t.Kind() == reflect.Interface && t.Implements(scannerType)
}

type rowData = map[string]interface{}

// AssignStructVals will assign the data from rd to i.
func AssignStructVals(i interface{}, rd rowData, cm ColumnMap) {
	val := reflect.Indirect(reflect.ValueOf(i))

	for name, data := range cm {
		src, ok := rd[name]
		if ok {
			SafeSetFieldByIndex(val, data.FieldIndex, src)
		}
	}
}

func NilUnexportedEmbed(v reflect.Value, fieldIndex []int) (field string, embedded reflect.Type, found bool) {
	v = reflect.Indirect(v)
	root := v.Type()
	t := root
	nilAbove := false
	for n, i := range fieldIndex[:len(fieldIndex)-1] {
		sf := t.Field(i)
		isPointer := IsPointer(sf.Type.Kind())
		if !nilAbove {
			v = v.Field(i)
			nilAbove = isPointer && v.IsNil()
			if isPointer && !nilAbove {
				v = v.Elem()
			}
		}
		if isPointer && nilAbove && !sf.IsExported() {
			return FieldPath(root, fieldIndex[:n+1]), sf.Type.Elem(), true
		}
		t = Deref(sf.Type)
	}
	return "", nil, false
}

func FieldPath(t reflect.Type, fieldIndex []int) string {
	names := make([]string, 0, len(fieldIndex))
	for _, i := range fieldIndex {
		f := t.Field(i)
		names = append(names, f.Name)
		t = Deref(f.Type)
	}
	return strings.Join(names, ".")
}

func IsSQLNull(t reflect.Type) bool {
	return IsStruct(t.Kind()) && t.PkgPath() == scannerType.PkgPath() && strings.HasPrefix(t.Name(), "Null[")
}

func ScanType(t reflect.Type) reflect.Type {
	switch {
	case IsPointer(t.Kind()):
		return reflect.PointerTo(ScanType(t.Elem()))
	case IsSQLNull(t):
		if v := t.Field(0).Type; ScanType(v) != v {
			return reflect.PointerTo(ScanType(v))
		}
	}
	return t
}

func AssignScanned(dst, scanned reflect.Value) {
	if scanned.Type() == dst.Type() {
		dst.Set(scanned)
		return
	}
	if scanned.IsNil() {
		dst.SetZero()
		return
	}
	if IsSQLNull(dst.Type()) {
		AssignScanned(dst.Field(0), scanned.Elem())
		dst.Field(1).SetBool(true)
		return
	}
	pointee := reflect.New(dst.Type().Elem())
	AssignScanned(pointee.Elem(), scanned.Elem())
	dst.Set(pointee)
}

func IsScanner(t reflect.Type) bool {
	return reflect.PointerTo(t).Implements(scannerType)
}

// ValuerValue resolves v the way database/sql does: a nil pointer whose element type implements driver.Valuer is
// NULL, any other Valuer is asked for its Value. See https://github.com/golang/go/commit/0ce1d79a6a771f7449ec493b993ed2a720917870
func ValuerValue(v driver.Valuer) (driver.Value, error) {
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Ptr && rv.IsNil() && rv.Type().Elem().Implements(valuerType) {
		return nil, nil
	}
	return v.Value()
}
