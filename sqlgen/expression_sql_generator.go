package sqlgen

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/sb"
	"github.com/doug-martin/goqu/v9/internal/util"
	"github.com/doug-martin/goqu/v9/sqlbuilder"
)

type (
	// An adapter interface to be used by a Dataset to generate SQL for a specific dialect.
	// See DefaultAdapter for a concrete implementation and examples.
	ExpressionSQLGenerator interface {
		Dialect() string
		Generate(b sqlbuilder.SQLBuilder, val interface{})
	}
	// The default adapter. This class should be used when building a new adapter. When creating a new adapter you can
	// either override methods, or more typically update default values.
	// See (github.com/doug-martin/goqu/dialect/postgres)
	expressionSQLGenerator struct {
		dialect        string
		tagName        string
		dialectOptions *SQLDialectOptions
	}
)

// replacementByte never occurs inside a multi-byte UTF-8 sequence, so placeholders are found bytewise.
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

func NewExpressionSQLGenerator(dialect, tagName string, do *SQLDialectOptions) ExpressionSQLGenerator {
	return &expressionSQLGenerator{dialect: dialect, tagName: tagName, dialectOptions: do}
}

func (esg *expressionSQLGenerator) Dialect() string {
	return esg.dialect
}

func (esg *expressionSQLGenerator) Generate(b sqlbuilder.SQLBuilder, val interface{}) {
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

func (esg *expressionSQLGenerator) valuerSQL(b sqlbuilder.SQLBuilder, v driver.Valuer) {
	dVal, err := util.ValuerValue(v)
	if err != nil {
		b.SetError(errValuerValue(v, err))
		return
	}
	esg.Generate(b, dVal)
}

func (esg *expressionSQLGenerator) typedSliceSQL(b sqlbuilder.SQLBuilder, val any) bool {
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

func sliceSQL[T any](esg *expressionSQLGenerator, b sqlbuilder.SQLBuilder, s []T, hint int) {
	if len(s) == 0 {
		b.SetError(errEmptyListValue(s))
		return
	}
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

func (esg *expressionSQLGenerator) reflectSQL(b sqlbuilder.SQLBuilder, val interface{}) {
	v := reflect.Indirect(reflect.ValueOf(val))
	valKind := v.Kind()
	switch {
	case util.IsInvalid(valKind):
		esg.literalNil(b)
	case util.IsSlice(valKind):
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
		if b.IsPrepared() && isBindableKind(valKind) {
			esg.placeHolderSQL(b, val)

			return
		}

		b.SetError(errUnencodableValue(val))
	}
}

//nolint:gocyclo,funlen
func (esg *expressionSQLGenerator) expressionSQL(b sqlbuilder.SQLBuilder, expression exp.Expression) {
	switch e := expression.(type) {
	case exp.ColumnListExpression:
		esg.columnListSQL(b, e)
	case exp.StructColumnsExpression:
		esg.structColumnsSQL(b, e)
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
	case exp.RangeAggregateExpression:
		esg.rangeAggregateSQL(b, e)
	case exp.AlignExpression:
		esg.alignExpressionSQL(b, e)
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
func (esg *expressionSQLGenerator) placeHolderSQL(b sqlbuilder.SQLBuilder, i interface{}) {
	b.Write(esg.dialectOptions.PlaceHolderFragment)
	if esg.dialectOptions.IncludePlaceholderNum {
		b.WriteInt(int64(b.CurrentArgPosition()))
	}
	b.WriteArg(i)
}

// Generates creates the sql for a sub select on a Dataset
func (esg *expressionSQLGenerator) appendableExpressionSQL(b sqlbuilder.SQLBuilder, a exp.AppendableExpression) {
	if util.IsNilOrNilPointer(a) {
		b.SetError(errNilSubquery(a))
		return
	}
	if !esg.dialectOptions.SupportsNestedWithCTE && hasCommonTables(a) {
		b.SetError(errNestedCTENotSupported(esg.dialect))
		return
	}
	b.WriteRunes(esg.dialectOptions.LeftParenRune)
	a.AppendSQL(b)
	b.WriteRunes(esg.dialectOptions.RightParenRune)
	if a.GetAs() != nil {
		b.Write(esg.dialectOptions.AsFragment)
		esg.Generate(b, a.GetAs())
	}
}

// Quotes an identifier (e.g. "col", "table"."col"
func (esg *expressionSQLGenerator) identifierExpressionSQL(b sqlbuilder.SQLBuilder, ident exp.IdentifierExpression) {
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

func (esg *expressionSQLGenerator) quotedIdentifierSQL(b sqlbuilder.SQLBuilder, name string) {
	q := string(esg.dialectOptions.QuoteRune)
	b.WriteStrings(q, strings.ReplaceAll(name, q, q+q), q)
}

func (esg *expressionSQLGenerator) lateralExpressionSQL(b sqlbuilder.SQLBuilder, le exp.LateralExpression) {
	if !esg.dialectOptions.SupportsLateral {
		b.SetError(errLateralNotSupported(esg.dialect))
		return
	}
	b.Write(esg.dialectOptions.LateralFragment)
	esg.Generate(b, le.Table())
}

// Generates SQL NULL value
func (esg *expressionSQLGenerator) literalNil(b sqlbuilder.SQLBuilder) {
	if b.IsPrepared() {
		esg.placeHolderSQL(b, nil)
		return
	}
	b.Write(esg.dialectOptions.Null)
}

// Generates SQL bool literal, (e.g. TRUE, FALSE, mysql 1, 0, sqlite3 1, 0)
func (esg *expressionSQLGenerator) literalBool(b sqlbuilder.SQLBuilder, bl bool) {
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
func (esg *expressionSQLGenerator) literalTime(b sqlbuilder.SQLBuilder, t time.Time) {
	if b.IsPrepared() {
		esg.placeHolderSQL(b, t)
		return
	}
	esg.Generate(b, t.In(timeLocation).Format(esg.dialectOptions.TimeFormat))
}

// Generates SQL for a Float Value
func (esg *expressionSQLGenerator) literalFloat(b sqlbuilder.SQLBuilder, f float64) {
	if b.IsPrepared() {
		esg.placeHolderSQL(b, f)
		return
	}
	b.WriteFloat(f)
}

// Generates SQL for an int value
func (esg *expressionSQLGenerator) literalInt(b sqlbuilder.SQLBuilder, i int64) {
	if b.IsPrepared() {
		esg.placeHolderSQL(b, i)
		return
	}
	b.WriteInt(i)
}

// Generates SQL for an uint value
func (esg *expressionSQLGenerator) literalUint(b sqlbuilder.SQLBuilder, i uint64) {
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
func (esg *expressionSQLGenerator) literalString(b sqlbuilder.SQLBuilder, s string) {
	if b.IsPrepared() {
		esg.placeHolderSQL(b, s)
		return
	}
	if !utf8.ValidString(s) {
		b.SetError(errInvalidUTF8Literal(s))
		return
	}
	esg.quotedStringSQL(b, s)
}

func (esg *expressionSQLGenerator) quotedStringSQL(b sqlbuilder.SQLBuilder, s string) {
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
func (esg *expressionSQLGenerator) literalBytes(b sqlbuilder.SQLBuilder, bs []byte) {
	if b.IsPrepared() {
		esg.placeHolderSQL(b, bs)
		return
	}
	if !utf8.Valid(bs) {
		b.SetError(errInvalidUTF8Literal(bs))
		return
	}
	b.GrowBuffer(len(bs) + quoteOverhead)
	b.WriteRunes(esg.dialectOptions.StringQuote)
	for len(bs) > 0 {
		char, l := utf8.DecodeRune(bs)
		if e, ok := esg.dialectOptions.EscapedRunes[char]; ok {
			b.Write(e)
		} else {
			b.WriteRunes(char)
		}
		bs = bs[l:]
	}
	b.WriteRunes(esg.dialectOptions.StringQuote)
}

func (esg *expressionSQLGenerator) sliceValueSQL(b sqlbuilder.SQLBuilder, slice reflect.Value) {
	if slice.Len() == 0 {
		b.SetError(errEmptyListValue(slice.Interface()))
		return
	}
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
func (esg *expressionSQLGenerator) aliasedExpressionSQL(b sqlbuilder.SQLBuilder, aliased exp.AliasedExpression) {
	esg.Generate(b, aliased.Aliased())
	b.Write(esg.dialectOptions.AsFragment)
	esg.Generate(b, aliased.GetAs())
}

func (esg *expressionSQLGenerator) booleanExpressionSQL(b sqlbuilder.SQLBuilder, operator exp.BooleanExpression) {
	operatorOp := operator.Op()
	if (operatorOp == exp.InOp || operatorOp == exp.NotInOp) && isEmptyList(operator.RHS()) {
		esg.emptyListSQL(b, operator)
		return
	}
	if err := booleanOperandsError(operator); err != nil {
		b.SetError(err)
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

func booleanOperandsError(operator exp.BooleanExpression) error {
	op, rhs := operator.Op(), operator.RHS()
	if op == exp.InOp || op == exp.NotInOp {
		if !isInRHS(rhs) {
			return errInWithoutList(op, rhs)
		}
		return nil
	}
	if _, rowValue := operator.LHS().(exp.LiteralExpression); !rowValue && isNotSingleValueList(rhs) {
		return errListAsSingleValue(fmt.Sprintf("operator %q", op), rhs)
	}
	return nil
}

func isNotSingleValueList(v interface{}) bool {
	return util.IsListValue(v) && reflect.Indirect(reflect.ValueOf(v)).Len() != 1
}

func isInRHS(rhs interface{}) bool {
	switch rhs.(type) {
	case exp.AppendableExpression, exp.LiteralExpression:
		return true
	default:
		return util.IsListValue(rhs)
	}
}

func firstNonSingleValueList(vals ...interface{}) (interface{}, bool) {
	for _, v := range vals {
		if isNotSingleValueList(v) {
			return v, true
		}
	}
	return nil, false
}

func isEmptyList(rhs interface{}) bool {
	if !util.IsListValue(rhs) {
		return false
	}
	return reflect.Indirect(reflect.ValueOf(rhs)).Len() == 0
}

func (esg *expressionSQLGenerator) emptyListSQL(b sqlbuilder.SQLBuilder, operator exp.BooleanExpression) {
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

func (esg *expressionSQLGenerator) lhsRenders(b sqlbuilder.SQLBuilder, lhs exp.Expression) bool {
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

func (esg *expressionSQLGenerator) likeEscapeSQL(b sqlbuilder.SQLBuilder, operatorOp exp.BooleanOperation) {
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
func (esg *expressionSQLGenerator) bitwiseExpressionSQL(b sqlbuilder.SQLBuilder, operator exp.BitwiseExpression) {
	if v, bad := firstNonSingleValueList(operator.LHS(), operator.RHS()); bad {
		b.SetError(errListAsSingleValue(operatorPosition(esg.dialectOptions.BitwiseOperatorLookup[operator.Op()], operator.Op()), v))
		return
	}
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
func (esg *expressionSQLGenerator) arithmeticExpressionSQL(b sqlbuilder.SQLBuilder, operator exp.ArithmeticExpression) {
	if v, bad := firstNonSingleValueList(operator.LHS(), operator.RHS()); bad {
		b.SetError(errListAsSingleValue(operatorPosition(esg.dialectOptions.ArithmeticOperatorLookup[operator.Op()], operator.Op()), v))
		return
	}
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
func (esg *expressionSQLGenerator) rangeExpressionSQL(b sqlbuilder.SQLBuilder, operator exp.RangeExpression) {
	if v, bad := firstNonSingleValueList(operator.RHS().Start(), operator.RHS().End()); bad {
		b.SetError(errListAsSingleValue(operatorPosition(esg.dialectOptions.RangeOperatorLookup[operator.Op()], operator.Op()), v))
		return
	}
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
func (esg *expressionSQLGenerator) orderedExpressionSQL(b sqlbuilder.SQLBuilder, order exp.OrderedExpression) {
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
func (esg *expressionSQLGenerator) expressionListSQL(b sqlbuilder.SQLBuilder, expressionList exp.ExpressionList) {
	if expressionList.IsEmpty() {
		if expressionList.Type() == exp.OrType {
			b.SetError(errEmptyOrCondition)
		} else {
			b.SetError(errEmptyAndCondition)
		}
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
func (esg *expressionSQLGenerator) columnListSQL(b sqlbuilder.SQLBuilder, columnList exp.ColumnListExpression) {
	cols := columnList.Columns()
	colLen := len(cols)
	for i, col := range cols {
		esg.Generate(b, col)
		if i < colLen-1 {
			b.WriteRunes(esg.dialectOptions.CommaRune, esg.dialectOptions.SpaceRune)
		}
	}
}

func (esg *expressionSQLGenerator) structColumnsSQL(b sqlbuilder.SQLBuilder, sc exp.StructColumnsExpression) {
	cols := exp.NewColumnListExpression(nil, esg.tagName, sc.Struct())
	if cols.IsEmpty() {
		b.SetError(errStructWithoutColumns(sc.Struct()))
		return
	}
	esg.columnListSQL(b, cols)
}

// Generates SQL for an UpdateEpxresion
func (esg *expressionSQLGenerator) updateExpressionSQL(b sqlbuilder.SQLBuilder, update exp.UpdateExpression) {
	esg.Generate(b, update.Col())
	b.WriteRunes(esg.dialectOptions.SetOperatorRune)
	writeValue(esg, esg.dialectOptions, b, update.Val())
}

func writeValue(esg ExpressionSQLGenerator, opts *SQLDialectOptions, b sqlbuilder.SQLBuilder, val interface{}) {
	switch {
	case isDefault(val) && len(opts.DefaultFragment) == 0:
		b.SetError(errDefaultValueNotSupported(esg.Dialect()))
	case isDefault(val):
		b.Write(opts.DefaultFragment)
	case util.IsListValue(val):
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
func (esg *expressionSQLGenerator) literalExpressionSQL(b sqlbuilder.SQLBuilder, literal exp.LiteralExpression) {
	l := literal.Literal()
	args := literal.Args()
	if argsLen := len(args); argsLen > 0 {
		currIndex := 0
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

func (esg *expressionSQLGenerator) sqlFunctionExpressionSQL(b sqlbuilder.SQLBuilder, sqlFunc exp.SQLFunctionExpression) {
	name := sqlFunc.Name()
	if val, ok := esg.dialectOptions.FunctionNameLookup[name]; ok {
		name = string(val)
	}
	args := sqlFunc.Args()
	if isAnyOrAll(sqlFunc.Name()) && len(args) == 1 && util.IsListValue(args[0]) {
		b.SetError(errListForAnyAll(sqlFunc.Name(), args[0]))
		return
	}
	if v, bad := firstNonSingleValueList(args...); bad {
		b.SetError(errListAsSingleValue(fmt.Sprintf("function %q argument", strings.TrimSpace(name)), v))
		return
	}
	if sqlFunc.HasWithinGroup() {
		if err := esg.withinGroupError(sqlFunc); err != nil {
			b.SetError(err)
			return
		}
	}
	b.WriteStrings(name)
	if sqlFunc.HasOrder() || sqlFunc.HasSeparator() {
		esg.clausedArgumentsSQL(b, args, sqlFunc)
	} else {
		esg.argumentsSQL(b, args)
	}
	if sqlFunc.HasWithinGroup() {
		b.WriteStrings(" WITHIN GROUP ")
		b.WriteRunes(esg.dialectOptions.LeftParenRune)
		b.Write(esg.dialectOptions.WindowOrderByFragment)
		esg.Generate(b, sqlFunc.WithinGroupCols())
		b.WriteRunes(esg.dialectOptions.RightParenRune)
	}
}

func (esg *expressionSQLGenerator) withinGroupError(sqlFunc exp.SQLFunctionExpression) error {
	switch {
	case !esg.dialectOptions.SupportsWithinGroup:
		return errWithinGroupNotSupported(esg.dialect)
	case sqlFunc.HasOrder():
		return errOrderByWithWithinGroup
	}
	return nil
}

func (esg *expressionSQLGenerator) clausedArgumentsSQL(b sqlbuilder.SQLBuilder, args []interface{}, sqlFunc exp.SQLFunctionExpression) {
	switch {
	case sqlFunc.HasOrder() && !esg.dialectOptions.SupportsAggregateOrderBy:
		b.SetError(errAggregateOrderByNotSupported(esg.dialect))
		return
	case sqlFunc.HasSeparator() && !esg.dialectOptions.SupportsAggregateSeparator:
		b.SetError(errAggregateSeparatorNotSupported(esg.dialect))
		return
	}
	b.WriteRunes(esg.dialectOptions.LeftParenRune)
	for i, arg := range args {
		if i > 0 {
			b.WriteRunes(esg.dialectOptions.CommaRune, esg.dialectOptions.SpaceRune)
		}
		esg.Generate(b, arg)
	}
	clauseStarted := len(args) > 0
	if sqlFunc.HasOrder() {
		if clauseStarted {
			b.WriteRunes(esg.dialectOptions.SpaceRune)
		}
		b.Write(esg.dialectOptions.WindowOrderByFragment)
		esg.Generate(b, sqlFunc.OrderCols())
		clauseStarted = true
	}
	if sqlFunc.HasSeparator() {
		if clauseStarted {
			b.WriteRunes(esg.dialectOptions.SpaceRune)
		}
		separator := sqlFunc.GetSeparator()
		if !utf8.ValidString(separator) {
			b.SetError(errInvalidUTF8Separator)
			return
		}
		b.WriteStrings("SEPARATOR ")
		esg.quotedStringSQL(b, separator)
	}
	b.WriteRunes(esg.dialectOptions.RightParenRune)
}

func (esg *expressionSQLGenerator) argumentsSQL(b sqlbuilder.SQLBuilder, args []interface{}) {
	if len(args) == 0 {
		b.WriteRunes(esg.dialectOptions.LeftParenRune, esg.dialectOptions.RightParenRune)
		return
	}
	sliceSQL(esg, b, args, sliceElemHint)
}

func (esg *expressionSQLGenerator) sqlWindowFunctionExpression(b sqlbuilder.SQLBuilder, sqlWinFunc exp.SQLWindowFunctionExpression) {
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
		esg.Generate(b, exp.NewWindowExpression(nil, nil, nil, nil))
	}
}

func (esg *expressionSQLGenerator) windowExpressionSQL(b sqlbuilder.SQLBuilder, we exp.WindowExpression) {
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
		if we.PartitionCols().IsEmpty() {
			b.SetError(errEmptyPartitionBy)
			return
		}
		b.Write(esg.dialectOptions.WindowPartitionByFragment)
		esg.Generate(b, we.PartitionCols())
		if hasOrder {
			b.WriteRunes(esg.dialectOptions.SpaceRune)
		}
	}
	if hasOrder {
		if we.OrderCols().IsEmpty() {
			b.SetError(errEmptyWindowOrderBy)
			return
		}
		b.Write(esg.dialectOptions.WindowOrderByFragment)
		esg.Generate(b, we.OrderCols())
	}

	b.WriteRunes(esg.dialectOptions.RightParenRune)
}

// Generates SQL for a CastExpression
//
//	I("a").Cast("NUMERIC") -> CAST("a" AS NUMERIC)
func (esg *expressionSQLGenerator) castExpressionSQL(b sqlbuilder.SQLBuilder, cast exp.CastExpression) {
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
func (esg *expressionSQLGenerator) collateExpressionSQL(b sqlbuilder.SQLBuilder, collate exp.CollateExpression) {
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
	{exp.NanosecondInterval, ""},
	{exp.MicrosecondInterval, "1 MICROSECOND"},
	{exp.MillisecondInterval, "1 MILLISECOND"},
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
		resolved, err := util.ValuerValue(v)
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
	millisecondFSP   = 3
	microsecondFSP   = 6
	nanosecondDigits = 9
)

type timeRenderers struct {
	now      func(b sqlbuilder.SQLBuilder, utc bool, fsp int, hasFSP bool) error
	interval func(b sqlbuilder.SQLBuilder, e exp.IntervalExpression) error
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
	case GreptimeDBTimeStyle:
		return timeRenderers{now: esg.greptimeDBTimeSQL, interval: esg.greptimeDBIntervalSQL}, nil
	default:
		return timeRenderers{}, errUnknownTimeStyle(esg.dialect, esg.dialectOptions.TimeStyle)
	}
}

func (esg *expressionSQLGenerator) timeFunctionExpressionSQL(b sqlbuilder.SQLBuilder, e exp.TimeFunctionExpression) {
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

func (esg *expressionSQLGenerator) parenthesizedSQL(b sqlbuilder.SQLBuilder, e exp.Expression) {
	b.WriteRunes(esg.dialectOptions.LeftParenRune)
	esg.Generate(b, e)
	b.WriteRunes(esg.dialectOptions.RightParenRune)
}

func (esg *expressionSQLGenerator) postgresTimeSQL(b sqlbuilder.SQLBuilder, utc bool, fsp int, hasFSP bool) error {
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

func (esg *expressionSQLGenerator) mysqlTimeSQL(b sqlbuilder.SQLBuilder, utc bool, fsp int, hasFSP bool) error {
	name := "NOW"
	if utc {
		name = "UTC_TIMESTAMP"
	}
	esg.Generate(b, timeFunc(name, fsp, hasFSP))
	return nil
}

func (esg *expressionSQLGenerator) sqliteTimeSQL(b sqlbuilder.SQLBuilder, utc bool, fsp int, hasFSP bool) error {
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

func (esg *expressionSQLGenerator) sqlServerTimeSQL(b sqlbuilder.SQLBuilder, utc bool, fsp int, hasFSP bool) error {
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

func (esg *expressionSQLGenerator) duckDBTimeSQL(b sqlbuilder.SQLBuilder, utc bool, fsp int, hasFSP bool) error {
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

func (esg *expressionSQLGenerator) greptimeDBTimeSQL(b sqlbuilder.SQLBuilder, utc bool, fsp int, hasFSP bool) error {
	switch {
	case utc:
		return errUTCTimestampNotSupported(esg.dialect)
	case hasFSP:
		return errTimeFSPNotSupported(esg.dialect, fsp)
	}
	esg.Generate(b, exp.NewSQLFunctionExpression("NOW"))
	return nil
}

func supportsRangeQuery(opts *SQLDialectOptions) bool {
	return opts.SupportsRangeQuery && slices.Contains(opts.SelectSQLOrder, AlignSQLFragment)
}

func (esg *expressionSQLGenerator) rangeAggregateSQL(b sqlbuilder.SQLBuilder, e exp.RangeAggregateExpression) {
	if !supportsRangeQuery(esg.dialectOptions) {
		b.SetError(errRangeQueryNotSupported(esg.dialect))
		return
	}
	b.WriteRunes(esg.dialectOptions.LeftParenRune)
	esg.Generate(b, e.Aggregate())
	b.WriteStrings(" RANGE ")
	esg.rangeDurationSQL(b, "RANGE", e.Duration())
	if e.HasFill() {
		esg.rangeFillSQL(b, e.GetFill())
	}
	b.WriteRunes(esg.dialectOptions.RightParenRune)
}

func (esg *expressionSQLGenerator) alignExpressionSQL(b sqlbuilder.SQLBuilder, a exp.AlignExpression) {
	if !supportsRangeQuery(esg.dialectOptions) {
		b.SetError(errRangeQueryNotSupported(esg.dialect))
		return
	}
	b.WriteStrings("ALIGN ")
	esg.rangeDurationSQL(b, "ALIGN", a.Duration())
	if a.HasTo() {
		esg.alignToSQL(b, a)
	}
	if a.HasBy() {
		b.WriteStrings(" BY ")
		b.WriteRunes(esg.dialectOptions.LeftParenRune)
		if !a.ByCols().IsEmpty() {
			esg.Generate(b, a.ByCols())
		}
		b.WriteRunes(esg.dialectOptions.RightParenRune)
	}
	if a.HasFill() {
		esg.rangeFillSQL(b, a.GetFill())
	}
}

func (esg *expressionSQLGenerator) alignToSQL(b sqlbuilder.SQLBuilder, a exp.AlignExpression) {
	if a.IsToNow() {
		b.WriteStrings(" TO NOW")
		return
	}
	origin := a.ToTime()
	if origin.Nanosecond()%int(time.Millisecond) != 0 {
		b.SetError(errAlignOrigin(origin))
		return
	}
	b.WriteStrings(" TO ")
	b.WriteRunes(esg.dialectOptions.StringQuote)
	b.WriteStrings(origin.In(timeLocation).Format(esg.dialectOptions.TimeFormat))
	b.WriteRunes(esg.dialectOptions.StringQuote)
}

var rangeDurationUnits = []struct {
	unit time.Duration
	name string
}{
	{time.Hour, "h"},
	{time.Minute, "m"},
	{time.Second, "s"},
	{time.Millisecond, "ms"},
}

func (esg *expressionSQLGenerator) rangeDurationSQL(b sqlbuilder.SQLBuilder, clause string, d time.Duration) {
	if d <= 0 || d%time.Millisecond != 0 {
		b.SetError(errRangeDuration(clause, d))
		return
	}
	b.WriteRunes(esg.dialectOptions.StringQuote)
	for _, u := range rangeDurationUnits {
		if n := d / u.unit; n > 0 {
			b.WriteInt(int64(n)).WriteStrings(u.name)
			d -= n * u.unit
		}
	}
	b.WriteRunes(esg.dialectOptions.StringQuote)
}

func (esg *expressionSQLGenerator) rangeFillSQL(b sqlbuilder.SQLBuilder, fill exp.RangeFill) {
	switch fill.Kind() {
	case exp.RangeFillNull:
		b.WriteStrings(" FILL NULL")
	case exp.RangeFillPrev:
		b.WriteStrings(" FILL PREV")
	case exp.RangeFillLinear:
		b.WriteStrings(" FILL LINEAR")
	case exp.RangeFillValue:
		v := fill.Value()
		if math.Signbit(v) || math.IsNaN(v) || math.IsInf(v, 0) {
			b.SetError(errRangeFillValue(v))
			return
		}
		b.WriteStrings(" FILL ").WriteFloat(v)
	default:
		b.SetError(errUnknownRangeFill(fill.Kind()))
	}
}

type intervalConversion struct {
	unit       exp.IntervalUnit
	multiplier int64
	divisor    uint16
	inSQL      bool
}

const (
	monthsPerQuarter           = 3
	microsecondsPerMillisecond = int64(time.Millisecond / time.Microsecond)
	nanosecondsPerMicrosecond  = uint16(time.Microsecond / time.Nanosecond)
)

var nanosecondsAsMicroseconds = intervalConversion{unit: exp.MicrosecondInterval, multiplier: 1, divisor: nanosecondsPerMicrosecond}

var intervalConversions = map[TimeStyle]map[exp.IntervalUnit]intervalConversion{
	PostgresTimeStyle: {exp.NanosecondInterval: nanosecondsAsMicroseconds},
	MySQLTimeStyle: {
		exp.NanosecondInterval:  nanosecondsAsMicroseconds,
		exp.MillisecondInterval: {unit: exp.MicrosecondInterval, multiplier: microsecondsPerMillisecond, divisor: 1, inSQL: true},
	},
	DuckDBTimeStyle:     {exp.NanosecondInterval: nanosecondsAsMicroseconds},
	GreptimeDBTimeStyle: {exp.QuarterInterval: {unit: exp.MonthInterval, multiplier: monthsPerQuarter, divisor: 1}},
}

var fractionalSecondDigits = map[TimeStyle]struct {
	digits int
	unit   exp.IntervalUnit
}{
	PostgresTimeStyle:   {digits: microsecondFSP, unit: exp.MicrosecondInterval},
	MySQLTimeStyle:      {digits: microsecondFSP, unit: exp.MicrosecondInterval},
	GreptimeDBTimeStyle: {digits: nanosecondDigits, unit: exp.NanosecondInterval},
}

func (esg *expressionSQLGenerator) intervalExpressionSQL(b sqlbuilder.SQLBuilder, e exp.IntervalExpression) {
	render, err := esg.timeRenderers()
	if err == nil {
		e, err = esg.exactInterval(e)
	}
	if err == nil {
		err = render.interval(b, e)
	}
	if err != nil {
		b.SetError(err)
	}
}

func (esg *expressionSQLGenerator) exactInterval(e exp.IntervalExpression) (exp.IntervalExpression, error) {
	style := esg.dialectOptions.TimeStyle
	conversions, converts := intervalConversions[style]
	fraction, limitsFraction := fractionalSecondDigits[style]
	if !converts && !limitsFraction {
		return e, nil
	}
	unit, _, err := lookupIntervalUnit(e.Unit())
	if err != nil {
		return nil, err
	}
	if conversion, ok := conversions[unit]; ok {
		return esg.convertIntervalUnit(e, unit, conversion)
	}
	if unit != exp.SecondInterval || !limitsFraction {
		return e, nil
	}
	count, err := intervalCount(esg.dialect, e.Count(), unit, true)
	if err != nil {
		return nil, err
	}
	if rv := reflect.Indirect(reflect.ValueOf(count)); rv.IsValid() && util.IsFloat(rv.Kind()) {
		text := strconv.FormatFloat(rv.Float(), 'f', -1, 64)
		if point := strings.IndexByte(text, '.'); point >= 0 && len(text)-point-1 > fraction.digits {
			return nil, errIntervalNotExact(esg.dialect, text, unit, fraction.unit)
		}
	}
	return exp.NewIntervalExpression(count, unit), nil
}

func (esg *expressionSQLGenerator) convertIntervalUnit(
	e exp.IntervalExpression, unit exp.IntervalUnit, conversion intervalConversion,
) (exp.IntervalExpression, error) {
	count, err := intervalCount(esg.dialect, e.Count(), unit, false)
	if err != nil {
		return nil, err
	}
	rv := reflect.Indirect(reflect.ValueOf(count))
	notExact := func() error { return errIntervalNotExact(esg.dialect, rv.Interface(), unit, conversion.unit) }
	var quotient int64
	switch {
	case !rv.IsValid():
		return exp.NewIntervalExpression(nil, conversion.unit), nil
	case util.IsInt(rv.Kind()):
		n, divisor := rv.Int(), int64(conversion.divisor)
		if n%divisor != 0 {
			return nil, notExact()
		}
		quotient = n / divisor
	case util.IsUint(rv.Kind()):
		u, divisor := rv.Uint(), uint64(conversion.divisor)
		if u%divisor != 0 {
			return nil, notExact()
		}
		q := u / divisor
		if q > math.MaxInt64 {
			return nil, notExact()
		}
		quotient = int64(q)
	case conversion.inSQL:
		return exp.NewIntervalExpression(count, unit), nil
	default:
		return nil, errIntervalConversionNeedsCount(esg.dialect, unit, conversion.unit)
	}
	if quotient > math.MaxInt64/conversion.multiplier || quotient < math.MinInt64/conversion.multiplier {
		return nil, notExact()
	}
	return exp.NewIntervalExpression(quotient*conversion.multiplier, conversion.unit), nil
}

func (esg *expressionSQLGenerator) mysqlIntervalSQL(b sqlbuilder.SQLBuilder, e exp.IntervalExpression) error {
	unit, _, count, err := intervalUnit(esg.dialect, e, true)
	if err != nil {
		return err
	}
	if unit == exp.MillisecondInterval {
		b.WriteStrings("INTERVAL ")
		b.WriteRunes(esg.dialectOptions.LeftParenRune)
		if literal, isLiteral := count.(exp.LiteralExpression); isLiteral {
			esg.parenthesizedSQL(b, literal)
		} else {
			esg.Generate(b, count)
		}
		b.WriteStrings(" * ").WriteInt(microsecondsPerMillisecond)
		b.WriteRunes(esg.dialectOptions.RightParenRune)
		b.WriteStrings(" " + string(exp.MicrosecondInterval))
		return nil
	}
	esg.Generate(b, exp.NewLiteralExpression("INTERVAL ? "+string(unit), count))
	return nil
}

func (esg *expressionSQLGenerator) greptimeDBIntervalSQL(b sqlbuilder.SQLBuilder, e exp.IntervalExpression) error {
	unit, _, count, err := intervalUnit(esg.dialect, e, true)
	if err != nil {
		return err
	}
	if expression, isExpression := count.(exp.Expression); isExpression {
		esg.Generate(b, exp.NewLiteralExpression("INTERVAL ? "+string(unit), expression))
		return nil
	}
	b.WriteStrings("INTERVAL ")
	esg.inlineNumberSQL(b, count)
	b.WriteStrings(" " + string(unit))
	return nil
}

func (esg *expressionSQLGenerator) inlineNumberSQL(b sqlbuilder.SQLBuilder, n any) {
	rv := reflect.Indirect(reflect.ValueOf(n))
	switch {
	case !rv.IsValid():
		b.Write(esg.dialectOptions.Null)
	case util.IsInt(rv.Kind()):
		b.WriteInt(rv.Int())
	case util.IsUint(rv.Kind()):
		b.WriteStrings(strconv.FormatUint(rv.Uint(), 10))
	default:
		b.WriteFloat(rv.Float())
	}
}

func (esg *expressionSQLGenerator) postgresIntervalSQL(b sqlbuilder.SQLBuilder, e exp.IntervalExpression) error {
	return esg.multipliedIntervalSQL(b, e, true)
}

func (esg *expressionSQLGenerator) duckDBIntervalSQL(b sqlbuilder.SQLBuilder, e exp.IntervalExpression) error {
	return esg.multipliedIntervalSQL(b, e, false)
}

func (esg *expressionSQLGenerator) multipliedIntervalSQL(b sqlbuilder.SQLBuilder, e exp.IntervalExpression, fractionalSeconds bool) error {
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

func (esg *expressionSQLGenerator) unsupportedIntervalSQL(sqlbuilder.SQLBuilder, exp.IntervalExpression) error {
	return errIntervalNotSupported(esg.dialect)
}

func (esg *expressionSQLGenerator) jsonExpressionSQL(b sqlbuilder.SQLBuilder, e exp.JSONExpression) {
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
			value, err := util.ValuerValue(valuer)
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

func (esg *expressionSQLGenerator) excludedExpressionSQL(b sqlbuilder.SQLBuilder, e exp.ExcludedExpression) {
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
func (esg *expressionSQLGenerator) commonTablesSliceSQL(b sqlbuilder.SQLBuilder, ctes []exp.CommonTableExpression) {
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
func (esg *expressionSQLGenerator) commonTableExpressionSQL(b sqlbuilder.SQLBuilder, cte exp.CommonTableExpression) {
	if strings.TrimSpace(cte.Name().Literal()) == "" {
		b.SetError(errEmptyCTEName)
		return
	}
	esg.Generate(b, cte.Name())
	b.Write(esg.dialectOptions.AsFragment)
	esg.Generate(b, cte.SubQuery())
}

// Generates SQL for a CompoundExpression
func (esg *expressionSQLGenerator) compoundExpressionSQL(b sqlbuilder.SQLBuilder, compound exp.CompoundExpression) {
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
	if util.IsNilOrNilPointer(compound.RHS()) {
		b.SetError(errNilSubquery(compound.RHS()))
		return
	}
	if !esg.dialectOptions.SupportsNestedWithCTE && hasCommonTables(compound.RHS()) {
		b.SetError(errNestedCTENotSupported(esg.dialect))
		return
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
func (esg *expressionSQLGenerator) caseExpressionSQL(b sqlbuilder.SQLBuilder, caseExpression exp.CaseExpression) {
	caseVal := caseExpression.GetValue()
	whens := caseExpression.GetWhens()
	elseResult := caseExpression.GetElse()

	if len(whens) == 0 {
		b.SetError(errEmptyCaseWhens)
		return
	}
	if err := caseValuesError(whens, elseResult); err != nil {
		b.SetError(err)
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

func (esg *expressionSQLGenerator) expressionMapSQL(b sqlbuilder.SQLBuilder, ex exp.Ex) {
	expressionList, err := ex.ToExpressions()
	if err != nil {
		b.SetError(err)
		return
	}
	esg.Generate(b, expressionList)
}

func (esg *expressionSQLGenerator) expressionOrMapSQL(b sqlbuilder.SQLBuilder, ex exp.ExOr) {
	expressionList, err := ex.ToExpressions()
	if err != nil {
		b.SetError(err)
		return
	}
	esg.Generate(b, expressionList)
}

func hasCommonTables(a exp.AppendableExpression) bool {
	switch ds := a.(type) {
	case interface{ GetClauses() exp.SelectClauses }:
		return len(ds.GetClauses().CommonTables()) > 0
	case interface{ GetClauses() exp.InsertClauses }:
		return len(ds.GetClauses().CommonTables()) > 0
	case interface{ GetClauses() exp.UpdateClauses }:
		return len(ds.GetClauses().CommonTables()) > 0
	case interface{ GetClauses() exp.DeleteClauses }:
		return len(ds.GetClauses().CommonTables()) > 0
	default:
		return false
	}
}

func isAnyOrAll(name string) bool {
	return name == exp.AnyFunctionName || name == exp.AllFunctionName
}

func operatorPosition(sql []byte, op fmt.Stringer) string {
	if name := strings.TrimSpace(string(sql)); name != "" {
		return fmt.Sprintf("operator %q", name)
	}
	return fmt.Sprintf("operator %q", op.String())
}

func caseValuesError(whens []exp.CaseWhen, elseResult exp.CaseElse) error {
	for _, when := range whens {
		if c := when.Condition(); isNotSingleValueList(c) {
			return errListAsSingleValue("CASE WHEN", c)
		}
		if r := when.Result(); isNotSingleValueList(r) {
			return errListAsSingleValue("CASE THEN", r)
		}
	}
	if elseResult == nil {
		return nil
	}
	if r := elseResult.Result(); isNotSingleValueList(r) {
		return errListAsSingleValue("CASE ELSE", r)
	}
	return nil
}
