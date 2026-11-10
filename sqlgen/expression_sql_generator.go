package sqlgen

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/sb"
	"github.com/doug-martin/goqu/v9/internal/util"
)

type (
	// An adapter interface to be used by a Dataset to generate SQL for a specific dialect.
	// See DefaultAdapter for a concrete implementation and examples.
	ExpressionSQLGenerator interface {
		Dialect() string
		Generate(b sb.SQLBuilder, val interface{})
	}
	// The default adapter. This class should be used when building a new adapter. When creating a new adapter you can
	// either override methods, or more typically update default values.
	// See (github.com/doug-martin/goqu/dialect/postgres)
	expressionSQLGenerator struct {
		dialect        string
		dialectOptions *SQLDialectOptions
	}
)

// replacementByte is replacementRune as a byte so placeholders can be found bytewise in valid UTF-8 text.
const (
	replacementByte byte = '?'
	// quoteOverhead is the room reserved for the surrounding quotes of a string/bytes literal.
	quoteOverhead = 2
	// identifierOverhead is the room reserved for quotes and separators of a qualified identifier.
	identifierOverhead = 32
)

var (
	replacementRune = '?'
	TrueLiteral     = exp.NewLiteralExpression("TRUE")
	FalseLiteral    = exp.NewLiteralExpression("FALSE")
)

func (esg *expressionSQLGenerator) likeIsCaseInsensitive() bool {
	return bytes.Equal(esg.dialectOptions.BooleanOperatorLookup[exp.ILikeOp], []byte("LIKE"))
}

var jsonFunctionNames = map[exp.JSONFunction]string{
	exp.JSONExtractFunction:  "JSONExtract",
	exp.JSONValueFunction:    "JSONValue",
	exp.JSONContainsFunction: "JSONContains",
}

func NewExpressionSQLGenerator(dialect string, do *SQLDialectOptions) ExpressionSQLGenerator {
	return &expressionSQLGenerator{dialect: dialect, dialectOptions: do}
}

func (esg *expressionSQLGenerator) Dialect() string {
	return esg.dialect
}

func (esg *expressionSQLGenerator) Generate(b sb.SQLBuilder, val interface{}) {
	if b.Error() != nil {
		return
	}
	if val == nil {
		esg.literalNil(b)
		return
	}
	switch v := val.(type) {
	case exp.Expression:
		esg.expressionSQL(b, v)
	case driver.Valuer:
		esg.valuerSQL(b, v)
	case int:
		esg.literalInt(b, int64(v))
	case int32:
		esg.literalInt(b, int64(v))
	case int64:
		esg.literalInt(b, v)
	case uint64:
		// uint and uint32 (and named unsigned types) reach here through reflectSQL.
		esg.literalUint(b, v)
	case float32:
		esg.literalFloat(b, float64(v))
	case float64:
		esg.literalFloat(b, v)
	case string:
		esg.literalString(b, v)
	case bool:
		esg.literalBool(b, v)
	case time.Time:
		esg.literalTime(b, v)
	case *time.Time:
		if v == nil {
			esg.literalNil(b)
			return
		}
		esg.literalTime(b, *v)
	default:
		if !esg.typedSliceSQL(b, val) {
			esg.reflectSQL(b, val)
		}
	}
}

func (esg *expressionSQLGenerator) valuerSQL(b sb.SQLBuilder, v driver.Valuer) {
	dVal, err := valuerValue(v)
	if err != nil {
		b.SetError(errValuerValue(v, err))
		return
	}
	esg.Generate(b, dVal)
}

var valuerReflectType = reflect.TypeOf((*driver.Valuer)(nil)).Elem()

// See https://github.com/golang/go/commit/0ce1d79a6a771f7449ec493b993ed2a720917870
func valuerValue(v driver.Valuer) (driver.Value, error) {
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Ptr && rv.IsNil() && rv.Type().Elem().Implements(valuerReflectType) {
		return nil, nil
	}
	return v.Value()
}

// typedSliceSQL handles the most common slice types without reflection. It returns false if val is not one of them.
// Only exact types are matched (not named types such as pq.StringArray), exactly like the reflection path did for
// []byte and []exp.CommonTableExpression.
func (esg *expressionSQLGenerator) typedSliceSQL(b sb.SQLBuilder, val any) bool {
	switch v := val.(type) {
	case []any:
		sliceSQL(esg, b, v, sliceElemHint)
	case []int64:
		sliceSQL(esg, b, v, sliceElemHint)
	case []int:
		sliceSQL(esg, b, v, sliceElemHint)
	case []string:
		sliceSQL(esg, b, v, sliceElemHint)
	case []float64:
		sliceSQL(esg, b, v, sliceElemHint)
	case []bool:
		sliceSQL(esg, b, v, sliceElemHint)
	case []time.Time:
		sliceSQL(esg, b, v, sliceElemHint)
	case []byte:
		esg.literalBytes(b, v)
	case json.RawMessage:
		if v == nil {
			// an unset RawMessage is not valid JSON, so write NULL instead of an empty string
			esg.literalNil(b)
		} else {
			esg.literalBytes(b, v)
		}
	case []exp.CommonTableExpression:
		esg.commonTablesSliceSQL(b, v)
	default:
		return false
	}
	return true
}

// sliceElemHint is a rough number of bytes per element used to pre-size the buffer for unprepared statements
// (prepared statements only write a short placeholder per element).
const (
	sliceElemHint         = 12
	sliceElemHintPrepared = 4
)

// sliceSQL writes (e1, e2, ...) for any slice, pre-sizing the buffer and the args for large slices.
func sliceSQL[T any](esg *expressionSQLGenerator, b sb.SQLBuilder, s []T, hint int) {
	b.WriteRunes(esg.dialectOptions.LeftParenRune)
	if b.IsPrepared() {
		b.GrowArgs(len(s))
		b.GrowBuffer(len(s) * sliceElemHintPrepared)
	} else {
		b.GrowBuffer(len(s) * hint)
	}
	for i, v := range s {
		esg.Generate(b, v)
		if i < len(s)-1 {
			b.WriteRunes(esg.dialectOptions.CommaRune, esg.dialectOptions.SpaceRune)
		}
	}
	b.WriteRunes(esg.dialectOptions.RightParenRune)
}

// isBindableKind returns false for kinds that can never be sent to a database as an argument.
func isBindableKind(k reflect.Kind) bool {
	switch k {
	case reflect.Func, reflect.Chan, reflect.UnsafePointer, reflect.Complex64, reflect.Complex128:
		return false
	default:
		return true
	}
}

func (esg *expressionSQLGenerator) reflectSQL(b sb.SQLBuilder, val interface{}) {
	v := reflect.Indirect(reflect.ValueOf(val))
	valKind := v.Kind()
	switch {
	case util.IsInvalid(valKind):
		esg.literalNil(b)
	case util.IsSlice(valKind):
		// pointers to byte slices reach here (the pointer is dereferenced above)
		if v.CanInterface() && esg.typedSliceSQL(b, v.Interface()) {
			return
		}
		esg.sliceValueSQL(b, v)
	case valKind == reflect.Array && util.IsListValue(val):
		esg.sliceValueSQL(b, v)
	case util.IsInt(valKind):
		esg.Generate(b, v.Int())
	case util.IsUint(valKind):
		esg.Generate(b, v.Uint())
	case util.IsFloat(valKind):
		esg.Generate(b, v.Float())
	case util.IsString(valKind):
		esg.Generate(b, v.String())
	case util.IsBool(valKind):
		esg.Generate(b, v.Bool())
	default:
		// In prepared mode, pass values of other kinds (structs, maps, arrays such as UUIDs)
		// through as arguments and let the driver decide whether it can bind them.
		// Kinds that no driver can bind are still rejected here.
		if b.IsPrepared() && isBindableKind(valKind) {
			esg.placeHolderSQL(b, val)

			return
		}

		b.SetError(errUnencodableValue(val))
	}
}

//nolint:gocyclo,funlen
func (esg *expressionSQLGenerator) expressionSQL(b sb.SQLBuilder, expression exp.Expression) {
	switch e := expression.(type) {
	case exp.ColumnListExpression:
		esg.columnListSQL(b, e)
	case exp.ExpressionList:
		esg.expressionListSQL(b, e)
	case exp.LiteralExpression:
		esg.literalExpressionSQL(b, e)
	case exp.IdentifierExpression:
		esg.identifierExpressionSQL(b, e)
	case exp.LateralExpression:
		esg.lateralExpressionSQL(b, e)
	case exp.AliasedExpression:
		esg.aliasedExpressionSQL(b, e)
	case exp.BooleanExpression:
		esg.booleanExpressionSQL(b, e)
	case exp.BitwiseExpression:
		esg.bitwiseExpressionSQL(b, e)
	case exp.ArithmeticExpression:
		esg.arithmeticExpressionSQL(b, e)
	case exp.RangeExpression:
		esg.rangeExpressionSQL(b, e)
	case exp.OrderedExpression:
		esg.orderedExpressionSQL(b, e)
	case exp.UpdateExpression:
		esg.updateExpressionSQL(b, e)
	case exp.SQLFunctionExpression:
		esg.sqlFunctionExpressionSQL(b, e)
	case exp.SQLWindowFunctionExpression:
		esg.sqlWindowFunctionExpression(b, e)
	case exp.WindowExpression:
		esg.windowExpressionSQL(b, e)
	case exp.CastExpression:
		esg.castExpressionSQL(b, e)
	case exp.CollateExpression:
		esg.collateExpressionSQL(b, e)
	case exp.ExcludedExpression:
		esg.excludedExpressionSQL(b, e)
	case exp.DefaultExpression:
		b.SetError(errDefaultOutsideValues)
	case exp.InvalidExpression:
		b.SetError(e.Err())
	case exp.JSONExpression:
		esg.jsonExpressionSQL(b, e)
	case exp.TimeFunctionExpression:
		esg.timeFunctionExpressionSQL(b, e)
	case exp.IntervalExpression:
		esg.intervalExpressionSQL(b, e)
	case exp.AppendableExpression:
		esg.appendableExpressionSQL(b, e)
	case exp.CommonTableExpression:
		esg.commonTableExpressionSQL(b, e)
	case exp.CompoundExpression:
		esg.compoundExpressionSQL(b, e)
	case exp.CaseExpression:
		esg.caseExpressionSQL(b, e)
	case exp.Ex:
		esg.expressionMapSQL(b, e)
	case exp.ExOr:
		esg.expressionOrMapSQL(b, e)
	default:
		b.SetError(errUnsupportedExpressionType(e))
	}
}

// Generates a placeholder (e.g. ?, $1)
func (esg *expressionSQLGenerator) placeHolderSQL(b sb.SQLBuilder, i interface{}) {
	b.Write(esg.dialectOptions.PlaceHolderFragment)
	if esg.dialectOptions.IncludePlaceholderNum {
		b.WriteInt(int64(b.CurrentArgPosition()))
	}
	b.WriteArg(i)
}

// Generates creates the sql for a sub select on a Dataset
func (esg *expressionSQLGenerator) appendableExpressionSQL(b sb.SQLBuilder, a exp.AppendableExpression) {
	b.WriteRunes(esg.dialectOptions.LeftParenRune)
	a.AppendSQL(b)
	b.WriteRunes(esg.dialectOptions.RightParenRune)
	if a.GetAs() != nil {
		b.Write(esg.dialectOptions.AsFragment)
		esg.Generate(b, a.GetAs())
	}
}

// Quotes an identifier (e.g. "col", "table"."col"
func (esg *expressionSQLGenerator) identifierExpressionSQL(b sb.SQLBuilder, ident exp.IdentifierExpression) {
	if ident.IsEmpty() {
		b.SetError(errEmptyIdentifier)
		return
	}
	schema, table, col := ident.GetSchema(), ident.GetTable(), ident.GetCol()
	b.GrowBuffer(len(schema) + len(table) + identifierOverhead)
	if schema != esg.dialectOptions.EmptyString {
		esg.quotedIdentifierSQL(b, schema)
	}
	if table != esg.dialectOptions.EmptyString {
		if schema != esg.dialectOptions.EmptyString {
			b.WriteRunes(esg.dialectOptions.PeriodRune)
		}
		esg.quotedIdentifierSQL(b, table)
	}
	switch t := col.(type) {
	case nil:
	case string:
		if col != esg.dialectOptions.EmptyString {
			if table != esg.dialectOptions.EmptyString || schema != esg.dialectOptions.EmptyString {
				b.WriteRunes(esg.dialectOptions.PeriodRune)
			}
			esg.quotedIdentifierSQL(b, t)
		}
	case exp.LiteralExpression:
		if table != esg.dialectOptions.EmptyString || schema != esg.dialectOptions.EmptyString {
			b.WriteRunes(esg.dialectOptions.PeriodRune)
		}
		esg.Generate(b, t)
	default:
		b.SetError(errUnsupportedColumnType(col))
	}
}

func (esg *expressionSQLGenerator) quotedIdentifierSQL(b sb.SQLBuilder, name string) {
	q := string(esg.dialectOptions.QuoteRune)
	b.WriteStrings(q, strings.ReplaceAll(name, q, q+q), q)
}

func (esg *expressionSQLGenerator) lateralExpressionSQL(b sb.SQLBuilder, le exp.LateralExpression) {
	if !esg.dialectOptions.SupportsLateral {
		b.SetError(errLateralNotSupported(esg.dialect))
		return
	}
	b.Write(esg.dialectOptions.LateralFragment)
	esg.Generate(b, le.Table())
}

// Generates SQL NULL value
func (esg *expressionSQLGenerator) literalNil(b sb.SQLBuilder) {
	if b.IsPrepared() {
		esg.placeHolderSQL(b, nil)
		return
	}
	b.Write(esg.dialectOptions.Null)
}

// Generates SQL bool literal, (e.g. TRUE, FALSE, mysql 1, 0, sqlite3 1, 0)
func (esg *expressionSQLGenerator) literalBool(b sb.SQLBuilder, bl bool) {
	if b.IsPrepared() {
		esg.placeHolderSQL(b, bl)
		return
	}
	if bl {
		b.Write(esg.dialectOptions.True)
	} else {
		b.Write(esg.dialectOptions.False)
	}
}

// Generates SQL for a time.Time value
func (esg *expressionSQLGenerator) literalTime(b sb.SQLBuilder, t time.Time) {
	if b.IsPrepared() {
		esg.placeHolderSQL(b, t)
		return
	}
	esg.Generate(b, t.In(timeLocation).Format(esg.dialectOptions.TimeFormat))
}

// Generates SQL for a Float Value
func (esg *expressionSQLGenerator) literalFloat(b sb.SQLBuilder, f float64) {
	if b.IsPrepared() {
		esg.placeHolderSQL(b, f)
		return
	}
	b.WriteFloat(f)
}

// Generates SQL for an int value
func (esg *expressionSQLGenerator) literalInt(b sb.SQLBuilder, i int64) {
	if b.IsPrepared() {
		esg.placeHolderSQL(b, i)
		return
	}
	b.WriteInt(i)
}

// Generates SQL for an uint value
func (esg *expressionSQLGenerator) literalUint(b sb.SQLBuilder, i uint64) {
	if b.IsPrepared() {
		if i <= math.MaxInt64 {
			esg.placeHolderSQL(b, int64(i))
		} else {
			esg.placeHolderSQL(b, i)
		}
		return
	}
	b.WriteStrings(strconv.FormatUint(i, 10))
}

// Generates SQL for a string
func (esg *expressionSQLGenerator) literalString(b sb.SQLBuilder, s string) {
	if b.IsPrepared() {
		esg.placeHolderSQL(b, s)
		return
	}
	b.GrowBuffer(len(s) + quoteOverhead)
	b.WriteRunes(esg.dialectOptions.StringQuote)
	for _, char := range s {
		if e, ok := esg.dialectOptions.EscapedRunes[char]; ok {
			b.Write(e)
		} else {
			b.WriteRunes(char)
		}
	}
	b.WriteRunes(esg.dialectOptions.StringQuote)
}

// Generates SQL for a slice of bytes
func (esg *expressionSQLGenerator) literalBytes(b sb.SQLBuilder, bs []byte) {
	if b.IsPrepared() {
		esg.placeHolderSQL(b, bs)
		return
	}
	b.GrowBuffer(len(bs) + quoteOverhead)
	b.WriteRunes(esg.dialectOptions.StringQuote)
	i := 0
	for len(bs) > 0 {
		char, l := utf8.DecodeRune(bs)
		if e, ok := esg.dialectOptions.EscapedRunes[char]; ok {
			b.Write(e)
		} else {
			b.WriteRunes(char)
		}
		i++
		bs = bs[l:]
	}
	b.WriteRunes(esg.dialectOptions.StringQuote)
}

func (esg *expressionSQLGenerator) sliceValueSQL(b sb.SQLBuilder, slice reflect.Value) {
	b.WriteRunes(esg.dialectOptions.LeftParenRune)
	for i, l := 0, slice.Len(); i < l; i++ {
		esg.Generate(b, slice.Index(i).Interface())
		if i < l-1 {
			b.WriteRunes(esg.dialectOptions.CommaRune, esg.dialectOptions.SpaceRune)
		}
	}
	b.WriteRunes(esg.dialectOptions.RightParenRune)
}

// Generates SQL for an AliasedExpression (e.g. I("a").As("b") -> "a" AS "b")
func (esg *expressionSQLGenerator) aliasedExpressionSQL(b sb.SQLBuilder, aliased exp.AliasedExpression) {
	esg.Generate(b, aliased.Aliased())
	b.Write(esg.dialectOptions.AsFragment)
	esg.Generate(b, aliased.GetAs())
}

func (esg *expressionSQLGenerator) booleanExpressionSQL(b sb.SQLBuilder, operator exp.BooleanExpression) {
	operatorOp := operator.Op()
	if (operatorOp == exp.InOp || operatorOp == exp.NotInOp) && isEmptyList(operator.RHS()) {
		esg.emptyListSQL(b, operator)
		return
	}
	b.WriteRunes(esg.dialectOptions.LeftParenRune)
	esg.Generate(b, operator.LHS())
	b.WriteRunes(esg.dialectOptions.SpaceRune)
	if val, ok := esg.dialectOptions.BooleanOperatorLookup[operatorOp]; ok {
		b.Write(val)
	} else {
		b.SetError(errBooleanOperatorNotSupported(esg.dialect, operatorOp, esg.likeIsCaseInsensitive()))
		return
	}
	rhs := operator.RHS()

	if (operatorOp == exp.IsOp || operatorOp == exp.IsNotOp) && rhs != nil && !esg.dialectOptions.BooleanDataTypeSupported {
		b.SetError(errBooleanDataTypeNotSupported(esg.dialect))
		return
	}

	if (operatorOp == exp.IsOp || operatorOp == exp.IsNotOp) && esg.dialectOptions.UseLiteralIsBools {
		// these values must be interpolated because preparing them generates invalid SQL
		if util.IsNil(reflect.ValueOf(rhs)) {
			rhs = exp.NewLiteralExpression(string(esg.dialectOptions.Null))
		} else {
			switch rhs {
			case true:
				rhs = TrueLiteral
			case false:
				rhs = FalseLiteral
			}
		}
	}

	b.WriteRunes(esg.dialectOptions.SpaceRune)

	if (operatorOp == exp.IsOp || operatorOp == exp.IsNotOp) && rhs == nil && !esg.dialectOptions.BooleanDataTypeSupported {
		// e.g. for SQL server dialect which does not support "IS @p1" for "IS NULL"
		b.Write(esg.dialectOptions.Null)
	} else {
		esg.Generate(b, rhs)
	}

	esg.likeEscapeSQL(b, operatorOp)
	b.WriteRunes(esg.dialectOptions.RightParenRune)
}

func isEmptyList(rhs interface{}) bool {
	if _, valuer := rhs.(driver.Valuer); valuer || !util.IsListValue(rhs) {
		return false
	}
	return reflect.Indirect(reflect.ValueOf(rhs)).Len() == 0
}

func (esg *expressionSQLGenerator) emptyListSQL(b sb.SQLBuilder, operator exp.BooleanExpression) {
	if !esg.lhsRenders(b, operator.LHS()) {
		return
	}
	op := operator.Op()
	if _, ok := esg.dialectOptions.BooleanOperatorLookup[op]; !ok {
		b.SetError(errBooleanOperatorNotSupported(esg.dialect, op, esg.likeIsCaseInsensitive()))
		return
	}
	fragment, fragmentName, list := esg.dialectOptions.EmptyInFragment, "EmptyInFragment", "IN"
	if op == exp.NotInOp {
		fragment, fragmentName, list = esg.dialectOptions.EmptyNotInFragment, "EmptyNotInFragment", "NOT IN"
	}
	if len(fragment) == 0 {
		b.SetError(errEmptyListFragmentMissing(esg.dialect, fragmentName, list))
		return
	}
	b.Write(fragment)
}

func (esg *expressionSQLGenerator) emptyConditionSQL(b sb.SQLBuilder, listType exp.ExpressionListType) {
	fragment, fragmentName, list := esg.dialectOptions.EmptyAndFragment, "EmptyAndFragment", "AND"
	if listType == exp.OrType {
		fragment, fragmentName, list = esg.dialectOptions.EmptyOrFragment, "EmptyOrFragment", "OR"
	}
	if len(fragment) == 0 {
		b.SetError(errEmptyListFragmentMissing(esg.dialect, fragmentName, list))
		return
	}
	b.Write(fragment)
}

func (esg *expressionSQLGenerator) lhsRenders(b sb.SQLBuilder, lhs exp.Expression) bool {
	scratch := sb.NewSQLBuilder(b.IsPrepared())
	defer sb.ReleaseSQLBuilder(scratch)
	if ub, ok := b.(*conflictUpdateBuilder); ok {
		probe := ub.withBuilder(scratch)
		esg.Generate(probe, lhs)
		ub.usesExcluded = ub.usesExcluded || probe.usesExcluded
	} else {
		esg.Generate(scratch, lhs)
	}
	if err := scratch.Error(); err != nil {
		b.SetError(err)
		return false
	}
	return true
}

func (esg *expressionSQLGenerator) likeEscapeSQL(b sb.SQLBuilder, operatorOp exp.BooleanOperation) {
	isLike := operatorOp == exp.LikeOp || operatorOp == exp.NotLikeOp ||
		operatorOp == exp.ILikeOp || operatorOp == exp.NotILikeOp
	if !isLike || esg.dialectOptions.LikeEscapeKey == "" {
		return
	}
	b.WriteRunes(esg.dialectOptions.SpaceRune)
	b.WriteStrings(esg.dialectOptions.LikeEscapeKey)
	b.WriteRunes(esg.dialectOptions.SpaceRune)
	esg.Generate(b, esg.dialectOptions.LikeEscapeValue)
}

// Generates SQL for a BitwiseExpresion (e.g. I("a").BitwiseOr(2) - > "a" | 2)
func (esg *expressionSQLGenerator) bitwiseExpressionSQL(b sb.SQLBuilder, operator exp.BitwiseExpression) {
	b.WriteRunes(esg.dialectOptions.LeftParenRune)

	if operator.LHS() != nil {
		esg.Generate(b, operator.LHS())
		b.WriteRunes(esg.dialectOptions.SpaceRune)
	}

	operatorOp := operator.Op()
	if val, ok := esg.dialectOptions.BitwiseOperatorLookup[operatorOp]; ok {
		b.Write(val)
	} else {
		b.SetError(errBitwiseOperatorNotSupported(esg.dialect, operatorOp))
		return
	}

	b.WriteRunes(esg.dialectOptions.SpaceRune)
	esg.Generate(b, operator.RHS())
	b.WriteRunes(esg.dialectOptions.RightParenRune)
}

// Generates SQL for a ArithmeticExpresion (e.g. I("a").Add(1) -> "a" + 1)
func (esg *expressionSQLGenerator) arithmeticExpressionSQL(b sb.SQLBuilder, operator exp.ArithmeticExpression) {
	b.WriteRunes(esg.dialectOptions.LeftParenRune)
	esg.Generate(b, operator.LHS())
	b.WriteRunes(esg.dialectOptions.SpaceRune)
	operatorOp := operator.Op()
	if val, ok := esg.dialectOptions.ArithmeticOperatorLookup[operatorOp]; ok {
		b.Write(val)
	} else {
		b.SetError(errArithmeticOperatorNotSupported(esg.dialect, operatorOp))
		return
	}
	b.WriteRunes(esg.dialectOptions.SpaceRune)
	esg.Generate(b, operator.RHS())
	b.WriteRunes(esg.dialectOptions.RightParenRune)
}

// Generates SQL for a RangeExpresion (e.g. I("a").Between(RangeVal{Start:2,End:5}) -> "a" BETWEEN 2 AND 5)
func (esg *expressionSQLGenerator) rangeExpressionSQL(b sb.SQLBuilder, operator exp.RangeExpression) {
	b.WriteRunes(esg.dialectOptions.LeftParenRune)
	esg.Generate(b, operator.LHS())
	b.WriteRunes(esg.dialectOptions.SpaceRune)
	operatorOp := operator.Op()
	if val, ok := esg.dialectOptions.RangeOperatorLookup[operatorOp]; ok {
		b.Write(val)
	} else {
		b.SetError(errRangeOperatorNotSupported(esg.dialect, operatorOp))
		return
	}
	rhs := operator.RHS()
	b.WriteRunes(esg.dialectOptions.SpaceRune)
	esg.Generate(b, rhs.Start())
	b.Write(esg.dialectOptions.AndFragment)
	esg.Generate(b, rhs.End())
	b.WriteRunes(esg.dialectOptions.RightParenRune)
}

// Generates SQL for an OrderedExpression (e.g. I("a").Asc() -> "a" ASC)
func (esg *expressionSQLGenerator) orderedExpressionSQL(b sb.SQLBuilder, order exp.OrderedExpression) {
	esg.Generate(b, order.SortExpression())
	if order.IsAsc() {
		b.Write(esg.dialectOptions.AscFragment)
	} else {
		b.Write(esg.dialectOptions.DescFragment)
	}
	switch order.NullSortType() {
	case exp.NoNullsSortType:
		return
	case exp.NullsFirstSortType:
		b.Write(esg.dialectOptions.NullsFirstFragment)
	case exp.NullsLastSortType:
		b.Write(esg.dialectOptions.NullsLastFragment)
	}
}

// Generates SQL for an ExpressionList (e.g. And(I("a").Eq("a"), I("b").Eq("b")) -> (("a" = 'a') AND ("b" = 'b')))
func (esg *expressionSQLGenerator) expressionListSQL(b sb.SQLBuilder, expressionList exp.ExpressionList) {
	if expressionList.IsEmpty() {
		esg.emptyConditionSQL(b, expressionList.Type())
		return
	}
	var op []byte
	if expressionList.Type() == exp.AndType {
		op = esg.dialectOptions.AndFragment
	} else {
		op = esg.dialectOptions.OrFragment
	}
	exps := expressionList.Expressions()
	expLen := len(exps) - 1
	if expLen > 0 {
		b.WriteRunes(esg.dialectOptions.LeftParenRune)
	} else {
		esg.Generate(b, exps[0])
		return
	}
	for i, e := range exps {
		esg.Generate(b, e)
		if i < expLen {
			b.Write(op)
		}
	}
	b.WriteRunes(esg.dialectOptions.RightParenRune)
}

// Generates SQL for a ColumnListExpression
func (esg *expressionSQLGenerator) columnListSQL(b sb.SQLBuilder, columnList exp.ColumnListExpression) {
	cols := columnList.Columns()
	colLen := len(cols)
	for i, col := range cols {
		esg.Generate(b, col)
		if i < colLen-1 {
			b.WriteRunes(esg.dialectOptions.CommaRune, esg.dialectOptions.SpaceRune)
		}
	}
}

// Generates SQL for an UpdateEpxresion
func (esg *expressionSQLGenerator) updateExpressionSQL(b sb.SQLBuilder, update exp.UpdateExpression) {
	esg.Generate(b, update.Col())
	b.WriteRunes(esg.dialectOptions.SetOperatorRune)
	writeValue(esg, esg.dialectOptions, b, update.Val())
}

func writeValue(esg ExpressionSQLGenerator, opts *SQLDialectOptions, b sb.SQLBuilder, val interface{}) {
	switch {
	case isDefault(val) && len(opts.DefaultFragment) == 0:
		b.SetError(errDefaultValueNotSupported(esg.Dialect()))
	case isDefault(val):
		b.Write(opts.DefaultFragment)
	case !util.IsValuerOrBytes(val) && util.IsListValue(val):
		b.SetError(errListAsValue(val))
	default:
		esg.Generate(b, val)
	}
}

func isDefault(val interface{}) bool {
	_, ok := val.(exp.DefaultExpression)
	return ok
}

// Generates SQL for a LiteralExpression
//
//	L("a + b") -> a + b
//	L("a = ?", 1) -> a = 1
func (esg *expressionSQLGenerator) literalExpressionSQL(b sb.SQLBuilder, literal exp.LiteralExpression) {
	l := literal.Literal()
	args := literal.Args()
	if argsLen := len(args); argsLen > 0 {
		currIndex := 0
		if !utf8.ValidString(l) {
			// Keep the historical behavior for invalid UTF-8: ranging over the string turns invalid bytes into U+FFFD.
			for _, char := range l {
				if char == replacementRune && currIndex < argsLen {
					esg.Generate(b, args[currIndex])
					currIndex++
				} else {
					b.WriteRunes(char)
				}
			}
			return
		}
		// '?' is ASCII, so it can be found bytewise and the text between placeholders written in one go.
		for l != "" {
			i := strings.IndexByte(l, replacementByte)
			if i == -1 {
				b.WriteStrings(l)
				return
			}
			b.WriteStrings(l[:i])
			if currIndex < argsLen {
				esg.Generate(b, args[currIndex])
				currIndex++
			} else {
				b.WriteRunes(replacementRune)
			}
			l = l[i+1:]
		}
		return
	}
	b.WriteStrings(l)
}

// Generates SQL for a SQLFunctionExpression
//
//	COUNT(I("a")) -> COUNT("a")
func (esg *expressionSQLGenerator) sqlFunctionExpressionSQL(b sb.SQLBuilder, sqlFunc exp.SQLFunctionExpression) {
	name := sqlFunc.Name()
	if val, ok := esg.dialectOptions.FunctionNameLookup[name]; ok {
		name = string(val)
	}
	b.WriteStrings(name)
	esg.Generate(b, sqlFunc.Args())
}

func (esg *expressionSQLGenerator) sqlWindowFunctionExpression(b sb.SQLBuilder, sqlWinFunc exp.SQLWindowFunctionExpression) {
	if !esg.dialectOptions.SupportsWindowFunction {
		b.SetError(errWindowNotSupported(esg.dialect))
		return
	}
	esg.Generate(b, sqlWinFunc.Func())
	b.Write(esg.dialectOptions.WindowOverFragment)
	switch {
	case sqlWinFunc.HasWindowName():
		esg.Generate(b, sqlWinFunc.WindowName())
	case sqlWinFunc.HasWindow():
		if sqlWinFunc.Window().HasName() {
			b.SetError(errUnexpectedNamedWindow)
			return
		}
		esg.Generate(b, sqlWinFunc.Window())
	default:
		esg.Generate(b, exp.NewWindowExpression("db", nil, nil, nil, nil))
	}
}

func (esg *expressionSQLGenerator) windowExpressionSQL(b sb.SQLBuilder, we exp.WindowExpression) {
	if !esg.dialectOptions.SupportsWindowFunction {
		b.SetError(errWindowNotSupported(esg.dialect))
		return
	}
	if we.HasName() {
		esg.Generate(b, we.Name())
		b.Write(esg.dialectOptions.AsFragment)
	}
	b.WriteRunes(esg.dialectOptions.LeftParenRune)

	hasPartition := we.HasPartitionBy()
	hasOrder := we.HasOrder()

	if we.HasParent() {
		esg.Generate(b, we.Parent())
		if hasPartition || hasOrder {
			b.WriteRunes(esg.dialectOptions.SpaceRune)
		}
	}

	if hasPartition {
		b.Write(esg.dialectOptions.WindowPartitionByFragment)
		esg.Generate(b, we.PartitionCols())
		if hasOrder {
			b.WriteRunes(esg.dialectOptions.SpaceRune)
		}
	}
	if hasOrder {
		b.Write(esg.dialectOptions.WindowOrderByFragment)
		esg.Generate(b, we.OrderCols())
	}

	b.WriteRunes(esg.dialectOptions.RightParenRune)
}

// Generates SQL for a CastExpression
//
//	I("a").Cast("NUMERIC") -> CAST("a" AS NUMERIC)
func (esg *expressionSQLGenerator) castExpressionSQL(b sb.SQLBuilder, cast exp.CastExpression) {
	b.Write(esg.dialectOptions.CastFragment).WriteRunes(esg.dialectOptions.LeftParenRune)
	esg.Generate(b, cast.Casted())
	b.Write(esg.dialectOptions.AsFragment)
	esg.Generate(b, cast.Type())
	b.WriteRunes(esg.dialectOptions.RightParenRune)
}

// Generates SQL for a CollateExpression
//
//	I("a").Collate("en_US") -> "a" COLLATE en_US
//	I("a").Collate("en_US") -> "a" COLLATE "en_US" (dialects with QuoteCollation)
//	I("a").Collate(I("pg_catalog.default")) -> "a" COLLATE "pg_catalog"."default" (dialects with QuoteCollation)
func (esg *expressionSQLGenerator) collateExpressionSQL(b sb.SQLBuilder, collate exp.CollateExpression) {
	opts := esg.dialectOptions
	parts, ok := esg.collationParts(collate.Collation())
	if !ok {
		b.SetError(errInvalidCollation(describeCollation(collate.Collation())))
		return
	}

	// a bare literal could be an arbitrary expression (e.g. a || b), only the last term would be collated
	_, isLiteral := collate.Collated().(exp.LiteralExpression)
	if isLiteral {
		b.WriteRunes(opts.LeftParenRune)
	}
	esg.Generate(b, collate.Collated())
	if isLiteral {
		b.WriteRunes(opts.RightParenRune)
	}
	b.Write(opts.CollateFragment)
	for i, part := range parts {
		if i > 0 {
			b.WriteRunes(opts.PeriodRune)
		}
		if opts.QuoteCollation {
			esg.quotedIdentifierSQL(b, part)
		} else {
			b.WriteStrings(part)
		}
	}
}

// Returns the (unquoted) parts of the collation name and whether it is valid for the dialect.
//
// A string is a single quoted name for dialects that quote collations (it may contain anything, e.g.
// "en_US.utf8"), otherwise it is split on '.' into schema qualified parts. An identifier is always split into its parts.
func (esg *expressionSQLGenerator) collationParts(collation any) (parts []string, ok bool) {
	quote := esg.dialectOptions.QuoteCollation
	switch c := collation.(type) {
	case string:
		if quote {
			return []string{c}, c != "" && !strings.ContainsRune(c, 0)
		}
		parts = strings.Split(c, ".")
	case exp.IdentifierExpression:
		if c.IsEmpty() {
			return nil, false
		}
		for _, part := range []any{c.GetSchema(), c.GetTable(), c.GetCol()} {
			switch p := part.(type) {
			case nil:
			case string:
				if p != esg.dialectOptions.EmptyString {
					parts = append(parts, p)
				}
			default:
				return nil, false
			}
		}
	default:
		return nil, false
	}
	if len(parts) == 0 || len(parts) > 3 {
		return nil, false
	}
	for _, part := range parts {
		if quote {
			if part == "" || strings.ContainsRune(part, 0) {
				return nil, false
			}
		} else if !validCollationPart(part) {
			return nil, false
		}
	}
	return parts, true
}

// describes a collation for error messages, identifiers are shown as their dotted name rather than as a struct
func describeCollation(collation any) string {
	if ident, ok := collation.(exp.IdentifierExpression); ok {
		var parts []string
		for _, part := range []any{ident.GetSchema(), ident.GetTable(), ident.GetCol()} {
			if p, isString := part.(string); isString && p != "" {
				parts = append(parts, p)
			}
		}
		return strings.Join(parts, ".")
	}
	return fmt.Sprintf("%v", collation)
}

var intervalUnits = []struct {
	unit     exp.IntervalUnit
	quantity string
}{
	{exp.MicrosecondInterval, "1 MICROSECOND"},
	{exp.SecondInterval, "1 SECOND"},
	{exp.MinuteInterval, "1 MINUTE"},
	{exp.HourInterval, "1 HOUR"},
	{exp.DayInterval, "1 DAY"},
	{exp.WeekInterval, "1 WEEK"},
	{exp.MonthInterval, "1 MONTH"},
	{exp.QuarterInterval, "3 MONTH"},
	{exp.YearInterval, "1 YEAR"},
}

var intervalUnitNames = func() string {
	names := make([]string, len(intervalUnits))
	for i, u := range intervalUnits {
		names[i] = string(u.unit)
	}
	return strings.Join(names, ", ")
}()

var durationType = reflect.TypeOf(time.Duration(0))

func intervalCount(dialect string, count any, unit exp.IntervalUnit, fractionalSeconds bool) (any, error) {
	value := count
	switch v := count.(type) {
	case exp.Ex, exp.ExOr:
		return nil, errIntervalCountCondition
	case exp.Expression:
		return count, nil
	case driver.Valuer:
		resolved, err := valuerValue(v)
		if err != nil {
			return nil, errIntervalCountValue(err)
		}
		value = resolved
	}
	rv := reflect.Indirect(reflect.ValueOf(value))
	switch {
	case !rv.IsValid():
		return value, nil
	case rv.Type() == durationType:
	case util.IsInt(rv.Kind()), util.IsUint(rv.Kind()):
		return value, nil
	case util.IsFloat(rv.Kind()) && fractionalSeconds && unit == exp.SecondInterval &&
		!math.IsNaN(rv.Float()) && !math.IsInf(rv.Float(), 0):
		return value, nil
	}
	return nil, errIntervalCount(dialect, count, rv.Interface(), unit, fractionalSeconds)
}

func intervalUnit(
	dialect string, e exp.IntervalExpression, fractionalSeconds bool,
) (unit exp.IntervalUnit, quantity string, count any, err error) {
	unit, quantity, err = lookupIntervalUnit(e.Unit())
	if err != nil {
		return "", "", nil, err
	}
	count, err = intervalCount(dialect, e.Count(), unit, fractionalSeconds)
	if err != nil {
		return "", "", nil, err
	}
	return unit, quantity, count, nil
}

func lookupIntervalUnit(unit exp.IntervalUnit) (canonical exp.IntervalUnit, quantity string, err error) {
	name := string(unit)
	for i := 0; i < len(name); i++ {
		if name[i] >= utf8.RuneSelf {
			return "", "", errUnknownIntervalUnit(unit)
		}
	}
	name = strings.ToUpper(name)
	for _, u := range intervalUnits {
		if string(u.unit) == name {
			return u.unit, u.quantity, nil
		}
	}
	return "", "", errUnknownIntervalUnit(unit)
}

const (
	millisecondFSP = 3
	microsecondFSP = 6
)

type timeRenderers struct {
	now      func(b sb.SQLBuilder, utc bool, fsp int, hasFSP bool) error
	interval func(b sb.SQLBuilder, e exp.IntervalExpression) error
}

func (esg *expressionSQLGenerator) timeRenderers() (timeRenderers, error) {
	switch esg.dialectOptions.TimeStyle {
	case PostgresTimeStyle:
		return timeRenderers{now: esg.postgresTimeSQL, interval: esg.postgresIntervalSQL}, nil
	case MySQLTimeStyle:
		return timeRenderers{now: esg.mysqlTimeSQL, interval: esg.mysqlIntervalSQL}, nil
	case SQLiteTimeStyle:
		return timeRenderers{now: esg.sqliteTimeSQL, interval: esg.unsupportedIntervalSQL}, nil
	case SQLServerTimeStyle:
		return timeRenderers{now: esg.sqlServerTimeSQL, interval: esg.unsupportedIntervalSQL}, nil
	case DuckDBTimeStyle:
		return timeRenderers{now: esg.duckDBTimeSQL, interval: esg.duckDBIntervalSQL}, nil
	default:
		return timeRenderers{}, errUnknownTimeStyle(esg.dialect, esg.dialectOptions.TimeStyle)
	}
}

func (esg *expressionSQLGenerator) timeFunctionExpressionSQL(b sb.SQLBuilder, e exp.TimeFunctionExpression) {
	render, err := esg.timeRenderers()
	if err != nil {
		b.SetError(err)
		return
	}
	fsp, hasFSP, err := timeFSP(e.FSP())
	if err != nil {
		b.SetError(err)
		return
	}
	var utc bool
	switch e.Function() {
	case exp.NowFunction:
	case exp.UTCTimestampFunction:
		utc = true
	default:
		b.SetError(errUnknownTimeFunction(e.Function()))
		return
	}
	if err := render.now(b, utc, fsp, hasFSP); err != nil {
		b.SetError(err)
	}
}

func timeFSP(fsp []int) (value int, ok bool, err error) {
	switch len(fsp) {
	case 0:
		return 0, false, nil
	case 1:
		if fsp[0] < 0 || fsp[0] > microsecondFSP {
			return 0, false, errTimeFSPRange(fsp[0])
		}
		return fsp[0], true, nil
	default:
		return 0, false, errTimeFSPCount(len(fsp))
	}
}

func timeFunc(name string, fsp int, hasFSP bool) exp.SQLFunctionExpression {
	if hasFSP {
		return exp.NewSQLFunctionExpression(name, exp.NewLiteralExpression(strconv.Itoa(fsp)))
	}
	return exp.NewSQLFunctionExpression(name)
}

func (esg *expressionSQLGenerator) parenthesizedSQL(b sb.SQLBuilder, e exp.Expression) {
	b.WriteRunes(esg.dialectOptions.LeftParenRune)
	esg.Generate(b, e)
	b.WriteRunes(esg.dialectOptions.RightParenRune)
}

func (esg *expressionSQLGenerator) postgresTimeSQL(b sb.SQLBuilder, utc bool, fsp int, hasFSP bool) error {
	now := exp.NewSQLFunctionExpression("NOW")
	if hasFSP {
		now = timeFunc("CURRENT_TIMESTAMP", fsp, hasFSP)
	}
	if utc {
		esg.parenthesizedSQL(b, exp.NewLiteralExpression("? AT TIME ZONE 'UTC'", now))
		return nil
	}
	esg.Generate(b, now)
	return nil
}

func (esg *expressionSQLGenerator) mysqlTimeSQL(b sb.SQLBuilder, utc bool, fsp int, hasFSP bool) error {
	name := "NOW"
	if utc {
		name = "UTC_TIMESTAMP"
	}
	esg.Generate(b, timeFunc(name, fsp, hasFSP))
	return nil
}

func (esg *expressionSQLGenerator) sqliteTimeSQL(b sb.SQLBuilder, utc bool, fsp int, hasFSP bool) error {
	args := []any{exp.NewLiteralExpression("'now'")}
	if !utc {
		args = append(args, exp.NewLiteralExpression("'localtime'"))
	}
	switch {
	case !hasFSP || fsp == 0:
		esg.Generate(b, exp.NewSQLFunctionExpression("datetime", args...))
	case fsp == millisecondFSP:
		esg.Generate(b, exp.NewSQLFunctionExpression("strftime", append([]any{exp.NewLiteralExpression("'%Y-%m-%d %H:%M:%f'")}, args...)...))
	default:
		return errTimeFSPNotSupported(esg.dialect, fsp, 0, millisecondFSP)
	}
	return nil
}

func (esg *expressionSQLGenerator) sqlServerTimeSQL(b sb.SQLBuilder, utc bool, fsp int, hasFSP bool) error {
	now := exp.NewSQLFunctionExpression("SYSDATETIME")
	if utc {
		now = exp.NewSQLFunctionExpression("SYSUTCDATETIME")
	}
	if hasFSP {
		esg.Generate(b, exp.NewCastExpression(now, fmt.Sprintf("DATETIME2(%d)", fsp)))
		return nil
	}
	esg.Generate(b, now)
	return nil
}

func (esg *expressionSQLGenerator) duckDBTimeSQL(b sb.SQLBuilder, utc bool, fsp int, hasFSP bool) error {
	var now exp.Expression = exp.NewCastExpression(exp.NewSQLFunctionExpression("now"), "TIMESTAMP")
	if utc {
		now = exp.NewSQLFunctionExpression("make_timestamp", exp.NewSQLFunctionExpression("epoch_us", exp.NewSQLFunctionExpression("now")))
	}
	switch {
	case !hasFSP || fsp == microsecondFSP:
		esg.Generate(b, now)
	case fsp == 0:
		esg.Generate(b, exp.NewCastExpression(now, "TIMESTAMP_S"))
	case fsp == millisecondFSP:
		esg.Generate(b, exp.NewCastExpression(now, "TIMESTAMP_MS"))
	default:
		return errTimeFSPNotSupported(esg.dialect, fsp, 0, millisecondFSP, microsecondFSP)
	}
	return nil
}

func (esg *expressionSQLGenerator) intervalExpressionSQL(b sb.SQLBuilder, e exp.IntervalExpression) {
	render, err := esg.timeRenderers()
	if err == nil {
		err = render.interval(b, e)
	}
	if err != nil {
		b.SetError(err)
	}
}

func (esg *expressionSQLGenerator) mysqlIntervalSQL(b sb.SQLBuilder, e exp.IntervalExpression) error {
	unit, _, count, err := intervalUnit(esg.dialect, e, true)
	if err != nil {
		return err
	}
	esg.Generate(b, exp.NewLiteralExpression("INTERVAL ? "+string(unit), count))
	return nil
}

func (esg *expressionSQLGenerator) postgresIntervalSQL(b sb.SQLBuilder, e exp.IntervalExpression) error {
	return esg.multipliedIntervalSQL(b, e, true)
}

func (esg *expressionSQLGenerator) duckDBIntervalSQL(b sb.SQLBuilder, e exp.IntervalExpression) error {
	return esg.multipliedIntervalSQL(b, e, false)
}

func (esg *expressionSQLGenerator) multipliedIntervalSQL(b sb.SQLBuilder, e exp.IntervalExpression, fractionalSeconds bool) error {
	_, quantity, count, err := intervalUnit(esg.dialect, e, fractionalSeconds)
	if err != nil {
		return err
	}
	b.WriteRunes(esg.dialectOptions.LeftParenRune)
	if literal, isLiteral := count.(exp.LiteralExpression); isLiteral {
		esg.parenthesizedSQL(b, literal)
	} else {
		esg.Generate(b, count)
	}
	b.WriteStrings(" * INTERVAL '" + quantity + "'")
	b.WriteRunes(esg.dialectOptions.RightParenRune)
	return nil
}

func (esg *expressionSQLGenerator) unsupportedIntervalSQL(sb.SQLBuilder, exp.IntervalExpression) error {
	return errIntervalNotSupported(esg.dialect)
}

func (esg *expressionSQLGenerator) jsonExpressionSQL(b sb.SQLBuilder, e exp.JSONExpression) {
	if !esg.dialectOptions.SupportsMySQLJSONFunctions {
		name, ok := jsonFunctionNames[e.Function()]
		if !ok {
			b.SetError(errUnknownJSONFunction(e.Function()))
			return
		}
		b.SetError(errJSONFunctionNotSupported(esg.dialect, name))
		return
	}
	paths := make([]interface{}, len(e.Paths()))
	for i, path := range e.Paths() {
		paths[i] = path
	}
	switch e.Function() {
	case exp.JSONExtractFunction:
		esg.Generate(b, exp.NewSQLFunctionExpression("JSON_EXTRACT", append([]interface{}{e.Col()}, paths...)...))
	case exp.JSONValueFunction:
		extract := exp.NewSQLFunctionExpression("JSON_EXTRACT", append([]interface{}{e.Col()}, paths...)...)
		esg.Generate(b, exp.NewSQLFunctionExpression("JSON_UNQUOTE", extract))
	case exp.JSONContainsFunction:
		if len(paths) > 1 {
			b.SetError(errJSONContainsPaths(len(paths)))
			return
		}
		candidate, err := jsonCandidate(e.Candidate())
		if err != nil {
			b.SetError(err)
			return
		}
		esg.Generate(b, exp.NewSQLFunctionExpression("JSON_CONTAINS", append([]interface{}{e.Col(), candidate}, paths...)...))
	default:
		b.SetError(errUnknownJSONFunction(e.Function()))
	}
}

func jsonCandidate(v interface{}) (interface{}, error) {
	switch v.(type) {
	case exp.Ex, exp.ExOr:
		return nil, errJSONCandidateCondition
	}
	if e, ok := v.(exp.Expression); ok {
		return e, nil
	}
	if _, marshals := v.(json.Marshaler); !marshals {
		if valuer, ok := v.(driver.Valuer); ok {
			value, err := valuerValue(valuer)
			if err != nil {
				return nil, errJSONCandidate(err)
			}
			v = value
		}
		if raw, ok := jsonBytes(v); ok {
			if raw == nil {
				return "null", nil
			}
			if !json.Valid(raw) {
				return nil, errJSONCandidateInvalid
			}
			return string(raw), nil
		}
	}
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, errJSONCandidate(err)
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

func jsonBytes(v interface{}) ([]byte, bool) {
	switch b := v.(type) {
	case []byte:
		return b, true
	case *[]byte:
		if b == nil {
			return nil, true
		}
		return *b, true
	default:
		return nil, false
	}
}

func (esg *expressionSQLGenerator) excludedExpressionSQL(b sb.SQLBuilder, e exp.ExcludedExpression) {
	ub, ok := b.(*conflictUpdateBuilder)
	if !ok {
		b.SetError(errExcludedOutsideConflictUpdate)
		return
	}
	if e.GetCol() == "" {
		b.SetError(errExcludedWithoutColumn)
		return
	}
	ub.usesExcluded = true
	switch esg.dialectOptions.ExcludedStyle {
	case ExcludedValuesFuncStyle:
		if len(esg.dialectOptions.ExcludedValuesFuncFragment) == 0 {
			b.SetError(errExcludedFragmentMissing(esg.dialect, "ExcludedValuesFuncFragment"))
			return
		}
		b.Write(esg.dialectOptions.ExcludedValuesFuncFragment).WriteRunes(esg.dialectOptions.LeftParenRune)
		esg.quotedIdentifierSQL(b, e.GetCol())
		b.WriteRunes(esg.dialectOptions.RightParenRune)
	case ExcludedRowAliasStyle:
		if ub.rowAlias == nil {
			b.SetError(errExcludedRowAliasUnresolved)
			return
		}
		esg.Generate(b, ub.rowAlias.Col(e.GetCol()))
	case ExcludedTableStyle:
		if len(esg.dialectOptions.ExcludedTableFragment) == 0 {
			b.SetError(errExcludedFragmentMissing(esg.dialect, "ExcludedTableFragment"))
			return
		}
		b.Write(esg.dialectOptions.ExcludedTableFragment).WriteRunes(esg.dialectOptions.PeriodRune)
		esg.quotedIdentifierSQL(b, e.GetCol())
	default:
		b.SetError(errUnknownExcludedStyle(esg.dialect, esg.dialectOptions.ExcludedStyle))
	}
}

// unquoted collation name parts are restricted to simple identifiers
func validCollationPart(part string) bool {
	for i, r := range part {
		switch {
		case r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return part != ""
}

// Generates the sql for the WITH clauses for common table expressions (CTE)
func (esg *expressionSQLGenerator) commonTablesSliceSQL(b sb.SQLBuilder, ctes []exp.CommonTableExpression) {
	l := len(ctes)
	if l == 0 {
		return
	}
	if !esg.dialectOptions.SupportsWithCTE {
		b.SetError(errCTENotSupported(esg.dialect))
		return
	}
	b.Write(esg.dialectOptions.WithFragment)
	anyRecursive := false
	for _, cte := range ctes {
		anyRecursive = anyRecursive || cte.IsRecursive()
	}
	if anyRecursive {
		if !esg.dialectOptions.SupportsWithCTERecursive {
			b.SetError(errRecursiveCTENotSupported(esg.dialect))
			return
		}
		b.Write(esg.dialectOptions.RecursiveFragment)
	}
	for i, cte := range ctes {
		esg.Generate(b, cte)
		if i < l-1 {
			b.WriteRunes(esg.dialectOptions.CommaRune, esg.dialectOptions.SpaceRune)
		}
	}
	b.WriteRunes(esg.dialectOptions.SpaceRune)
}

// Generates SQL for a CommonTableExpression
func (esg *expressionSQLGenerator) commonTableExpressionSQL(b sb.SQLBuilder, cte exp.CommonTableExpression) {
	esg.Generate(b, cte.Name())
	b.Write(esg.dialectOptions.AsFragment)
	esg.Generate(b, cte.SubQuery())
}

// Generates SQL for a CompoundExpression
func (esg *expressionSQLGenerator) compoundExpressionSQL(b sb.SQLBuilder, compound exp.CompoundExpression) {
	switch compound.Type() {
	case exp.UnionCompoundType:
		b.Write(esg.dialectOptions.UnionFragment)
	case exp.UnionAllCompoundType:
		b.Write(esg.dialectOptions.UnionAllFragment)
	case exp.IntersectCompoundType:
		b.Write(esg.dialectOptions.IntersectFragment)
	case exp.IntersectAllCompoundType:
		b.Write(esg.dialectOptions.IntersectAllFragment)
	}
	if esg.dialectOptions.WrapCompoundsInParens {
		b.WriteRunes(esg.dialectOptions.LeftParenRune)
		compound.RHS().AppendSQL(b)
		b.WriteRunes(esg.dialectOptions.RightParenRune)
	} else {
		compound.RHS().AppendSQL(b)
	}
}

// Generates SQL for a CaseExpression
func (esg *expressionSQLGenerator) caseExpressionSQL(b sb.SQLBuilder, caseExpression exp.CaseExpression) {
	caseVal := caseExpression.GetValue()
	whens := caseExpression.GetWhens()
	elseResult := caseExpression.GetElse()

	if len(whens) == 0 {
		b.SetError(errEmptyCaseWhens)
		return
	}
	b.Write(esg.dialectOptions.CaseFragment)
	if caseVal != nil {
		esg.Generate(b, caseVal)
	}
	for _, when := range whens {
		b.Write(esg.dialectOptions.WhenFragment)
		esg.Generate(b, when.Condition())
		b.Write(esg.dialectOptions.ThenFragment)
		esg.Generate(b, when.Result())
	}
	if elseResult != nil {
		b.Write(esg.dialectOptions.ElseFragment)
		esg.Generate(b, elseResult.Result())
	}
	b.Write(esg.dialectOptions.EndFragment)
}

func (esg *expressionSQLGenerator) expressionMapSQL(b sb.SQLBuilder, ex exp.Ex) {
	expressionList, err := ex.ToExpressions()
	if err != nil {
		b.SetError(err)
		return
	}
	esg.Generate(b, expressionList)
}

func (esg *expressionSQLGenerator) expressionOrMapSQL(b sb.SQLBuilder, ex exp.ExOr) {
	expressionList, err := ex.ToExpressions()
	if err != nil {
		b.SetError(err)
		return
	}
	esg.Generate(b, expressionList)
}
