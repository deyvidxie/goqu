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
	if _, valuer := v.(driver.Valuer); valuer && rv.Kind() == reflect.Array {
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
	// If the value here is changing, reset the struct map cache
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

// InterfaceScanTarget returns the value stored in an interface typed struct field of i when it can be used as the
// destination of a scan: a non-nil pointer implementing sql.Scanner. This lets callers scan into fields such as
// sql.Scanner, whose concrete type is only known from the value of the field and so cannot be part of the cached
// ColumnMap.
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

// IsScannerInterface returns true if t is an interface type that sql.Scanner values can be stored in (e.g. sql.Scanner),
// i.e. one that a driver value can never be assigned to directly.
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
	allocated := false
	for n, i := range fieldIndex[:len(fieldIndex)-1] {
		sf := t.Field(i)
		if IsPointer(sf.Type.Kind()) {
			nilUnexported := allocated && !sf.IsExported()
			if !allocated {
				f := v.Field(i)
				allocated = f.IsNil()
				nilUnexported = allocated && !f.CanSet()
				if !allocated {
					v = f.Elem()
				}
			}
			if nilUnexported {
				return fieldPath(root, fieldIndex[:n+1]), sf.Type.Elem(), true
			}
		} else if !allocated {
			v = v.Field(i)
		}
		t = Deref(sf.Type)
	}
	return "", nil, false
}

func fieldPath(t reflect.Type, fieldIndex []int) string {
	names := make([]string, 0, len(fieldIndex))
	for _, i := range fieldIndex {
		f := t.Field(i)
		names = append(names, f.Name)
		t = Deref(f.Type)
	}
	return strings.Join(names, ".")
}

func GetColumnMap(i interface{}, tagName string) (ColumnMap, error) {
	t, valKind := GetTypeInfo(i)
	if valKind != reflect.Struct {
		return nil, errColumnMapNotStruct(t)
	}

	return StructColumnMap(t, tagName), nil
}

func StructColumnMap(t reflect.Type, tagName string) ColumnMap {
	structMapCacheLock.Lock()
	defer structMapCacheLock.Unlock()
	key := columnMapKey{t: t, tagName: tagName}
	if _, ok := structMapCache[key]; !ok {
		structMapCache[key] = newColumnMap(t, []int{}, []string{}, tagName)
	}
	return structMapCache[key]
}

func IsScanner(t reflect.Type) bool {
	return reflect.PointerTo(t).Implements(scannerType)
}
