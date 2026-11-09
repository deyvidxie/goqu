package util

import (
	"reflect"
	"sort"
	"strings"

	"github.com/doug-martin/goqu/v9/internal/tag"
)

type (
	ColumnData struct {
		ColumnName        string
		ColumnSubqueryKey string
		FieldIndex        []int
		ShouldInsert      bool
		ShouldUpdate      bool
		DefaultIfEmpty    bool
		OmitNil           bool
		OmitEmpty         bool
		GoType            reflect.Type
	}
	ColumnMap map[string]ColumnData
)

func newColumnMap(t reflect.Type, fieldIndex []int, prefixes []string, tagName string) ColumnMap {
	cm, n := ColumnMap{}, t.NumField()
	var subColMaps []ColumnMap
	for i := 0; i < n; i++ {
		f := t.Field(i)
		sub, follow := followedType(&f, tagName)
		if isEmbed(&f) {
			if follow {
				subColMaps = append(subColMaps, getStructColumnMap(&f, sub, fieldIndex, tag.New(tagName, f.Tag).Values(), prefixes, tagName))
			}
		} else if f.IsExported() {
			dbTag := tag.New(tagName, f.Tag)
			subqueryTag := tag.New("subquery", f.Tag)
			columnName := getColumnName(&f, dbTag)
			columnSubqueryKey := getSubqueryKey(subqueryTag)
			if !shouldIgnoreField(dbTag) {
				if follow {
					subCm := getStructColumnMap(&f, sub, fieldIndex, []string{columnName}, prefixes, tagName)
					if len(subCm) != 0 {
						subColMaps = append(subColMaps, subCm)
						continue
					}
				}
				goquTag := tag.New("goqu", f.Tag)
				columnName = strings.Join(append(prefixes, columnName), ".")
				cm[columnName] = newColumnData(&f, columnName, columnSubqueryKey, fieldIndex, goquTag)
			}
		}
	}
	return cm.Merge(subColMaps)
}

func (cm ColumnMap) Cols() (structCols []string, colSubqueries map[string]string) {
	structCols = make([]string, 0, len(cm))
	colSubqueries = make(map[string]string, len(cm))

	for key, val := range cm {
		structCols = append(structCols, key)
		if val.ColumnSubqueryKey != "" {
			colSubqueries[key] = val.ColumnSubqueryKey
		}
	}
	sort.Strings(structCols)
	return structCols, colSubqueries
}

func (cm ColumnMap) Merge(colMaps []ColumnMap) ColumnMap {
	for _, subCm := range colMaps {
		for key, val := range subCm {
			if _, ok := cm[key]; !ok {
				cm[key] = val
			}
		}
	}
	return cm
}

func HasTaggedField(t reflect.Type, tagName string) bool {
	return hasTaggedField(t, tagName, false, map[taggedVisit]bool{})
}

type taggedVisit struct {
	t      reflect.Type
	tagged bool
}

func hasTaggedField(t reflect.Type, tagName string, tagged bool, seen map[taggedVisit]bool) bool {
	if seen[taggedVisit{t, tagged}] {
		return false
	}
	seen[taggedVisit{t, tagged}] = true
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		fieldTag := tag.New(tagName, f.Tag)
		sub, follow := followedType(&f, tagName)
		fieldTagged := tagged || !fieldTag.IsEmpty()
		switch {
		case isEmbed(&f) && follow && !IsStruct(sub.Kind()):
			if fieldTagged {
				return true
			}
		case isEmbed(&f):
			if follow && hasTaggedField(sub, tagName, fieldTagged, seen) {
				return true
			}
		case !f.IsExported() || shouldIgnoreField(fieldTag):
		case fieldTagged:
			return true
		case follow && hasTaggedField(sub, tagName, false, seen):
			return true
		}
	}
	return false
}

type ColumnMapIssue int

const (
	ColumnMapOK ColumnMapIssue = iota
	ColumnMapNonStructEmbed
	ColumnMapUnexportedPointerEmbed
	ColumnMapCycle
)

type ColumnMapProblem struct {
	Issue ColumnMapIssue
	Path  string
	Type  reflect.Type
}

func CheckColumnSource(t reflect.Type, tagName string) ColumnMapProblem {
	return checkFollowedFields(t, tagName, "", map[reflect.Type]bool{})
}

func CheckColumnMap(t reflect.Type, tagName string) ColumnMapProblem {
	if problem := CheckColumnSource(t, tagName); problem.Issue != ColumnMapOK {
		return problem
	}
	return checkUnexportedPointerEmbeds(t, StructColumnMap(t, tagName))
}

func checkFollowedFields(t reflect.Type, tagName, prefix string, ancestors map[reflect.Type]bool) ColumnMapProblem {
	ancestors[t] = true
	defer delete(ancestors, t)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		sub, follow := followedType(&f, tagName)
		field := prefix + f.Name
		switch {
		case !follow:
			continue
		case !IsStruct(sub.Kind()):
			return ColumnMapProblem{ColumnMapNonStructEmbed, field, sub}
		case ancestors[sub]:
			return ColumnMapProblem{ColumnMapCycle, field, sub}
		}
		if problem := checkFollowedFields(sub, tagName, field+".", ancestors); problem.Issue != ColumnMapOK {
			return problem
		}
	}
	return ColumnMapProblem{}
}

func checkUnexportedPointerEmbeds(t reflect.Type, cm ColumnMap) ColumnMapProblem {
	cols, _ := cm.Cols()
	for _, col := range cols {
		index := cm[col].FieldIndex
		owner, field := t, ""
		for _, i := range index[:len(index)-1] {
			f := owner.Field(i)
			field, owner = field+f.Name, Deref(f.Type)
			if isEmbed(&f) && IsPointer(f.Type.Kind()) && !f.IsExported() {
				return ColumnMapProblem{ColumnMapUnexportedPointerEmbed, field, owner}
			}
			field += "."
		}
	}
	return ColumnMapProblem{}
}

func followedType(f *reflect.StructField, tagName string) (reflect.Type, bool) {
	fieldTag := tag.New(tagName, f.Tag)
	if isEmbed(f) {
		return Deref(f.Type), !fieldTag.Contains("-")
	}
	return Deref(f.Type), f.IsExported() && !shouldIgnoreField(fieldTag) && !isColumnLeaf(f.Type)
}

func isEmbed(f *reflect.StructField) bool {
	return f.Anonymous && (IsStruct(f.Type.Kind()) || IsPointer(f.Type.Kind()))
}

func DerefAll(t reflect.Type) reflect.Type {
	for IsPointer(t.Kind()) {
		t = t.Elem()
	}
	return t
}

func Deref(t reflect.Type) reflect.Type {
	if IsPointer(t.Kind()) {
		return t.Elem()
	}
	return t
}

func isColumnLeaf(t reflect.Type) bool {
	t = Deref(t)
	return IsScanner(t) || !IsStruct(t.Kind())
}

func newColumnData(f *reflect.StructField, columnName, columnSubqueryKey string, fieldIndex []int, goquTag tag.Options) ColumnData {
	return ColumnData{
		ColumnName:        columnName,
		ColumnSubqueryKey: columnSubqueryKey,
		ShouldInsert:      !goquTag.Contains(skipInsertTagName),
		ShouldUpdate:      !goquTag.Contains(skipUpdateTagName),
		DefaultIfEmpty:    goquTag.Contains(defaultIfEmptyTagName),
		OmitNil:           goquTag.Contains(omitNilTagName),
		OmitEmpty:         goquTag.Contains(omitEmptyTagName),
		FieldIndex:        concatFieldIndexes(fieldIndex, f.Index),
		GoType:            f.Type,
	}
}

func getStructColumnMap(
	f *reflect.StructField, sub reflect.Type, fieldIndex []int, fieldNames, prefixes []string, tagName string,
) ColumnMap {
	subPrefixes := prefixes
	subPrefixes = append(subPrefixes, fieldNames...)
	return newColumnMap(sub, concatFieldIndexes(fieldIndex, f.Index), subPrefixes, tagName)
}

func getColumnName(f *reflect.StructField, dbTag tag.Options) string {
	if dbTag.IsEmpty() {
		return columnRenameFunction(f.Name)
	}
	return dbTag.Values()[0]
}

func getSubqueryKey(subqueryTag tag.Options) string {
	if subqueryTag.IsEmpty() {
		return ""
	}
	return string(subqueryTag)
}

func shouldIgnoreField(dbTag tag.Options) bool {
	if dbTag.Equals("-") {
		return true
	} else if dbTag.IsEmpty() && ignoreUntaggedFields {
		return true
	}

	return false
}

// safely concat two fieldIndex slices into one.
func concatFieldIndexes(fieldIndexPath, fieldIndex []int) []int {
	fieldIndexes := make([]int, 0, len(fieldIndexPath)+len(fieldIndex))
	fieldIndexes = append(fieldIndexes, fieldIndexPath...)
	return append(fieldIndexes, fieldIndex...)
}
