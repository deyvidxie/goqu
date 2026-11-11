package sqlgen_test

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/sb"
	"github.com/doug-martin/goqu/v9/sqlgen"
	"github.com/stretchr/testify/suite"
)

var emptyArgs = make([]interface{}, 0)

type testAppendableExpression struct {
	sql            string
	args           []interface{}
	err            error
	alias          exp.IdentifierExpression
	returnsColumns bool
}

func newTestAppendableExpression(
	sql string,
	args []interface{},
	err error,
	alias exp.IdentifierExpression) exp.AppendableExpression {
	return &testAppendableExpression{sql: sql, args: args, err: err, alias: alias}
}

func (tae *testAppendableExpression) Expression() exp.Expression {
	return tae
}

func (tae *testAppendableExpression) Clone() exp.Expression {
	return tae
}

func (tae *testAppendableExpression) GetAs() exp.IdentifierExpression {
	return tae.alias
}

func (tae *testAppendableExpression) ReturnsColumns() bool {
	return tae.returnsColumns
}

func (tae *testAppendableExpression) AppendSQL(b sb.SQLBuilder) {
	if tae.err != nil {
		b.SetError(tae.err)
		return
	}
	b.WriteStrings(tae.sql)
	if len(tae.args) > 0 {
		b.WriteArg(tae.args...)
	}
}

type (
	expressionTestCase struct {
		val        interface{}
		sql        string
		err        string
		isPrepared bool
		args       []interface{}
	}
	expressionSQLGeneratorSuite struct {
		suite.Suite
	}
)

func (esgs *expressionSQLGeneratorSuite) assertCases(esg sqlgen.ExpressionSQLGenerator, cases ...expressionTestCase) {
	for i, c := range cases {
		b := sb.NewSQLBuilder(c.isPrepared)
		esg.Generate(b, c.val)
		actualSQL, actualArgs, err := b.ToSQL()
		if c.err == "" {
			esgs.NoError(err, "test case %d failed", i)
		} else {
			esgs.EqualError(err, c.err, "test case %d failed", i)
		}
		esgs.Equal(c.sql, actualSQL, "test case %d failed", i)
		if c.isPrepared && c.args != nil || len(c.args) > 0 {
			esgs.Equal(c.args, actualArgs, "test case %d failed", i)
		} else {
			esgs.Empty(actualArgs, "test case %d failed", i)
		}
	}
}

func (esgs *expressionSQLGeneratorSuite) TestDialect() {
	esg := sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions())
	esgs.Equal("test", esg.Dialect())
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ErroredBuilder() {
	esg := sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions())
	expectedErr := newTestError("test error")
	b := sb.NewSQLBuilder(false).SetError(expectedErr)
	esg.Generate(b, 1)
	sql, args, err := b.ToSQL()
	esgs.Equal(expectedErr, err)
	esgs.Empty(sql)
	esgs.Empty(args)

	b = sb.NewSQLBuilder(true).SetError(err)
	esg.Generate(b, true)
	sql, args, err = b.ToSQL()
	esgs.Equal(expectedErr, err)
	esgs.Empty(sql)
	esgs.Empty(args)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_Invalid() {
	var b *bool
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: b, sql: "NULL"},
		expressionTestCase{val: b, sql: "?", isPrepared: true, args: []interface{}{nil}},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_OtherType() {
	type strct struct{}
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: strct{}, err: "goqu: unable to encode value {}"},
		expressionTestCase{val: strct{}, sql: "?", isPrepared: true, args: []interface{}{strct{}}},
		expressionTestCase{val: map[string]int{"a": 1}, err: "goqu: unable to encode value map[a:1]"},
		expressionTestCase{
			val: map[string]int{"a": 1}, sql: "?", isPrepared: true, args: []interface{}{map[string]int{"a": 1}},
		},
		expressionTestCase{val: [2]byte{1, 2}, err: "goqu: unable to encode value [1 2]"},
		expressionTestCase{val: [2]byte{1, 2}, sql: "?", isPrepared: true, args: []interface{}{[2]byte{1, 2}}},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_UnbindableType() {
	// no driver can bind these, so they are rejected even in prepared mode
	for _, val := range []interface{}{func() {}, make(chan int), complex(1, 2)} {
		esgs.assertCases(
			sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
			expressionTestCase{val: val, err: fmt.Sprintf("goqu: unable to encode value %+v", val), isPrepared: true},
		)
	}
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_IncludePlaceholderNum() {
	opts := sqlgen.DefaultDialectOptions()
	opts.IncludePlaceholderNum = true
	opts.PlaceHolderFragment = []byte("$")
	ex := exp.Ex{
		"a": 1,
		"b": true,
		"c": false,
		"d": []string{"a", "b", "c"},
	}
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", opts),
		expressionTestCase{
			val: ex,
			sql: `(("a" = 1) AND ("b" IS TRUE) AND ("c" IS FALSE) AND ("d" IN ('a', 'b', 'c')))`,
		},
		expressionTestCase{
			val:        ex,
			sql:        `(("a" = $1) AND ("b" IS TRUE) AND ("c" IS FALSE) AND ("d" IN ($2, $3, $4)))`,
			isPrepared: true,
			args:       []interface{}{int64(1), "a", "b", "c"},
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_FloatTypes() {
	var float float64
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: float32(10.01), sql: "10.010000228881836"},
		expressionTestCase{val: float32(10.01), sql: "?", isPrepared: true, args: []interface{}{float64(float32(10.01))}},

		expressionTestCase{val: float64(10.01), sql: "10.01"},
		expressionTestCase{val: float64(10.01), sql: "?", isPrepared: true, args: []interface{}{float64(10.01)}},

		expressionTestCase{val: &float, sql: "0"},
		expressionTestCase{val: &float, sql: "?", isPrepared: true, args: []interface{}{float}},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_IntTypes() {
	var i int64
	ints := []interface{}{
		int(10),
		int16(10),
		int32(10),
		int64(10),
	}

	for _, i := range ints {
		esgs.assertCases(
			sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
			expressionTestCase{val: i, sql: "10"},
			expressionTestCase{val: i, sql: "?", isPrepared: true, args: []interface{}{int64(10)}},
		)
	}
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: &i, sql: "0"},
		expressionTestCase{val: &i, sql: "?", isPrepared: true, args: []interface{}{i}},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_UintTypes() {
	var u uint64
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: uint(65535), sql: "65535"},
		expressionTestCase{val: uint(65535), sql: "?", isPrepared: true, args: []interface{}{int64(65535)}},

		expressionTestCase{val: uint16(65535), sql: "65535"},
		expressionTestCase{val: uint16(65535), sql: "?", isPrepared: true, args: []interface{}{int64(65535)}},

		expressionTestCase{val: uint32(4294967295), sql: "4294967295"},
		expressionTestCase{val: uint32(4294967295), sql: "?", isPrepared: true, args: []interface{}{int64(4294967295)}},

		expressionTestCase{val: uint64(18446744073709551615), sql: "18446744073709551615"},
		expressionTestCase{val: uint64(18446744073709551615), sql: "?", isPrepared: true, args: []interface{}{uint64(18446744073709551615)}},

		expressionTestCase{val: &u, sql: "0"},
		expressionTestCase{val: &u, sql: "?", isPrepared: true, args: []interface{}{int64(0)}},
	)
}

// Named unsigned types are not matched by the type switch and go through reflection.
func (esgs *expressionSQLGeneratorSuite) TestGenerate_NamedUintTypes() {
	type id uint64
	type small uint8
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: id(18446744073709551615), sql: "18446744073709551615"},
		expressionTestCase{val: id(18446744073709551615), sql: "?", isPrepared: true, args: []interface{}{uint64(18446744073709551615)}},
		expressionTestCase{val: small(255), sql: "255"},
		expressionTestCase{val: small(255), sql: "?", isPrepared: true, args: []interface{}{int64(255)}},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_StringTypes() {
	var str string
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: "Hello", sql: "'Hello'"},
		expressionTestCase{val: "Hello", sql: "?", isPrepared: true, args: []interface{}{"Hello"}},

		expressionTestCase{val: "Hello'", sql: "'Hello'''"},
		expressionTestCase{val: "Hello'", sql: "?", isPrepared: true, args: []interface{}{"Hello'"}},

		expressionTestCase{val: &str, sql: "''"},
		expressionTestCase{val: &str, sql: "?", isPrepared: true, args: []interface{}{str}},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_BytesTypes() {
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: []byte("Hello"), sql: "'Hello'"},
		expressionTestCase{val: []byte("Hello"), sql: "?", isPrepared: true, args: []interface{}{[]byte("Hello")}},

		expressionTestCase{val: []byte("Hello'"), sql: "'Hello'''"},
		expressionTestCase{val: []byte("Hello'"), sql: "?", isPrepared: true, args: []interface{}{[]byte("Hello'")}},

		expressionTestCase{val: json.RawMessage("Hello"), sql: "'Hello'"},
		expressionTestCase{val: json.RawMessage("Hello"), sql: "?", isPrepared: true, args: []interface{}{[]byte("Hello")}},

		expressionTestCase{val: json.RawMessage("Hello'"), sql: "'Hello'''"},
		expressionTestCase{val: json.RawMessage("Hello'"), sql: "?", isPrepared: true, args: []interface{}{[]byte("Hello'")}},

		expressionTestCase{val: json.RawMessage(nil), sql: "NULL"},
		expressionTestCase{val: json.RawMessage(nil), sql: "?", isPrepared: true, args: []interface{}{nil}},
		expressionTestCase{val: json.RawMessage{}, sql: "''"},

		expressionTestCase{val: ptrTo(json.RawMessage(`{"a":1}`)), sql: `'{"a":1}'`},
		expressionTestCase{
			val: ptrTo(json.RawMessage(`{"a":1}`)), sql: "?", isPrepared: true, args: []interface{}{[]byte(`{"a":1}`)},
		},
		expressionTestCase{val: ptrTo([]byte("Hello")), sql: "'Hello'"},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_BoolTypes() {
	var bl bool
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: true, sql: "TRUE"},
		expressionTestCase{val: true, sql: "?", isPrepared: true, args: []interface{}{true}},

		expressionTestCase{val: false, sql: "FALSE"},
		expressionTestCase{val: false, sql: "?", isPrepared: true, args: []interface{}{false}},

		expressionTestCase{val: &bl, sql: "FALSE"},
		expressionTestCase{val: &bl, sql: "?", isPrepared: true, args: []interface{}{bl}},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_TimeTypes() {
	var nt *time.Time

	ts, err := time.Parse(time.RFC3339, "2019-10-01T15:01:00Z")
	esgs.Require().NoError(err)
	originalLoc := sqlgen.GetTimeLocation()

	loc, err := time.LoadLocation("Asia/Shanghai")
	esgs.Require().NoError(err)

	sqlgen.SetTimeLocation(loc)
	// non time
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: ts, sql: "'2019-10-01T23:01:00+08:00'"},
		expressionTestCase{val: ts, sql: "?", isPrepared: true, args: []interface{}{ts}},

		expressionTestCase{val: &ts, sql: "'2019-10-01T23:01:00+08:00'"},
		expressionTestCase{val: &ts, sql: "?", isPrepared: true, args: []interface{}{ts}},
	)
	sqlgen.SetTimeLocation(time.UTC)
	// utc time
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: ts, sql: "'2019-10-01T15:01:00Z'"},
		expressionTestCase{val: ts, sql: "?", isPrepared: true, args: []interface{}{ts}},

		expressionTestCase{val: &ts, sql: "'2019-10-01T15:01:00Z'"},
		expressionTestCase{val: &ts, sql: "?", isPrepared: true, args: []interface{}{ts}},
	)
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: nt, sql: "NULL"},
		expressionTestCase{val: nt, sql: "?", isPrepared: true, args: []interface{}{nil}},
	)
	sqlgen.SetTimeLocation(originalLoc)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_NilTypes() {
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: nil, sql: "NULL"},
		expressionTestCase{val: nil, sql: "?", isPrepared: true, args: []interface{}{nil}},
	)
}

type pointerValuer struct{}

func (*pointerValuer) Value() (driver.Value, error) { return "pointer", nil }

type datasetValuerType struct {
	int int64
	err error
}

func (j datasetValuerType) Value() (driver.Value, error) {
	if j.err != nil {
		return nil, j.err
	}
	return []byte(fmt.Sprintf("Hello World %d", j.int)), nil
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_Valuer() {
	err := newTestError("valuer error")
	var val *datasetValuerType
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: datasetValuerType{int: 10}, sql: "'Hello World 10'"},
		expressionTestCase{
			val: datasetValuerType{int: 10}, sql: "?", isPrepared: true, args: []interface{}{[]byte("Hello World 10")},
		},

		expressionTestCase{val: datasetValuerType{err: err}, err: "goqu: driver.Valuer sqlgen_test.datasetValuerType: valuer error"},
		expressionTestCase{
			val: datasetValuerType{err: err}, isPrepared: true, err: "goqu: driver.Valuer sqlgen_test.datasetValuerType: valuer error",
		},
		expressionTestCase{
			val: val, sql: "NULL",
		},
		expressionTestCase{val: &datasetValuerType{int: 10}, sql: "'Hello World 10'"},
		expressionTestCase{val: (*pointerValuer)(nil), sql: "'pointer'"},
	)

	b := sb.NewSQLBuilder(false)
	sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()).Generate(b, datasetValuerType{err: err})
	_, _, genErr := b.ToSQL()
	esgs.True(goqu.IsInvalidExpression(genErr))
	esgs.ErrorIs(genErr, err)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_Slice() {
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: []string{"a", "b", "c"}, sql: `('a', 'b', 'c')`},
		expressionTestCase{
			val: []string{"a", "b", "c"}, sql: "(?, ?, ?)", isPrepared: true, args: []interface{}{"a", "b", "c"},
		},

		expressionTestCase{val: []byte{'a', 'b', 'c'}, sql: `'abc'`},
		expressionTestCase{
			val: []byte{'a', 'b', 'c'}, sql: "?", isPrepared: true, args: []interface{}{[]byte{'a', 'b', 'c'}},
		},
	)
}

type unknownExpression struct{}

func (ue unknownExpression) Expression() exp.Expression {
	return ue
}

func (ue unknownExpression) Clone() exp.Expression {
	return ue
}

func (esgs *expressionSQLGeneratorSuite) TestGenerateUnsupportedExpression() {
	errMsg := "goqu: unsupported expression type sqlgen_test.unknownExpression"
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: unknownExpression{}, err: errMsg},
		expressionTestCase{
			val: unknownExpression{}, isPrepared: true, err: errMsg,
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_AppendableExpression() {
	ti := exp.NewIdentifierExpression("", "b", "")
	a := newTestAppendableExpression(`select * from "a"`, []interface{}{}, nil, nil)
	aliasedA := newTestAppendableExpression(`select * from "a"`, []interface{}{}, nil, ti)
	argsA := newTestAppendableExpression(`select * from "a" where x=?`, []interface{}{true}, nil, ti)
	ae := newTestAppendableExpression(`select * from "a"`, emptyArgs, newTestError("expected error"), nil)

	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: a, sql: `(select * from "a")`},
		expressionTestCase{val: a, sql: `(select * from "a")`, isPrepared: true},

		expressionTestCase{val: aliasedA, sql: `(select * from "a") AS "b"`},
		expressionTestCase{val: aliasedA, sql: `(select * from "a") AS "b"`, isPrepared: true},

		expressionTestCase{val: ae, err: "expected error"},
		expressionTestCase{val: ae, err: "expected error", isPrepared: true},

		expressionTestCase{val: argsA, sql: `(select * from "a" where x=?) AS "b"`, args: []interface{}{true}},
		expressionTestCase{val: argsA, sql: `(select * from "a" where x=?) AS "b"`, isPrepared: true, args: []interface{}{true}},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ColumnList() {
	cl := exp.NewColumnListExpression(nil, "db", "a", exp.NewLiteralExpression("true"))
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: cl, sql: `"a", true`},
		expressionTestCase{val: cl, sql: `"a", true`, isPrepared: true},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_EmptyConditions() {
	a := exp.NewIdentifierExpression("", "", "a").Eq(1)
	and := func(e ...exp.Expression) exp.ExpressionList { return exp.NewExpressionList(exp.AndType, e...) }
	or := func(e ...exp.Expression) exp.ExpressionList { return exp.NewExpressionList(exp.OrType, e...) }
	const (
		emptyAnd = `goqu: empty AND condition (And() or Ex{}); write the condition, e.g. L("1=1")`
		emptyOr  = `goqu: empty OR condition (Or() or ExOr{}); write the condition, e.g. L("1=0")`
	)
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: and(), err: emptyAnd},
		expressionTestCase{val: or(), err: emptyOr},
		expressionTestCase{val: exp.Ex{}, err: emptyAnd},
		expressionTestCase{val: exp.ExOr{}, err: emptyOr},
		expressionTestCase{val: and(a, and()), err: emptyAnd},
		expressionTestCase{val: and(a, exp.ExOr{}), err: emptyOr},
		expressionTestCase{val: or(a, exp.Ex{}), err: emptyAnd},
		expressionTestCase{val: or(a, or()), err: emptyOr, isPrepared: true},
		expressionTestCase{val: exp.NewCaseExpression().When(exp.Ex{}, 1), err: emptyAnd},
		expressionTestCase{val: exp.NewLiteralExpression("NOT ?", and()), err: emptyAnd},
		expressionTestCase{val: or(a, exp.NewLiteralExpression("")), sql: `(("a" = 1) OR )`},
		expressionTestCase{val: exp.Ex{"a": 1, "b": exp.Op{}}, err: `goqu: Op for "b" has no operators`},
		expressionTestCase{val: exp.ExOr{"a": exp.Op{}}, err: `goqu: Op for "a" has no operators`},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExpressionList() {
	andEl := exp.NewExpressionList(
		exp.AndType,
		exp.NewIdentifierExpression("", "", "a").Eq("b"),
		exp.NewIdentifierExpression("", "", "c").Neq(1),
	)

	orEl := exp.NewExpressionList(
		exp.OrType,
		exp.NewIdentifierExpression("", "", "a").Eq("b"),
		exp.NewIdentifierExpression("", "", "c").Neq(1),
	)

	andOrEl := exp.NewExpressionList(exp.OrType,
		exp.NewIdentifierExpression("", "", "a").Eq("b"),
		exp.NewExpressionList(exp.AndType,
			exp.NewIdentifierExpression("", "", "c").Neq(1),
			exp.NewIdentifierExpression("", "", "d").Eq(exp.NewLiteralExpression("NOW()")),
		),
	)

	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: andEl, sql: `(("a" = 'b') AND ("c" != 1))`},
		expressionTestCase{
			val: andEl, sql: `(("a" = ?) AND ("c" != ?))`, isPrepared: true, args: []interface{}{"b", int64(1)},
		},

		expressionTestCase{val: orEl, sql: `(("a" = 'b') OR ("c" != 1))`},
		expressionTestCase{
			val: orEl, sql: `(("a" = ?) OR ("c" != ?))`, isPrepared: true, args: []interface{}{"b", int64(1)},
		},

		expressionTestCase{val: andOrEl, sql: `(("a" = 'b') OR (("c" != 1) AND ("d" = NOW())))`},
		expressionTestCase{
			val:        andOrEl,
			sql:        `(("a" = ?) OR (("c" != ?) AND ("d" = NOW())))`,
			isPrepared: true,
			args:       []interface{}{"b", int64(1)},
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_LiteralExpression() {
	noArgsL := exp.NewLiteralExpression(`"b"::DATE = '2010-09-02'`)
	argsL := exp.NewLiteralExpression(`"b" = ? or "c" = ? or d IN ? or "e" = ?`, "a", 1, []int64{1, 2, 3, 4}, uint64(11169823557460058355))

	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: noArgsL, sql: `"b"::DATE = '2010-09-02'`},
		expressionTestCase{val: noArgsL, sql: `"b"::DATE = '2010-09-02'`, isPrepared: true},

		expressionTestCase{val: argsL, sql: `"b" = 'a' or "c" = 1 or d IN (1, 2, 3, 4) or "e" = 11169823557460058355`},
		expressionTestCase{
			val:        argsL,
			sql:        `"b" = ? or "c" = ? or d IN (?, ?, ?, ?) or "e" = ?`,
			isPrepared: true,
			args: []interface{}{
				"a",
				int64(1),
				int64(1),
				int64(2),
				int64(3),
				int64(4),
				uint64(11169823557460058355),
			},
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_AliasedExpression() {
	aliasedI := exp.NewIdentifierExpression("", "", "a").As("b")
	aliasedWithII := exp.NewIdentifierExpression("", "", "a").
		As(exp.NewIdentifierExpression("", "", "b"))
	aliasedL := exp.NewLiteralExpression("count(*)").As("count")

	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: aliasedI, sql: `"a" AS "b"`},
		expressionTestCase{val: aliasedI, sql: `"a" AS "b"`, isPrepared: true},

		expressionTestCase{val: aliasedWithII, sql: `"a" AS "b"`},
		expressionTestCase{val: aliasedWithII, sql: `"a" AS "b"`, isPrepared: true},

		expressionTestCase{val: aliasedL, sql: `count(*) AS "count"`},
		expressionTestCase{val: aliasedL, sql: `count(*) AS "count"`, isPrepared: true},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_BooleanExpressionAliased() {
	ident := exp.NewIdentifierExpression("", "", "a")

	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: ident.Eq(1).As("b"), sql: `("a" = 1) AS "b"`},
		expressionTestCase{val: ident.Eq(1).As("b"), sql: `("a" = ?) AS "b"`,
			isPrepared: true, args: []interface{}{int64(1)}},
	)
}
func (esgs *expressionSQLGeneratorSuite) TestGenerate_BooleanExpression() {
	ae := newTestAppendableExpression(`SELECT "id" FROM "test2"`, emptyArgs, nil, nil)
	re := regexp.MustCompile("[ab]")
	ident := exp.NewIdentifierExpression("", "", "a")

	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: ident.Eq(1), sql: `("a" = 1)`},
		expressionTestCase{val: ident.Eq(1), sql: `("a" = ?)`, isPrepared: true, args: []interface{}{int64(1)}},

		expressionTestCase{val: ident.Eq(true), sql: `("a" IS TRUE)`},
		expressionTestCase{val: ident.Eq(true), sql: `("a" IS TRUE)`, isPrepared: true},

		expressionTestCase{val: ident.Eq(false), sql: `("a" IS FALSE)`},
		expressionTestCase{val: ident.Eq(false), sql: `("a" IS FALSE)`, isPrepared: true},

		expressionTestCase{val: ident.Eq(nil), sql: `("a" IS NULL)`},
		expressionTestCase{val: ident.Eq(nil), sql: `("a" IS NULL)`, isPrepared: true},

		expressionTestCase{val: ident.Eq([]int64{1, 2, 3}), sql: `("a" IN (1, 2, 3))`},
		expressionTestCase{val: ident.Eq([]int64{1, 2, 3}), sql: `("a" IN (?, ?, ?))`, isPrepared: true, args: []interface{}{
			int64(1), int64(2), int64(3),
		}},

		expressionTestCase{val: ident.Eq(ae), sql: `("a" IN (SELECT "id" FROM "test2"))`},
		expressionTestCase{val: ident.Eq(ae), sql: `("a" IN (SELECT "id" FROM "test2"))`, isPrepared: true},

		expressionTestCase{val: ident.Neq(1), sql: `("a" != 1)`},
		expressionTestCase{val: ident.Neq(1), sql: `("a" != ?)`, isPrepared: true, args: []interface{}{int64(1)}},

		expressionTestCase{val: ident.Neq(true), sql: `("a" IS NOT TRUE)`},
		expressionTestCase{val: ident.Neq(true), sql: `("a" IS NOT TRUE)`, isPrepared: true},

		expressionTestCase{val: ident.Neq(false), sql: `("a" IS NOT FALSE)`},
		expressionTestCase{val: ident.Neq(false), sql: `("a" IS NOT FALSE)`, isPrepared: true},

		expressionTestCase{val: ident.Neq(nil), sql: `("a" IS NOT NULL)`},
		expressionTestCase{val: ident.Neq(nil), sql: `("a" IS NOT NULL)`, isPrepared: true},

		expressionTestCase{val: ident.Neq([]int64{1, 2, 3}), sql: `("a" NOT IN (1, 2, 3))`},
		expressionTestCase{val: ident.Neq([]int64{1, 2, 3}), sql: `("a" NOT IN (?, ?, ?))`, isPrepared: true, args: []interface{}{
			int64(1), int64(2), int64(3),
		}},

		expressionTestCase{val: ident.Neq(ae), sql: `("a" NOT IN (SELECT "id" FROM "test2"))`},
		expressionTestCase{val: ident.Neq(ae), sql: `("a" NOT IN (SELECT "id" FROM "test2"))`, isPrepared: true},

		expressionTestCase{val: ident.Is(true), sql: `("a" IS TRUE)`},
		expressionTestCase{val: ident.Is(true), sql: `("a" IS TRUE)`, isPrepared: true},

		expressionTestCase{val: ident.Is(false), sql: `("a" IS FALSE)`},
		expressionTestCase{val: ident.Is(false), sql: `("a" IS FALSE)`, isPrepared: true},

		expressionTestCase{val: ident.Is(nil), sql: `("a" IS NULL)`},
		expressionTestCase{val: ident.Is(nil), sql: `("a" IS NULL)`, isPrepared: true},

		expressionTestCase{val: ident.IsNot(true), sql: `("a" IS NOT TRUE)`},
		expressionTestCase{val: ident.IsNot(true), sql: `("a" IS NOT TRUE)`, isPrepared: true},

		expressionTestCase{val: ident.IsNot(false), sql: `("a" IS NOT FALSE)`},
		expressionTestCase{val: ident.IsNot(false), sql: `("a" IS NOT FALSE)`, isPrepared: true},

		expressionTestCase{val: ident.IsNot(nil), sql: `("a" IS NOT NULL)`},
		expressionTestCase{val: ident.IsNot(nil), sql: `("a" IS NOT NULL)`, isPrepared: true},

		expressionTestCase{val: ident.Gt(1), sql: `("a" > 1)`},
		expressionTestCase{val: ident.Gt(1), sql: `("a" > ?)`, isPrepared: true, args: []interface{}{int64(1)}},

		expressionTestCase{val: ident.Gte(1), sql: `("a" >= 1)`},
		expressionTestCase{val: ident.Gte(1), sql: `("a" >= ?)`, isPrepared: true, args: []interface{}{int64(1)}},

		expressionTestCase{val: ident.Lt(1), sql: `("a" < 1)`},
		expressionTestCase{val: ident.Lt(1), sql: `("a" < ?)`, isPrepared: true, args: []interface{}{int64(1)}},

		expressionTestCase{val: ident.Lte(1), sql: `("a" <= 1)`},
		expressionTestCase{val: ident.Lte(1), sql: `("a" <= ?)`, isPrepared: true, args: []interface{}{int64(1)}},

		expressionTestCase{val: ident.In([]int64{1, 2, 3}), sql: `("a" IN (1, 2, 3))`},
		expressionTestCase{val: ident.In([]int64{1, 2, 3}), sql: `("a" IN (?, ?, ?))`, isPrepared: true, args: []interface{}{
			int64(1), int64(2), int64(3),
		}},

		expressionTestCase{val: ident.In(ae), sql: `("a" IN (SELECT "id" FROM "test2"))`},
		expressionTestCase{val: ident.In(ae), sql: `("a" IN (SELECT "id" FROM "test2"))`, isPrepared: true},

		expressionTestCase{val: ident.NotIn([]int64{1, 2, 3}), sql: `("a" NOT IN (1, 2, 3))`},
		expressionTestCase{val: ident.NotIn([]int64{1, 2, 3}), sql: `("a" NOT IN (?, ?, ?))`, isPrepared: true, args: []interface{}{
			int64(1), int64(2), int64(3),
		}},

		expressionTestCase{val: ident.NotIn(ae), sql: `("a" NOT IN (SELECT "id" FROM "test2"))`},
		expressionTestCase{val: ident.NotIn(ae), sql: `("a" NOT IN (SELECT "id" FROM "test2"))`, isPrepared: true},

		expressionTestCase{val: ident.Like("a%"), sql: `("a" LIKE 'a%')`},
		expressionTestCase{val: ident.Like("a%"), sql: `("a" LIKE ?)`, isPrepared: true, args: []interface{}{"a%"}},

		expressionTestCase{val: ident.Like(re), sql: `("a" ~ '[ab]')`},
		expressionTestCase{val: ident.Like(re), sql: `("a" ~ ?)`, isPrepared: true, args: []interface{}{"[ab]"}},

		expressionTestCase{val: ident.ILike("a%"), sql: `("a" ILIKE 'a%')`},
		expressionTestCase{val: ident.ILike("a%"), sql: `("a" ILIKE ?)`, isPrepared: true, args: []interface{}{"a%"}},

		expressionTestCase{val: ident.ILike(re), sql: `("a" ~* '[ab]')`},
		expressionTestCase{val: ident.ILike(re), sql: `("a" ~* ?)`, isPrepared: true, args: []interface{}{"[ab]"}},

		expressionTestCase{val: ident.NotLike("a%"), sql: `("a" NOT LIKE 'a%')`},
		expressionTestCase{val: ident.NotLike("a%"), sql: `("a" NOT LIKE ?)`, isPrepared: true, args: []interface{}{"a%"}},

		expressionTestCase{val: ident.NotLike(re), sql: `("a" !~ '[ab]')`},
		expressionTestCase{val: ident.NotLike(re), sql: `("a" !~ ?)`, isPrepared: true, args: []interface{}{"[ab]"}},

		expressionTestCase{val: ident.NotILike("a%"), sql: `("a" NOT ILIKE 'a%')`},
		expressionTestCase{val: ident.NotILike("a%"), sql: `("a" NOT ILIKE ?)`, isPrepared: true, args: []interface{}{"a%"}},

		expressionTestCase{val: ident.NotILike(re), sql: `("a" !~* '[ab]')`},
		expressionTestCase{val: ident.NotILike(re), sql: `("a" !~* ?)`, isPrepared: true, args: []interface{}{"[ab]"}},
	)

	opts := sqlgen.DefaultDialectOptions()
	opts.BooleanOperatorLookup = map[exp.BooleanOperation][]byte{}
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", opts),
		expressionTestCase{val: ident.Eq(1), err: "goqu: dialect does not support boolean operator \"eq\" [dialect=test]"},
		expressionTestCase{val: ident.Neq(1), err: "goqu: dialect does not support boolean operator \"neq\" [dialect=test]"},
		expressionTestCase{val: ident.Is(true), err: "goqu: dialect does not support boolean operator \"is\" [dialect=test]"},
		expressionTestCase{val: ident.IsNot(true), err: "goqu: dialect does not support boolean operator \"isnot\" [dialect=test]"},
		expressionTestCase{val: ident.Gt(1), err: "goqu: dialect does not support boolean operator \"gt\" [dialect=test]"},
		expressionTestCase{val: ident.Gte(1), err: "goqu: dialect does not support boolean operator \"gte\" [dialect=test]"},
		expressionTestCase{val: ident.Lt(1), err: "goqu: dialect does not support boolean operator \"lt\" [dialect=test]"},
		expressionTestCase{val: ident.Lte(1), err: "goqu: dialect does not support boolean operator \"lte\" [dialect=test]"},
		expressionTestCase{val: ident.In([]int64{1, 2, 3}), err: "goqu: dialect does not support boolean operator \"in\" [dialect=test]"},
		expressionTestCase{val: ident.NotIn([]int64{1, 2, 3}), err: "goqu: dialect does not support boolean operator \"notin\" [dialect=test]"},
		expressionTestCase{val: ident.Like("a%"), err: "goqu: dialect does not support boolean operator \"like\" [dialect=test]"},
		expressionTestCase{val: ident.Like(re), err: "goqu: dialect does not support boolean operator \"regexplike\" [dialect=test]"},
		expressionTestCase{val: ident.ILike("a%"), err: "goqu: dialect does not support boolean operator \"ilike\" [dialect=test]"},
		expressionTestCase{val: ident.ILike(re), err: "goqu: dialect does not support boolean operator \"regexpilike\" [dialect=test]"},
		expressionTestCase{val: ident.NotLike("a%"), err: "goqu: dialect does not support boolean operator \"notlike\" [dialect=test]"},
		expressionTestCase{val: ident.NotLike(re), err: "goqu: dialect does not support boolean operator \"regexpnotlike\" [dialect=test]"},
		expressionTestCase{val: ident.NotILike("a%"), err: "goqu: dialect does not support boolean operator \"notilike\" [dialect=test]"},
		expressionTestCase{val: ident.NotILike(re), err: "goqu: dialect does not support boolean operator \"regexpnotilike\" [dialect=test]"},
	)
}

type arrayLiteralValuer []int

func (v arrayLiteralValuer) Value() (driver.Value, error) {
	parts := make([]string, len(v))
	for i, n := range v {
		parts[i] = strconv.Itoa(n)
	}
	return "{" + strings.Join(parts, ",") + "}", nil
}

type uint8Codes []uint8

func (esgs *expressionSQLGeneratorSuite) TestGenerate_BooleanExpressionEmptyList() {
	ident := exp.NewIdentifierExpression("", "", "a")
	var nilInts []int
	var nilIntsPtr *[]int
	emptyStrings := []string{}

	for _, c := range []struct {
		val          exp.Expression
		sql          string
		preparedSQL  string
		preparedArgs []interface{}
	}{
		{val: ident.In([]int{}), sql: `(1=0)`},
		{val: ident.In(nilInts), sql: `(1=0)`},
		{val: ident.In(nilIntsPtr), sql: `(1=0)`},
		{val: ident.In(nil), sql: `(1=0)`},
		{val: ident.In(), sql: `(1=0)`},
		{val: ident.In([]interface{}{}), sql: `(1=0)`},
		{val: ident.In(&emptyStrings), sql: `(1=0)`},
		{val: ident.In(uint8Codes{}), sql: `(1=0)`},
		{val: ident.In([0]int{}), sql: `(1=0)`},
		{val: ident.In((*[2]int)(nil)), sql: `(1=0)`},
		{val: exp.Ex{"a": [0]string{}}, sql: `(1=0)`},
		{val: ident.Eq([]int{}), sql: `(1=0)`},
		{val: exp.Ex{"a": []string{}}, sql: `(1=0)`},
		{val: exp.Ex{"a": nilInts}, sql: `(1=0)`},
		{val: exp.Ex{"a": exp.Op{"in": []string{}}}, sql: `(1=0)`},
		{val: exp.Ex{"a": exp.Op{"in": nil}}, sql: `(1=0)`},
		{val: exp.Ex{"a": exp.Op{"eq": []int{}}}, sql: `(1=0)`},
		{val: ident.Is([]int{}), sql: `(1=0)`},
		{val: exp.Ex{"a": exp.Op{"is": []int{}}}, sql: `(1=0)`},
		{val: exp.NewLiteralExpression("? + 1", 5).In([]int{}), sql: `(1=0)`},
		{val: ident.NotIn([]int{}), sql: `(1=1)`},
		{val: ident.NotIn([0]int{}), sql: `(1=1)`},
		{val: ident.NotIn(nil), sql: `(1=1)`},
		{val: ident.NotIn(nilIntsPtr), sql: `(1=1)`},
		{val: ident.NotIn(), sql: `(1=1)`},
		{val: ident.Neq(nilInts), sql: `(1=1)`},
		{val: exp.Ex{"a": exp.Op{"notIn": []string{}}}, sql: `(1=1)`},
		{val: exp.Ex{"a": exp.Op{"notIn": nil}}, sql: `(1=1)`},
		{val: exp.Ex{"a": exp.Op{"neq": []int{}}}, sql: `(1=1)`},
		{val: ident.IsNot([]int{}), sql: `(1=1)`},
		{val: exp.Ex{"a": exp.Op{"isNot": []int{}}}, sql: `(1=1)`},
		{val: exp.ExOr{"a": []int{}, "b": exp.Op{"notIn": []int{}}}, sql: `((1=0) OR (1=1))`},
		{
			val:          exp.NewExpressionList(exp.AndType, ident.In([]int{}), ident.Eq(1)),
			sql:          `((1=0) AND ("a" = 1))`,
			preparedSQL:  `((1=0) AND ("a" = ?))`,
			preparedArgs: []interface{}{int64(1)},
		},
	} {
		if c.preparedSQL == "" {
			c.preparedSQL = c.sql
		}
		esgs.assertCases(
			sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
			expressionTestCase{val: c.val, sql: c.sql},
			expressionTestCase{val: c.val, sql: c.preparedSQL, isPrepared: true, args: c.preparedArgs},
		)
	}
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ArrayList() {
	ident := exp.NewIdentifierExpression("", "", "a")
	pair := [2]int{1, 2}
	inPair := ident.In(&pair)
	pair[0] = 9
	bytes16 := [16]byte{1}
	for _, c := range []struct {
		val          interface{}
		sql          string
		preparedSQL  string
		preparedArgs []interface{}
	}{
		{
			val: ident.In([2]int{1, 2}), sql: `("a" IN (1, 2))`,
			preparedSQL: `("a" IN (?, ?))`, preparedArgs: []interface{}{int64(1), int64(2)},
		},
		{
			val: ident.NotIn([2]int{1, 2}), sql: `("a" NOT IN (1, 2))`,
			preparedSQL: `("a" NOT IN (?, ?))`, preparedArgs: []interface{}{int64(1), int64(2)},
		},
		{
			val: exp.Ex{"a": [2]string{"x", "y"}}, sql: `("a" IN ('x', 'y'))`,
			preparedSQL: `("a" IN (?, ?))`, preparedArgs: []interface{}{"x", "y"},
		},
		{
			val: exp.Ex{"a": exp.Op{"neq": [2]int{1, 2}}}, sql: `("a" NOT IN (1, 2))`,
			preparedSQL: `("a" NOT IN (?, ?))`, preparedArgs: []interface{}{int64(1), int64(2)},
		},
		{val: inPair, sql: `("a" IN (1, 2))`, preparedSQL: `("a" IN (?, ?))`, preparedArgs: []interface{}{int64(1), int64(2)}},
		{
			val: exp.NewLiteralExpression("?", [2]int{1, 2}), sql: `(1, 2)`,
			preparedSQL: `(?, ?)`, preparedArgs: []interface{}{int64(1), int64(2)},
		},
	} {
		esgs.assertCases(
			sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
			expressionTestCase{val: c.val, sql: c.sql},
			expressionTestCase{val: c.val, sql: c.preparedSQL, isPrepared: true, args: c.preparedArgs},
		)
	}
	const unencodable = "goqu: unable to encode value [1 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0]"
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: ident.In(bytes16), sql: `("a" IN (?))`, isPrepared: true, args: []interface{}{bytes16}},
		expressionTestCase{val: ident.NotIn(bytes16), sql: `("a" NOT IN (?))`, isPrepared: true, args: []interface{}{bytes16}},
		expressionTestCase{val: ident.Eq(bytes16), sql: `("a" = ?)`, isPrepared: true, args: []interface{}{bytes16}},
		expressionTestCase{val: exp.Ex{"a": bytes16}, sql: `("a" = ?)`, isPrepared: true, args: []interface{}{bytes16}},
		expressionTestCase{val: ident.In(bytes16), err: unencodable},
		expressionTestCase{val: ident.NotIn(bytes16), err: unencodable},
		expressionTestCase{val: ident.Eq(bytes16), err: unencodable},
		expressionTestCase{val: exp.Ex{"a": bytes16}, err: unencodable},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_BooleanExpressionEmptyListLHSFollowsPreparedMode() {
	lhs := exp.NewLiteralExpression("?", [2]byte{1, 2})
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: lhs.In([]int{}), err: "goqu: unable to encode value [1 2]"},
		expressionTestCase{val: lhs.In([]int{}), sql: `(1=0)`, isPrepared: true},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_BooleanExpressionEmptyListUsesDialectFragments() {
	ident := exp.NewIdentifierExpression("", "", "a")
	opts := sqlgen.DefaultDialectOptions()
	opts.EmptyInFragment = []byte("FALSE")
	opts.EmptyNotInFragment = []byte("TRUE")
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", opts),
		expressionTestCase{val: ident.In([]int{}), sql: `FALSE`},
		expressionTestCase{val: ident.NotIn([]int{}), sql: `TRUE`},
		expressionTestCase{val: ident.NotIn([]int{}), sql: `TRUE`, isPrepared: true},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_BooleanExpressionRejectsListsComparedToAColumn() {
	ident := exp.NewIdentifierExpression("", "", "a")
	const (
		gt   = `goqu: operator "gt" compares a column with a single value, got a list ([]int)`
		like = `goqu: operator "like" compares a column with a single value, got a list ([]string)`
	)
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: ident.Gt([]int{}), err: gt},
		expressionTestCase{val: ident.Gt([]int{}), err: gt, isPrepared: true},
		expressionTestCase{val: ident.Gt([]int{1, 2}), err: gt},
		expressionTestCase{val: ident.Gt([]int{1, 2}), err: gt, isPrepared: true},
		expressionTestCase{val: exp.Ex{"a": exp.Op{"like": []string{"x", "y"}}}, err: like},
		expressionTestCase{
			val: exp.NewBooleanExpression(exp.EqOp, ident, []int{1, 2}),
			err: `goqu: operator "eq" compares a column with a single value, got a list ([]int)`,
		},
		expressionTestCase{val: ident.Gt([]int{5}), sql: `("a" > (5))`},
		expressionTestCase{val: ident.Gt([]int{5}), sql: `("a" > (?))`, isPrepared: true, args: []interface{}{int64(5)}},
		expressionTestCase{val: exp.Ex{"a": exp.Op{"like": []string{"x%"}}}, sql: `("a" LIKE ('x%'))`},
		expressionTestCase{val: ident.Gt([]byte("x")), sql: `("a" > 'x')`},
		expressionTestCase{val: ident.Gt(arrayLiteralValuer{1, 2}), sql: `("a" > '{1,2}')`},
		expressionTestCase{val: exp.NewLiteralExpression("(a, b)").Gt([]int{1, 2}), sql: `((a, b) > (1, 2))`},
		expressionTestCase{
			val:        exp.NewLiteralExpression("(a, b)").Gt([]int{1, 2}),
			sql:        `((a, b) > (?, ?))`,
			isPrepared: true,
			args:       []interface{}{int64(1), int64(2)},
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_BooleanExpressionInWithPointerToSliceIsAList() {
	ident := exp.NewIdentifierExpression("", "", "a")
	ints := []int{1, 2}
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: ident.In(&ints), sql: `("a" IN (1, 2))`},
		expressionTestCase{val: ident.In(&ints), sql: `("a" IN (?, ?))`, isPrepared: true, args: []interface{}{int64(1), int64(2)}},
		expressionTestCase{val: ident.NotIn(&ints), sql: `("a" NOT IN (1, 2))`},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_BooleanExpressionNilPointerToSliceInEqIsNull() {
	var nilIntsPtr *[]int
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: exp.Ex{"a": nilIntsPtr}, sql: `("a" IS NULL)`},
		expressionTestCase{val: exp.Ex{"a": exp.Op{"neq": nilIntsPtr}}, sql: `("a" IS NOT NULL)`},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_BooleanExpressionEmptyListLeavesSingleValuesAlone() {
	ident := exp.NewIdentifierExpression("", "", "a")
	var nilInt *int
	var nilBytesPtr *[]byte
	var nilJSONPtr *json.RawMessage
	var nilValuerPtr *arrayLiteralValuer
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: ident.In(nilInt), sql: `("a" IN (NULL))`},
		expressionTestCase{val: ident.In(nilInt), sql: `("a" IN (?))`, isPrepared: true, args: []interface{}{nil}},
		expressionTestCase{val: ident.NotIn(nilBytesPtr), sql: `("a" NOT IN (NULL))`},
		expressionTestCase{val: ident.NotIn(nilJSONPtr), sql: `("a" NOT IN (NULL))`},
		expressionTestCase{val: ident.NotIn(nilValuerPtr), sql: `("a" NOT IN (NULL))`},
		expressionTestCase{val: exp.NewBooleanExpression(exp.InOp, ident, []byte{}), sql: `("a" IN '')`},
		expressionTestCase{
			val: exp.NewBooleanExpression(exp.InOp, ident, []byte{}), sql: `("a" IN ?)`, isPrepared: true, args: []interface{}{[]byte{}},
		},
		expressionTestCase{val: exp.NewBooleanExpression(exp.InOp, ident, json.RawMessage{}), sql: `("a" IN '')`},
		expressionTestCase{
			val:        exp.NewBooleanExpression(exp.InOp, ident, json.RawMessage{}),
			sql:        `("a" IN ?)`,
			isPrepared: true,
			args:       []interface{}{[]byte{}},
		},
		expressionTestCase{val: exp.NewBooleanExpression(exp.InOp, ident, arrayLiteralValuer{}), sql: `("a" IN '{}')`},
		expressionTestCase{
			val:        exp.NewBooleanExpression(exp.InOp, ident, arrayLiteralValuer{}),
			sql:        `("a" IN ?)`,
			isPrepared: true,
			args:       []interface{}{"{}"},
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_EmptyListOutsideInFails() {
	ident := exp.NewIdentifierExpression("", "", "a")
	const empty = "goqu: an empty list ([]int) is only accepted as the whole right side of In or NotIn"
	for _, prepared := range []bool{false, true} {
		esgs.assertCases(
			sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
			expressionTestCase{val: ident.In([]interface{}{1, []int{}}), err: empty, isPrepared: prepared},
			expressionTestCase{val: exp.NewLiteralExpression("(a, b)").In([]interface{}{[]int{}}), err: empty, isPrepared: prepared},
			expressionTestCase{val: ident.Eq(exp.NewSQLFunctionExpression("ANY ", []int{})), err: empty, isPrepared: prepared},
			expressionTestCase{val: ident.Neq(exp.NewSQLFunctionExpression("ALL ", []int{})), err: empty, isPrepared: prepared},
			expressionTestCase{val: exp.NewLiteralExpression("a IN ?", []int{}), err: empty, isPrepared: prepared},
			expressionTestCase{val: exp.NewLiteralExpression("a IN ?", []int(nil)), err: empty, isPrepared: prepared},
		)
	}
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: ident.In([]interface{}{1, []int{2, 3}}), sql: `("a" IN (1, (2, 3)))`},
		expressionTestCase{val: exp.NewLiteralExpression("a IN ?", []int{1}), sql: `a IN (1)`},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_BooleanExpressionSliceValuerIsOneValue() {
	ident := exp.NewIdentifierExpression("", "", "a")
	valuer := arrayLiteralValuer{1, 2}
	args := []interface{}{"{1,2}"}
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: ident.In(valuer), sql: `("a" IN ('{1,2}'))`},
		expressionTestCase{val: ident.In(valuer), sql: `("a" IN (?))`, isPrepared: true, args: args},
		expressionTestCase{val: ident.NotIn(valuer), sql: `("a" NOT IN ('{1,2}'))`},
		expressionTestCase{val: ident.NotIn(valuer), sql: `("a" NOT IN (?))`, isPrepared: true, args: args},
		expressionTestCase{val: ident.Eq(valuer), sql: `("a" = '{1,2}')`},
		expressionTestCase{val: ident.Eq(valuer), sql: `("a" = ?)`, isPrepared: true, args: args},
		expressionTestCase{val: ident.Neq(valuer), sql: `("a" != '{1,2}')`},
		expressionTestCase{val: ident.Neq(valuer), sql: `("a" != ?)`, isPrepared: true, args: args},
		expressionTestCase{val: exp.Ex{"a": valuer}, sql: `("a" = '{1,2}')`},
		expressionTestCase{val: exp.Ex{"a": valuer}, sql: `("a" = ?)`, isPrepared: true, args: args},
		expressionTestCase{val: exp.Ex{"a": arrayLiteralValuer{}}, sql: `("a" = '{}')`},
		expressionTestCase{val: ident.Eq(exp.NewSQLFunctionExpression("ANY ", valuer)), sql: `("a" = ANY ('{1,2}'))`},
		expressionTestCase{val: ident.Eq(exp.NewSQLFunctionExpression("ANY ", valuer)), sql: `("a" = ANY (?))`, isPrepared: true, args: args},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_BooleanExpressionEmptyListWithoutFragment() {
	ident := exp.NewIdentifierExpression("", "", "a")
	withoutIn := sqlgen.DefaultDialectOptions()
	withoutIn.EmptyInFragment = nil
	withoutNotIn := sqlgen.DefaultDialectOptions()
	withoutNotIn.EmptyNotInFragment = nil

	esgs.assertCases(sqlgen.NewExpressionSQLGenerator("test", withoutIn),
		expressionTestCase{val: ident.In([]int{}), err: "goqu: dialect has no EmptyInFragment for an empty IN list [dialect=test]"})
	esgs.assertCases(sqlgen.NewExpressionSQLGenerator("test", withoutNotIn),
		expressionTestCase{val: ident.NotIn([]int{}), err: "goqu: dialect has no EmptyNotInFragment for an empty NOT IN list [dialect=test]"})

	for opts, val := range map[*sqlgen.SQLDialectOptions]exp.Expression{withoutIn: ident.In([]int{}), withoutNotIn: ident.NotIn([]int{})} {
		b := sb.NewSQLBuilder(false)
		sqlgen.NewExpressionSQLGenerator("test", opts).Generate(b, val)
		_, _, err := b.ToSQL()
		esgs.True(goqu.IsUnsupported(err))
	}
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_BooleanExpressionEmptyListUnsupportedOperator() {
	ident := exp.NewIdentifierExpression("", "", "a")
	withoutIn := sqlgen.DefaultDialectOptions()
	delete(withoutIn.BooleanOperatorLookup, exp.InOp)
	withoutIn.EmptyInFragment = nil
	withoutNotIn := sqlgen.DefaultDialectOptions()
	delete(withoutNotIn.BooleanOperatorLookup, exp.NotInOp)
	withoutNotIn.EmptyNotInFragment = nil
	esgs.assertCases(sqlgen.NewExpressionSQLGenerator("test", withoutIn),
		expressionTestCase{val: ident.In([]int{}), err: "goqu: dialect does not support boolean operator \"in\" [dialect=test]"})
	esgs.assertCases(sqlgen.NewExpressionSQLGenerator("test", withoutNotIn),
		expressionTestCase{val: ident.NotIn([]int{}), err: "goqu: dialect does not support boolean operator \"notin\" [dialect=test]"})
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_BooleanExpressionEmptyListReportsLHSError() {
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: exp.NewExcludedExpression("a").In([]int{}), err: "goqu: Excluded is only valid in an OnConflict DoUpdate"},
		expressionTestCase{
			val: exp.NewExcludedExpression("a").In([]int{}), err: "goqu: Excluded is only valid in an OnConflict DoUpdate", isPrepared: true,
		},
	)

	withoutIn := sqlgen.DefaultDialectOptions()
	delete(withoutIn.BooleanOperatorLookup, exp.InOp)
	withoutIn.EmptyInFragment = nil
	esgs.assertCases(sqlgen.NewExpressionSQLGenerator("test", withoutIn),
		expressionTestCase{val: exp.NewExcludedExpression("a").In([]int{}), err: "goqu: Excluded is only valid in an OnConflict DoUpdate"})
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_BooleanExpressionLikeEscape() {
	ident := exp.NewIdentifierExpression("", "", "a")
	re := regexp.MustCompile("[ab]")
	opts := sqlgen.DefaultDialectOptions()
	opts.LikeEscapeKey = "ESCAPE"
	opts.LikeEscapeValue = "\\"
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", opts),
		expressionTestCase{val: ident.Like("a%"), sql: `("a" LIKE 'a%' ESCAPE '\')`},
		expressionTestCase{val: ident.NotLike("a%"), sql: `("a" NOT LIKE 'a%' ESCAPE '\')`},
		expressionTestCase{val: ident.ILike("a%"), sql: `("a" ILIKE 'a%' ESCAPE '\')`},
		expressionTestCase{val: ident.NotILike("a%"), sql: `("a" NOT ILIKE 'a%' ESCAPE '\')`},
		expressionTestCase{val: ident.ILike(re), sql: `("a" ~* '[ab]')`},
		expressionTestCase{val: ident.Eq("a%"), sql: `("a" = 'a%')`},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_BitwiseExpression() {
	ident := exp.NewIdentifierExpression("", "", "a")
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: ident.BitwiseInversion(), sql: `(~ "a")`},
		expressionTestCase{val: ident.BitwiseInversion(), sql: `(~ "a")`, isPrepared: true},

		expressionTestCase{val: ident.BitwiseAnd(1), sql: `("a" & 1)`},
		expressionTestCase{val: ident.BitwiseAnd(1), sql: `("a" & ?)`, isPrepared: true, args: []interface{}{int64(1)}},

		expressionTestCase{val: ident.BitwiseOr(1), sql: `("a" | 1)`},
		expressionTestCase{val: ident.BitwiseOr(1), sql: `("a" | ?)`, isPrepared: true, args: []interface{}{int64(1)}},

		expressionTestCase{val: ident.BitwiseXor(1), sql: `("a" # 1)`},
		expressionTestCase{val: ident.BitwiseXor(1), sql: `("a" # ?)`, isPrepared: true, args: []interface{}{int64(1)}},

		expressionTestCase{val: ident.BitwiseLeftShift(1), sql: `("a" << 1)`},
		expressionTestCase{val: ident.BitwiseLeftShift(1), sql: `("a" << ?)`, isPrepared: true, args: []interface{}{int64(1)}},

		expressionTestCase{val: ident.BitwiseRightShift(1), sql: `("a" >> 1)`},
		expressionTestCase{val: ident.BitwiseRightShift(1), sql: `("a" >> ?)`, isPrepared: true, args: []interface{}{int64(1)}},
	)

	opts := sqlgen.DefaultDialectOptions()
	opts.BitwiseOperatorLookup = map[exp.BitwiseOperation][]byte{}
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", opts),
		expressionTestCase{val: ident.BitwiseInversion(), err: "goqu: dialect does not support bitwise operator \"Inversion\" [dialect=test]"},
		expressionTestCase{val: ident.BitwiseAnd(1), err: "goqu: dialect does not support bitwise operator \"AND\" [dialect=test]"},
		expressionTestCase{val: ident.BitwiseOr(1), err: "goqu: dialect does not support bitwise operator \"OR\" [dialect=test]"},
		expressionTestCase{val: ident.BitwiseXor(1), err: "goqu: dialect does not support bitwise operator \"XOR\" [dialect=test]"},
		expressionTestCase{val: ident.BitwiseLeftShift(1), err: "goqu: dialect does not support bitwise operator \"Left Shift\" [dialect=test]"},
		expressionTestCase{
			val: ident.BitwiseRightShift(1),
			err: "goqu: dialect does not support bitwise operator \"Right Shift\" [dialect=test]",
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ArithmeticExpression() {
	ident := exp.NewIdentifierExpression("", "", "a")
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: ident.Add(1), sql: `("a" + 1)`},
		expressionTestCase{val: ident.Add(1), sql: `("a" + ?)`, isPrepared: true, args: []interface{}{int64(1)}},

		expressionTestCase{val: ident.Sub(1), sql: `("a" - 1)`},
		expressionTestCase{val: ident.Sub(1), sql: `("a" - ?)`, isPrepared: true, args: []interface{}{int64(1)}},

		expressionTestCase{val: ident.Mul(1), sql: `("a" * 1)`},
		expressionTestCase{val: ident.Mul(1), sql: `("a" * ?)`, isPrepared: true, args: []interface{}{int64(1)}},

		expressionTestCase{val: ident.Div(1), sql: `("a" / 1)`},
		expressionTestCase{val: ident.Div(1), sql: `("a" / ?)`, isPrepared: true, args: []interface{}{int64(1)}},
	)
	opts := sqlgen.DefaultDialectOptions()
	opts.ArithmeticOperatorLookup = map[exp.ArithmeticOperation][]byte{}
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", opts),
		expressionTestCase{val: ident.Add(1), err: "goqu: dialect does not support arithmetic operator \"Addition\" [dialect=test]"},
		expressionTestCase{val: ident.Sub(1), err: "goqu: dialect does not support arithmetic operator \"Subtraction\" [dialect=test]"},
		expressionTestCase{val: ident.Mul(1), err: "goqu: dialect does not support arithmetic operator \"Multiplication\" [dialect=test]"},
		expressionTestCase{val: ident.Div(1), err: "goqu: dialect does not support arithmetic operator \"Division\" [dialect=test]"},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_RangeExpression() {
	betweenNum := exp.NewIdentifierExpression("", "", "a").
		Between(exp.NewRangeVal(1, 2))
	notBetweenNum := exp.NewIdentifierExpression("", "", "a").
		NotBetween(exp.NewRangeVal(1, 2))

	betweenStr := exp.NewIdentifierExpression("", "", "a").
		Between(exp.NewRangeVal("aaa", "zzz"))
	notBetweenStr := exp.NewIdentifierExpression("", "", "a").
		NotBetween(exp.NewRangeVal("aaa", "zzz"))
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: betweenNum, sql: `("a" BETWEEN 1 AND 2)`},
		expressionTestCase{val: betweenNum, sql: `("a" BETWEEN ? AND ?)`, isPrepared: true, args: []interface{}{
			int64(1),
			int64(2),
		}},

		expressionTestCase{val: notBetweenNum, sql: `("a" NOT BETWEEN 1 AND 2)`},
		expressionTestCase{val: notBetweenNum, sql: `("a" NOT BETWEEN ? AND ?)`, isPrepared: true, args: []interface{}{
			int64(1),
			int64(2),
		}},

		expressionTestCase{val: betweenStr, sql: `("a" BETWEEN 'aaa' AND 'zzz')`},
		expressionTestCase{val: betweenStr, sql: `("a" BETWEEN ? AND ?)`, isPrepared: true, args: []interface{}{
			"aaa",
			"zzz",
		}},

		expressionTestCase{val: notBetweenStr, sql: `("a" NOT BETWEEN 'aaa' AND 'zzz')`},
		expressionTestCase{val: notBetweenStr, sql: `("a" NOT BETWEEN ? AND ?)`, isPrepared: true, args: []interface{}{
			"aaa",
			"zzz",
		}},
	)

	opts := sqlgen.DefaultDialectOptions()
	opts.RangeOperatorLookup = map[exp.RangeOperation][]byte{}
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", opts),
		expressionTestCase{val: betweenNum, err: "goqu: dialect does not support range operator \"between\" [dialect=test]"},
		expressionTestCase{val: betweenNum, err: "goqu: dialect does not support range operator \"between\" [dialect=test]", isPrepared: true},

		expressionTestCase{val: notBetweenNum, err: "goqu: dialect does not support range operator \"not between\" [dialect=test]"},
		expressionTestCase{
			val: notBetweenNum,
			err: "goqu: dialect does not support range operator \"not between\" [dialect=test]", isPrepared: true,
		},

		expressionTestCase{val: betweenStr, err: "goqu: dialect does not support range operator \"between\" [dialect=test]"},
		expressionTestCase{val: betweenStr, err: "goqu: dialect does not support range operator \"between\" [dialect=test]", isPrepared: true},

		expressionTestCase{val: notBetweenStr, err: "goqu: dialect does not support range operator \"not between\" [dialect=test]"},
		expressionTestCase{
			val: notBetweenStr,
			err: "goqu: dialect does not support range operator \"not between\" [dialect=test]", isPrepared: true,
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_OrderedExpression() {
	asc := exp.NewIdentifierExpression("", "", "a").Asc()
	ascNf := exp.NewIdentifierExpression("", "", "a").Asc().NullsFirst()
	ascNl := exp.NewIdentifierExpression("", "", "a").Asc().NullsLast()

	desc := exp.NewIdentifierExpression("", "", "a").Desc()
	descNf := exp.NewIdentifierExpression("", "", "a").Desc().NullsFirst()
	descNl := exp.NewIdentifierExpression("", "", "a").Desc().NullsLast()

	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: asc, sql: `"a" ASC`},
		expressionTestCase{val: asc, sql: `"a" ASC`, isPrepared: true},

		expressionTestCase{val: ascNf, sql: `"a" ASC NULLS FIRST`},
		expressionTestCase{val: ascNf, sql: `"a" ASC NULLS FIRST`, isPrepared: true},

		expressionTestCase{val: ascNl, sql: `"a" ASC NULLS LAST`},
		expressionTestCase{val: ascNl, sql: `"a" ASC NULLS LAST`, isPrepared: true},

		expressionTestCase{val: desc, sql: `"a" DESC`},
		expressionTestCase{val: desc, sql: `"a" DESC`, isPrepared: true},

		expressionTestCase{val: descNf, sql: `"a" DESC NULLS FIRST`},
		expressionTestCase{val: descNf, sql: `"a" DESC NULLS FIRST`, isPrepared: true},

		expressionTestCase{val: descNl, sql: `"a" DESC NULLS LAST`},
		expressionTestCase{val: descNl, sql: `"a" DESC NULLS LAST`, isPrepared: true},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_UpdateExpression() {
	ue := exp.NewIdentifierExpression("", "", "a").Set(1)
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: ue, sql: `"a"=1`},
		expressionTestCase{val: ue, sql: `"a"=?`, isPrepared: true, args: []interface{}{int64(1)}},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_SQLFunctionExpression() {
	minimum := exp.NewSQLFunctionExpression("MIN", exp.NewIdentifierExpression("", "", "a"))
	coalesce := exp.NewSQLFunctionExpression("COALESCE", exp.NewIdentifierExpression("", "", "a"), "a")
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: minimum, sql: `MIN("a")`},
		expressionTestCase{val: minimum, sql: `MIN("a")`, isPrepared: true},

		expressionTestCase{val: coalesce, sql: `COALESCE("a", 'a')`},
		expressionTestCase{val: coalesce, sql: `COALESCE("a", ?)`, isPrepared: true, args: []interface{}{"a"}},

		expressionTestCase{val: exp.NewSQLFunctionExpression("NOW"), sql: `NOW()`},
		expressionTestCase{val: exp.NewSQLFunctionExpression("NOW"), sql: `NOW()`, isPrepared: true},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_SQLWindowFunctionExpression() {
	sqlWinFunc := exp.NewSQLWindowFunctionExpression(
		exp.NewSQLFunctionExpression("some_func"),
		nil,
		exp.NewWindowExpression(
			"db",
			nil,
			exp.NewIdentifierExpression("", "", "win"),
			nil,
			nil,
		),
	)
	sqlWinFuncFromWindow := exp.NewSQLWindowFunctionExpression(
		exp.NewSQLFunctionExpression("some_func"),
		exp.NewIdentifierExpression("", "", "win"),
		nil,
	)

	emptyWinFunc := exp.NewSQLWindowFunctionExpression(
		exp.NewSQLFunctionExpression("some_func"),
		nil,
		nil,
	)
	badNamedSQLWinFuncInherit := exp.NewSQLWindowFunctionExpression(
		exp.NewSQLFunctionExpression("some_func"),
		nil,
		exp.NewWindowExpression(
			"db",
			exp.NewIdentifierExpression("", "", "w"),
			nil,
			nil,
			nil,
		),
	)
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: sqlWinFunc, sql: `some_func() OVER ("win")`},
		expressionTestCase{val: sqlWinFunc, sql: `some_func() OVER ("win")`, isPrepared: true},

		expressionTestCase{val: sqlWinFuncFromWindow, sql: `some_func() OVER "win"`},
		expressionTestCase{val: sqlWinFuncFromWindow, sql: `some_func() OVER "win"`, isPrepared: true},

		expressionTestCase{val: emptyWinFunc, sql: `some_func() OVER ()`},
		expressionTestCase{val: emptyWinFunc, sql: `some_func() OVER ()`, isPrepared: true},

		expressionTestCase{val: badNamedSQLWinFuncInherit, err: "goqu: unexpected named window function"},
		expressionTestCase{val: badNamedSQLWinFuncInherit, err: "goqu: unexpected named window function", isPrepared: true},
	)
	opts := sqlgen.DefaultDialectOptions()
	opts.SupportsWindowFunction = false
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", opts),
		expressionTestCase{val: sqlWinFunc, err: "goqu: dialect does not support WINDOW clause [dialect=test]"},
		expressionTestCase{val: sqlWinFunc, err: "goqu: dialect does not support WINDOW clause [dialect=test]", isPrepared: true},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_WindowExpression() {
	opts := sqlgen.DefaultDialectOptions()
	opts.WindowPartitionByFragment = []byte("partition by ")
	opts.WindowOrderByFragment = []byte("order by ")

	emptySQLWinFunc := exp.NewWindowExpression("db", nil, nil, nil, nil)
	namedSQLWinFunc := exp.NewWindowExpression(
		"db", exp.NewIdentifierExpression("", "", "w"), nil, nil, nil,
	)
	inheritSQLWinFunc := exp.NewWindowExpression(
		"db", nil, exp.NewIdentifierExpression("", "", "w"), nil, nil,
	)
	partitionBySQLWinFunc := exp.NewWindowExpression(
		"db", nil, nil, exp.NewColumnListExpression(nil, "db", "a", "b"), nil,
	)
	orderBySQLWinFunc := exp.NewWindowExpression(
		"db", nil, nil, nil, exp.NewOrderedColumnList(
			exp.NewIdentifierExpression("", "", "a").Asc(),
			exp.NewIdentifierExpression("", "", "b").Desc(),
		),
	)

	namedInheritPartitionOrderSQLWinFunc := exp.NewWindowExpression(
		"db",
		exp.NewIdentifierExpression("", "", "w1"),
		exp.NewIdentifierExpression("", "", "w2"),
		exp.NewColumnListExpression(nil, "db", "a", "b"),
		exp.NewOrderedColumnList(
			exp.NewIdentifierExpression("", "", "a").Asc(),
			exp.NewIdentifierExpression("", "", "b").Desc(),
		),
	)

	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", opts),
		expressionTestCase{val: emptySQLWinFunc, sql: `()`},
		expressionTestCase{val: emptySQLWinFunc, sql: `()`, isPrepared: true},

		expressionTestCase{val: namedSQLWinFunc, sql: `"w" AS ()`},
		expressionTestCase{val: namedSQLWinFunc, sql: `"w" AS ()`, isPrepared: true},

		expressionTestCase{val: inheritSQLWinFunc, sql: `("w")`},
		expressionTestCase{val: inheritSQLWinFunc, sql: `("w")`, isPrepared: true},

		expressionTestCase{val: partitionBySQLWinFunc, sql: `(partition by "a", "b")`},
		expressionTestCase{val: partitionBySQLWinFunc, sql: `(partition by "a", "b")`, isPrepared: true},

		expressionTestCase{val: orderBySQLWinFunc, sql: `(order by "a" ASC, "b" DESC)`},
		expressionTestCase{val: orderBySQLWinFunc, sql: `(order by "a" ASC, "b" DESC)`, isPrepared: true},

		expressionTestCase{
			val: namedInheritPartitionOrderSQLWinFunc,
			sql: `"w1" AS ("w2" partition by "a", "b" order by "a" ASC, "b" DESC)`,
		},
		expressionTestCase{
			val:        namedInheritPartitionOrderSQLWinFunc,
			sql:        `"w1" AS ("w2" partition by "a", "b" order by "a" ASC, "b" DESC)`,
			isPrepared: true,
		},
	)

	opts = sqlgen.DefaultDialectOptions()
	opts.SupportsWindowFunction = false
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", opts),
		expressionTestCase{val: emptySQLWinFunc, err: "goqu: dialect does not support WINDOW clause [dialect=test]"},
		expressionTestCase{val: emptySQLWinFunc, err: "goqu: dialect does not support WINDOW clause [dialect=test]", isPrepared: true},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_CastExpression() {
	cast := exp.NewIdentifierExpression("", "", "a").Cast("DATE")
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: cast, sql: `CAST("a" AS DATE)`},
		expressionTestCase{val: cast, sql: `CAST("a" AS DATE)`, isPrepared: true},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_CollateExpression() {
	ident := exp.NewIdentifierExpression("", "", "a")
	collate := ident.Collate("en_US")
	quoted := sqlgen.DefaultDialectOptions()
	quoted.QuoteCollation = true
	custom := sqlgen.DefaultDialectOptions()
	custom.CollateFragment = []byte(" collate ")

	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: collate, sql: `"a" COLLATE en_US`},
		expressionTestCase{val: collate, sql: `"a" COLLATE en_US`, isPrepared: true},
		expressionTestCase{val: ident.Collate("Latin1_General_CS_AS"), sql: `"a" COLLATE Latin1_General_CS_AS`},
		expressionTestCase{val: ident.Collate("pg_catalog.C1"), sql: `"a" COLLATE pg_catalog.C1`},
		expressionTestCase{val: ident.Collate("s.t.c"), sql: `"a" COLLATE s.t.c`},
		expressionTestCase{val: ident.Collate(exp.NewIdentifierExpression("", "", "en_US")), sql: `"a" COLLATE en_US`},
		expressionTestCase{
			val: ident.Collate(exp.ParseIdentifier("pg_catalog.default")), sql: `"a" COLLATE pg_catalog.default`,
		},
		expressionTestCase{val: collate.Asc(), sql: `"a" COLLATE en_US ASC`},
		expressionTestCase{val: collate.Desc(), sql: `"a" COLLATE en_US DESC`},
		expressionTestCase{val: collate.Eq("b"), sql: `("a" COLLATE en_US = 'b')`},
		expressionTestCase{
			val: collate.Eq("b"), sql: `("a" COLLATE en_US = ?)`, isPrepared: true, args: []any{"b"},
		},
		// a literal could be any expression, so it is wrapped to make sure the whole thing is collated
		expressionTestCase{val: exp.NewCollateExpression(exp.NewLiteralExpression("a || b"), "en_US"), sql: `(a || b) COLLATE en_US`},
	)
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", quoted),
		expressionTestCase{val: collate, sql: `"a" COLLATE "en_US"`},
		expressionTestCase{val: collate, sql: `"a" COLLATE "en_US"`, isPrepared: true},
		expressionTestCase{val: ident.Collate("en_US.utf8"), sql: `"a" COLLATE "en_US.utf8"`},
		expressionTestCase{val: ident.Collate(`we"ird`), sql: `"a" COLLATE "we""ird"`},
		expressionTestCase{val: ident.Collate("English_United States.1252"), sql: `"a" COLLATE "English_United States.1252"`},
		expressionTestCase{
			val: ident.Collate(exp.ParseIdentifier("pg_catalog.default")), sql: `"a" COLLATE "pg_catalog"."default"`,
		},
		expressionTestCase{
			val: ident.Collate(exp.NewIdentifierExpression("my_schema", "", "en_US")), sql: `"a" COLLATE "my_schema"."en_US"`,
		},
		expressionTestCase{
			val: exp.NewCollateExpression(exp.NewLiteralExpression("a || b"), "C"), sql: `(a || b) COLLATE "C"`,
		},
	)
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", custom),
		expressionTestCase{val: collate, sql: `"a" collate en_US`},
	)

	// names that are not simple identifiers are rejected when unquoted (they would end up verbatim in the SQL)
	for _, name := range []string{
		"", "en US", "en_US; DROP TABLE a", `en"US`, "1abc", "a-b", "a'b", "a.", ".a", "a..b", "a.b.c.d", "a$", "ü",
	} {
		esgs.assertCases(
			sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
			expressionTestCase{val: ident.Collate(name), err: fmt.Sprintf("goqu: invalid collation name %q", name)},
		)
	}
	// only strings and identifiers (with plain string parts) are valid collations
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", quoted),
		expressionTestCase{val: ident.Collate(1), err: `goqu: invalid collation name "1"`},
		expressionTestCase{val: ident.Collate(nil), err: `goqu: invalid collation name "<nil>"`},
		expressionTestCase{val: ident.Collate(exp.NewIdentifierExpression("", "", "")), err: `goqu: invalid collation name ""`},
		expressionTestCase{
			val: ident.Collate(exp.NewIdentifierExpression("", "", exp.NewLiteralExpression("x"))),
			err: `goqu: invalid collation name ""`,
		},
	)
	// unquoted dialects validate every part of an identifier
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{
			val: ident.Collate(exp.NewIdentifierExpression("my schema", "", "en_US")),
			err: `goqu: invalid collation name "my schema.en_US"`,
		},
	)
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", quoted),
		expressionTestCase{val: ident.Collate(""), err: `goqu: invalid collation name ""`},
		expressionTestCase{val: ident.Collate("a\x00b"), err: "goqu: invalid collation name \"a\\x00b\""},
		expressionTestCase{
			val: ident.Collate(exp.NewIdentifierExpression("", "", "a\x00b")), err: "goqu: invalid collation name \"a\\x00b\"",
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_TimeFunctionExpression() {
	now, now3, utc, utc0 := exp.NewNowExpression(), exp.NewNowExpression(3), exp.NewUTCTimestampExpression(), exp.NewUTCTimestampExpression(0)
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", timeStyleOptions(sqlgen.PostgresTimeStyle)),
		expressionTestCase{val: now, sql: "NOW()"},
		expressionTestCase{val: now3, sql: "CURRENT_TIMESTAMP(3)"},
		expressionTestCase{val: now3, sql: "CURRENT_TIMESTAMP(3)", isPrepared: true},
		expressionTestCase{val: utc, sql: "(NOW() AT TIME ZONE 'UTC')"},
		expressionTestCase{val: utc0, sql: "(CURRENT_TIMESTAMP(0) AT TIME ZONE 'UTC')"},
	)
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", timeStyleOptions(sqlgen.MySQLTimeStyle)),
		expressionTestCase{val: now, sql: "NOW()"},
		expressionTestCase{val: now3, sql: "NOW(3)"},
		expressionTestCase{val: utc, sql: "UTC_TIMESTAMP()"},
		expressionTestCase{val: utc0, sql: "UTC_TIMESTAMP(0)", isPrepared: true},
	)
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", timeStyleOptions(sqlgen.SQLiteTimeStyle)),
		expressionTestCase{val: now, sql: "datetime('now', 'localtime')"},
		expressionTestCase{val: now3, sql: "strftime('%Y-%m-%d %H:%M:%f', 'now', 'localtime')"},
		expressionTestCase{val: utc, sql: "datetime('now')"},
		expressionTestCase{val: utc0, sql: "datetime('now')"},
		expressionTestCase{
			val: exp.NewNowExpression(6), err: "goqu: dialect does not support fsp 6, want one of 0, 3 [dialect=test]",
		},
	)
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", timeStyleOptions(sqlgen.SQLServerTimeStyle)),
		expressionTestCase{val: now, sql: "SYSDATETIME()"},
		expressionTestCase{val: now3, sql: "CAST(SYSDATETIME() AS DATETIME2(3))"},
		expressionTestCase{val: utc, sql: "SYSUTCDATETIME()"},
		expressionTestCase{val: utc0, sql: "CAST(SYSUTCDATETIME() AS DATETIME2(0))"},
	)
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", timeStyleOptions(sqlgen.DuckDBTimeStyle)),
		expressionTestCase{val: now, sql: "CAST(now() AS TIMESTAMP)"},
		expressionTestCase{val: exp.NewNowExpression(6), sql: "CAST(now() AS TIMESTAMP)"},
		expressionTestCase{val: now3, sql: "CAST(CAST(now() AS TIMESTAMP) AS TIMESTAMP_MS)"},
		expressionTestCase{val: utc, sql: "make_timestamp(epoch_us(now()))"},
		expressionTestCase{val: utc0, sql: "CAST(make_timestamp(epoch_us(now())) AS TIMESTAMP_S)"},
		expressionTestCase{
			val: exp.NewNowExpression(2), err: "goqu: dialect does not support fsp 2, want one of 0, 3, 6 [dialect=test]",
		},
	)
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", timeStyleOptions(sqlgen.MySQLTimeStyle)),
		expressionTestCase{val: exp.NewNowExpression(7), err: "goqu: Now and UTCTimestamp fsp must be between 0 and 6, got 7"},
		expressionTestCase{val: exp.NewNowExpression(-1), err: "goqu: Now and UTCTimestamp fsp must be between 0 and 6, got -1"},
		expressionTestCase{val: exp.NewNowExpression(1, 2), err: "goqu: Now and UTCTimestamp take at most one fsp, got 2"},
		expressionTestCase{val: unknownTimeFunction{now}, err: "goqu: unknown TimeFunction 99"},
	)
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", timeStyleOptions(99)),
		expressionTestCase{val: now, err: "goqu: dialect has unknown TimeStyle 99 [dialect=test]"},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_IntervalExpression() {
	mysql := sqlgen.NewExpressionSQLGenerator("test", timeStyleOptions(sqlgen.MySQLTimeStyle))
	standard := sqlgen.NewExpressionSQLGenerator("test", timeStyleOptions(sqlgen.PostgresTimeStyle))
	units := []exp.IntervalUnit{
		exp.MicrosecondInterval, exp.SecondInterval, exp.MinuteInterval, exp.HourInterval,
		exp.DayInterval, exp.WeekInterval, exp.MonthInterval, exp.YearInterval,
	}
	for _, unit := range units {
		esgs.assertCases(mysql, expressionTestCase{val: exp.NewIntervalExpression(2, unit), sql: "INTERVAL 2 " + string(unit)})
		esgs.assertCases(standard, expressionTestCase{
			val: exp.NewIntervalExpression(2, unit), sql: "(2 * INTERVAL '1 " + string(unit) + "')",
		})
	}
	esgs.assertCases(
		mysql,
		expressionTestCase{val: exp.NewIntervalExpression(1, "quarter"), sql: "INTERVAL 1 QUARTER"},
		expressionTestCase{
			val: exp.NewIntervalExpression(1, exp.SecondInterval), sql: "INTERVAL ? SECOND", isPrepared: true, args: []any{int64(1)},
		},
		expressionTestCase{
			val: exp.NewIntervalExpression(exp.NewIdentifierExpression("", "", "n"), exp.DayInterval), sql: `INTERVAL "n" DAY`,
		},
		expressionTestCase{
			val: exp.NewIntervalExpression(1, "fortnight"),
			err: `goqu: unknown Interval unit "fortnight", want one of MICROSECOND, SECOND, MINUTE, HOUR, DAY, WEEK, MONTH, QUARTER, YEAR`,
		},
		expressionTestCase{
			val: exp.NewIntervalExpression(1, "ſecond"),
			err: `goqu: unknown Interval unit "ſecond", want one of MICROSECOND, SECOND, MINUTE, HOUR, DAY, WEEK, MONTH, QUARTER, YEAR`,
		},
		expressionTestCase{
			val: exp.NewIntervalExpression(1.5, exp.HourInterval),
			err: "goqu: Interval count must be an integer, an expression or, with SECOND, a finite number, got float64 1.5 for HOUR [dialect=test]",
		},
		expressionTestCase{val: exp.NewIntervalExpression(1.5, exp.SecondInterval), sql: "INTERVAL 1.5 SECOND"},
		expressionTestCase{
			val: exp.NewIntervalExpression(exp.NewLiteralExpression("? - 1", 5), exp.DayInterval),
			sql: "INTERVAL ? - 1 DAY", isPrepared: true, args: []any{int64(5)},
		},
	)
	two := 2
	esgs.assertCases(
		standard,
		expressionTestCase{val: exp.NewIntervalExpression(&two, exp.DayInterval), sql: "(2 * INTERVAL '1 DAY')"},
		expressionTestCase{val: exp.NewIntervalExpression(1, "Quarter"), sql: "(1 * INTERVAL '3 MONTH')"},
		expressionTestCase{
			val: exp.NewIntervalExpression(exp.NewLiteralExpression("n - 1"), exp.DayInterval), sql: "((n - 1) * INTERVAL '1 DAY')",
		},
		expressionTestCase{
			val: exp.NewIntervalExpression(exp.NewIdentifierExpression("", "", "n"), exp.DayInterval), sql: `("n" * INTERVAL '1 DAY')`,
		},
		expressionTestCase{
			val: exp.NewIntervalExpression(exp.NewLiteralExpression("? - 1", 5), exp.DayInterval),
			sql: "((? - 1) * INTERVAL '1 DAY')", isPrepared: true, args: []any{int64(5)},
		},
		expressionTestCase{
			val: exp.NewIntervalExpression(float32(2), exp.DayInterval),
			err: "goqu: Interval count must be an integer, an expression or, with SECOND, a finite number, got float32 2 for DAY [dialect=test]",
		},
		expressionTestCase{val: exp.NewIntervalExpression(1.5, exp.SecondInterval), sql: "(1.5 * INTERVAL '1 SECOND')"},
		expressionTestCase{val: exp.NewIntervalExpression(1.5, "second"), sql: "(1.5 * INTERVAL '1 SECOND')"},
		expressionTestCase{val: exp.NewIntervalExpression(uint8(2), exp.DayInterval), sql: "(2 * INTERVAL '1 DAY')"},
		expressionTestCase{val: exp.NewIntervalExpression(countValuer{2}, exp.DayInterval), sql: "(2 * INTERVAL '1 DAY')"},
		expressionTestCase{val: exp.NewIntervalExpression(nil, exp.DayInterval), sql: "(NULL * INTERVAL '1 DAY')"},
		expressionTestCase{val: exp.NewIntervalExpression(countValuer{nil}, exp.DayInterval), sql: "(NULL * INTERVAL '1 DAY')"},
		expressionTestCase{
			val: exp.NewIntervalExpression(1, exp.SecondInterval), sql: "(? * INTERVAL '1 SECOND')", isPrepared: true, args: []any{int64(1)},
		},
	)
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", timeStyleOptions(sqlgen.DuckDBTimeStyle)),
		expressionTestCase{val: exp.NewIntervalExpression(1, exp.SecondInterval), sql: "(1 * INTERVAL '1 SECOND')"},
	)
	for _, style := range []sqlgen.TimeStyle{sqlgen.SQLiteTimeStyle, sqlgen.SQLServerTimeStyle} {
		esgs.assertCases(
			sqlgen.NewExpressionSQLGenerator("test", timeStyleOptions(style)),
			expressionTestCase{val: exp.NewIntervalExpression(1, exp.SecondInterval), err: "goqu: dialect does not support Interval [dialect=test]"},
		)
	}
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", timeStyleOptions(99)),
		expressionTestCase{val: exp.NewIntervalExpression(1, exp.SecondInterval), err: "goqu: dialect has unknown TimeStyle 99 [dialect=test]"},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_TimeExpressionParenRunes() {
	do := timeStyleOptions(sqlgen.PostgresTimeStyle)
	do.LeftParenRune, do.RightParenRune = '{', '}'
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", do),
		expressionTestCase{val: exp.NewUTCTimestampExpression(), sql: "{NOW{} AT TIME ZONE 'UTC'}"},
		expressionTestCase{val: exp.NewIntervalExpression(1, exp.SecondInterval), sql: "{1 * INTERVAL '1 SECOND'}"},
		expressionTestCase{
			val: exp.NewIntervalExpression(exp.NewLiteralExpression("n - 1"), exp.DayInterval), sql: "{{n - 1} * INTERVAL '1 DAY'}",
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_TimeExpressionErrorTypes() {
	render := func(style sqlgen.TimeStyle, e exp.Expression) error {
		b := sb.NewSQLBuilder(false)
		sqlgen.NewExpressionSQLGenerator("test", timeStyleOptions(style)).Generate(b, e)
		_, _, err := b.ToSQL()
		return err
	}
	esgs.True(goqu.IsInvalidExpression(render(sqlgen.MySQLTimeStyle, exp.NewNowExpression(7))))
	esgs.True(goqu.IsInvalidExpression(render(sqlgen.MySQLTimeStyle, exp.NewNowExpression(1, 2))))
	esgs.True(goqu.IsInvalidExpression(render(sqlgen.MySQLTimeStyle, unknownTimeFunction{exp.NewNowExpression()})))
	esgs.True(goqu.IsInvalidExpression(render(sqlgen.MySQLTimeStyle, exp.NewIntervalExpression(1, "x"))))
	esgs.True(goqu.IsUnsupported(render(sqlgen.SQLiteTimeStyle, exp.NewNowExpression(6))))
	esgs.True(goqu.IsUnsupported(render(sqlgen.SQLServerTimeStyle, exp.NewIntervalExpression(1, exp.SecondInterval))))
	esgs.True(goqu.IsUnsupported(render(99, exp.NewNowExpression())))
	esgs.True(goqu.IsUnsupported(render(99, exp.NewNowExpression(7))))
	esgs.True(goqu.IsUnsupported(render(sqlgen.SQLiteTimeStyle, exp.NewIntervalExpression(1, "x"))))
	esgs.True(goqu.IsInvalidExpression(render(sqlgen.PostgresTimeStyle, exp.NewIntervalExpression(1.5, exp.DayInterval))))
}

type (
	intervalHours float64
	countValuer   struct{ value driver.Value }
)

func (c countValuer) Value() (driver.Value, error) { return c.value, nil }

type sequenceValuer struct{ values *[]driver.Value }

func (s sequenceValuer) Value() (driver.Value, error) {
	v := (*s.values)[0]
	*s.values = (*s.values)[1:]
	return v, nil
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_IntervalCountValuerResolvedOnce() {
	for style, sql := range map[sqlgen.TimeStyle]string{
		sqlgen.PostgresTimeStyle: "(2 * INTERVAL '1 DAY')",
		sqlgen.MySQLTimeStyle:    "INTERVAL 2 DAY",
	} {
		values := []driver.Value{int64(2), 1.5}
		esgs.assertCases(
			sqlgen.NewExpressionSQLGenerator("test", timeStyleOptions(style)),
			expressionTestCase{val: exp.NewIntervalExpression(sequenceValuer{&values}, exp.DayInterval), sql: sql},
		)
		esgs.Len(values, 1)
	}
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_IntervalCountErrors() {
	const anyNumber = "goqu: Interval count must be an integer, an expression or, with SECOND, a finite number, got "
	half := 1.5
	for _, tc := range []struct {
		style sqlgen.TimeStyle
		count any
		unit  exp.IntervalUnit
		err   string
	}{
		{sqlgen.PostgresTimeStyle, intervalHours(1.5), exp.HourInterval, anyNumber + "sqlgen_test.intervalHours 1.5 for HOUR [dialect=test]"},
		{sqlgen.PostgresTimeStyle, &half, exp.HourInterval, anyNumber + "*float64 1.5 for HOUR [dialect=test]"},
		{sqlgen.PostgresTimeStyle, countValuer{1.5}, exp.HourInterval, anyNumber + "sqlgen_test.countValuer 1.5 for HOUR [dialect=test]"},
		{sqlgen.PostgresTimeStyle, "1.5", exp.HourInterval, anyNumber + "string 1.5 for HOUR [dialect=test]"},
		{sqlgen.PostgresTimeStyle, time.Second, exp.HourInterval, anyNumber + "time.Duration 1s for HOUR [dialect=test]"},
		{sqlgen.PostgresTimeStyle, math.NaN(), exp.SecondInterval, anyNumber + "float64 NaN for SECOND [dialect=test]"},
		{sqlgen.MySQLTimeStyle, math.Inf(1), exp.SecondInterval, anyNumber + "float64 +Inf for SECOND [dialect=test]"},
		{sqlgen.PostgresTimeStyle, exp.Ex{"a": 1}, exp.DayInterval, "goqu: Interval count goqu.Ex/ExOr is a condition, not a count"},
		{sqlgen.PostgresTimeStyle, exp.ExOr{"a": 1}, exp.DayInterval, "goqu: Interval count goqu.Ex/ExOr is a condition, not a count"},
		{
			sqlgen.DuckDBTimeStyle, 1.5, exp.SecondInterval,
			"goqu: Interval count must be an integer or an expression, got float64 1.5 for SECOND [dialect=test]",
		},
		{sqlgen.PostgresTimeStyle, jsonFailingValuer{}, exp.DayInterval, "goqu: Interval count: value failed"},
	} {
		b := sb.NewSQLBuilder(false)
		sqlgen.NewExpressionSQLGenerator("test", timeStyleOptions(tc.style)).Generate(b, exp.NewIntervalExpression(tc.count, tc.unit))
		_, _, err := b.ToSQL()
		esgs.EqualError(err, tc.err, "%T", tc.count)
		esgs.True(goqu.IsInvalidExpression(err), "%T", tc.count)
	}

	b := sb.NewSQLBuilder(false)
	sqlgen.NewExpressionSQLGenerator("test", timeStyleOptions(sqlgen.PostgresTimeStyle)).
		Generate(b, exp.NewIntervalExpression(jsonFailingValuer{}, exp.DayInterval))
	_, _, err := b.ToSQL()
	var cause jsonValueError
	esgs.ErrorAs(err, &cause)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_TimeExpressionCastFragment() {
	for _, tc := range []struct {
		style sqlgen.TimeStyle
		val   exp.Expression
		sql   string
	}{
		{sqlgen.SQLServerTimeStyle, exp.NewNowExpression(3), "TRY_CAST{SYSDATETIME{} AS DATETIME2(3)}"},
		{sqlgen.DuckDBTimeStyle, exp.NewUTCTimestampExpression(0), "TRY_CAST{make_timestamp{epoch_us{now{}}} AS TIMESTAMP_S}"},
	} {
		do := timeStyleOptions(tc.style)
		do.LeftParenRune, do.RightParenRune = '{', '}'
		do.CastFragment = []byte("TRY_CAST")
		esgs.assertCases(sqlgen.NewExpressionSQLGenerator("test", do), expressionTestCase{val: tc.val, sql: tc.sql})
	}
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_LikeHintOnlyWhenILikeIsPlainLike() {
	like := exp.NewIdentifierExpression("", "", "a").Like("x%")
	for ilike, want := range map[string]string{
		"LIKE":  "goqu: dialect does not support boolean operator \"like\", use ILike/NotILike [dialect=test]",
		"ILIKE": "goqu: dialect does not support boolean operator \"like\" [dialect=test]",
	} {
		opts := sqlgen.DefaultDialectOptions()
		delete(opts.BooleanOperatorLookup, exp.LikeOp)
		opts.BooleanOperatorLookup[exp.ILikeOp] = []byte(ilike)
		esgs.assertCases(sqlgen.NewExpressionSQLGenerator("test", opts), expressionTestCase{val: like, err: want})
	}
}

func timeStyleOptions(style sqlgen.TimeStyle) *sqlgen.SQLDialectOptions {
	do := sqlgen.DefaultDialectOptions()
	do.TimeStyle = style
	return do
}

type unknownTimeFunction struct{ exp.TimeFunctionExpression }

func (unknownTimeFunction) Function() exp.TimeFunction { return 99 }

type (
	jsonStringValuer      string
	jsonBytesValuer       string
	jsonMarshalingValuer  struct{}
	jsonFailingValuer     struct{}
	jsonValueError        struct{}
	unknownJSONExpression struct{ exp.JSONExpression }
)

func (jsonFailingValuer) Value() (driver.Value, error) { return nil, jsonValueError{} }

func (jsonValueError) Error() string { return "value failed" }

func (v jsonBytesValuer) Value() (driver.Value, error) { return []byte(v), nil }

func (jsonMarshalingValuer) Value() (driver.Value, error) { return "x", nil }

func (jsonMarshalingValuer) MarshalJSON() ([]byte, error) { return []byte(`{"k":1}`), nil }

func (v jsonStringValuer) Value() (driver.Value, error) {
	if v == "" {
		return nil, nil
	}
	return string(v), nil
}

func (unknownJSONExpression) Function() exp.JSONFunction { return 99 }

func (esgs *expressionSQLGeneratorSuite) TestGenerate_JSONExpression() {
	jsonDoc, name, name2 := []byte(`{"a": 1}`), "x", jsonStringValuer("y")
	do := sqlgen.DefaultDialectOptions()
	do.SupportsMySQLJSONFunctions = true
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", do),
		expressionTestCase{val: exp.NewJSONExtractExpression("doc", "$.a"), sql: `JSON_EXTRACT("doc", '$.a')`},
		expressionTestCase{
			val: exp.NewJSONExtractExpression("doc", "$.a"), sql: `JSON_EXTRACT("doc", ?)`, isPrepared: true, args: []interface{}{"$.a"},
		},
		expressionTestCase{val: exp.NewJSONValueExpression("t.doc", "$.a"), sql: `JSON_UNQUOTE(JSON_EXTRACT("t"."doc", '$.a'))`},
		expressionTestCase{
			val: exp.NewJSONValueExpression("doc", "$.a"), sql: `JSON_UNQUOTE(JSON_EXTRACT("doc", ?))`, isPrepared: true,
			args: []interface{}{"$.a"},
		},
		expressionTestCase{val: exp.NewJSONContainsExpression("doc", map[string]int{"a": 1}), sql: `JSON_CONTAINS("doc", '{"a":1}')`},
		expressionTestCase{val: exp.NewJSONContainsExpression("doc", "<a>&", "$.t"), sql: `JSON_CONTAINS("doc", '"<a>&"', '$.t')`},
		expressionTestCase{
			val: exp.NewJSONContainsExpression("doc", "a\x01b", "$.t"), sql: `JSON_CONTAINS("doc", ?, ?)`, isPrepared: true,
			args: []interface{}{`"a\u0001b"`, "$.t"},
		},
		expressionTestCase{val: exp.NewJSONContainsExpression("doc", 1, "$.a", "$.b"), err: "goqu: JSONContains takes at most one path, got 2"},
		expressionTestCase{
			val: exp.NewJSONContainsExpression("doc", math.NaN()), err: "goqu: JSONContains candidate: json: unsupported value: NaN",
		},
		expressionTestCase{
			val: exp.NewJSONContainsExpression("doc", exp.NewIdentifierExpression("", "o", "doc")), sql: `JSON_CONTAINS("doc", "o"."doc")`,
		},
		expressionTestCase{val: exp.NewJSONContainsExpression("doc", []byte(`{"a": 1}`)), sql: `JSON_CONTAINS("doc", '{"a": 1}')`},
		expressionTestCase{val: exp.NewJSONContainsExpression("doc", json.RawMessage(`{"a": 1}`)), sql: `JSON_CONTAINS("doc", '{"a":1}')`},
		expressionTestCase{
			val: exp.NewJSONContainsExpression("doc", []byte("{")), err: "goqu: JSONContains candidate is not valid JSON",
		},
		expressionTestCase{val: exp.NewJSONContainsExpression("doc", jsonStringValuer("x")), sql: `JSON_CONTAINS("doc", '"x"')`},
		expressionTestCase{val: exp.NewJSONContainsExpression("doc", jsonStringValuer("")), sql: `JSON_CONTAINS("doc", 'null')`},
		expressionTestCase{
			val: exp.NewJSONContainsExpression("doc", jsonFailingValuer{}), err: "goqu: JSONContains candidate: value failed",
		},
		expressionTestCase{val: exp.NewJSONContainsExpression("doc", (*jsonStringValuer)(nil)), sql: `JSON_CONTAINS("doc", 'null')`},
		expressionTestCase{val: exp.NewJSONContainsExpression("doc", &name2), sql: `JSON_CONTAINS("doc", '"y"')`},
		expressionTestCase{val: exp.NewJSONContainsExpression("doc", (*pointerValuer)(nil)), sql: `JSON_CONTAINS("doc", '"pointer"')`},
		expressionTestCase{val: exp.NewJSONContainsExpression("doc", []byte(nil)), sql: `JSON_CONTAINS("doc", 'null')`},
		expressionTestCase{
			val: exp.NewJSONContainsExpression("doc", exp.Ex{"name": "bob"}),
			err: "goqu: JSONContains candidate goqu.Ex/ExOr is a condition, not a JSON object; use map[string]any",
		},
		expressionTestCase{
			val: exp.NewJSONContainsExpression("doc", exp.ExOr{"name": "bob"}),
			err: "goqu: JSONContains candidate goqu.Ex/ExOr is a condition, not a JSON object; use map[string]any",
		},
		expressionTestCase{val: exp.NewJSONContainsExpression("doc", &jsonDoc), sql: `JSON_CONTAINS("doc", '{"a": 1}')`},
		expressionTestCase{val: exp.NewJSONContainsExpression("doc", (*[]byte)(nil)), sql: `JSON_CONTAINS("doc", 'null')`},
		expressionTestCase{val: exp.NewJSONContainsExpression("doc", net.ParseIP("1.2.3.4")), sql: `JSON_CONTAINS("doc", '"1.2.3.4"')`},
		expressionTestCase{val: exp.NewJSONContainsExpression("doc", &name), sql: `JSON_CONTAINS("doc", '"x"')`},
		expressionTestCase{
			val: exp.NewJSONContainsExpression("doc", jsonMarshalingValuer{}), sql: `JSON_CONTAINS("doc", '{"k":1}')`,
		},
		expressionTestCase{
			val: exp.NewJSONContainsExpression("doc", jsonBytesValuer(`{"a": 1}`)), sql: `JSON_CONTAINS("doc", '{"a": 1}')`,
		},
		expressionTestCase{
			val: exp.NewJSONContainsExpression("doc", jsonBytesValuer("{")), err: "goqu: JSONContains candidate is not valid JSON",
		},
	)
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{
			val: exp.NewJSONValueExpression("doc", "$.a"),
			err: "goqu: dialect does not support JSONValue [dialect=test]",
		},
		expressionTestCase{
			val: exp.NewJSONExtractExpression("doc", "$.a"), err: "goqu: dialect does not support JSONExtract [dialect=test]",
		},
		expressionTestCase{
			val: exp.NewJSONContainsExpression("doc", 1), err: "goqu: dialect does not support JSONContains [dialect=test]",
		},
		expressionTestCase{
			val: unknownJSONExpression{exp.NewJSONExtractExpression("doc", "$")},
			err: "goqu: unknown JSONFunction 99",
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_JSONExpressionErrorTypes() {
	b := sb.NewSQLBuilder(false)
	sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()).
		Generate(b, unknownJSONExpression{exp.NewJSONExtractExpression("doc", "$")})
	_, _, err := b.ToSQL()
	esgs.EqualError(err, "goqu: unknown JSONFunction 99")
	esgs.True(goqu.IsInvalidExpression(err))

	do := sqlgen.DefaultDialectOptions()
	do.SupportsMySQLJSONFunctions = true
	render := func(e exp.Expression) error {
		b := sb.NewSQLBuilder(false)
		sqlgen.NewExpressionSQLGenerator("test", do).Generate(b, e)
		_, _, err := b.ToSQL()
		return err
	}
	esgs.True(goqu.IsInvalidExpression(render(exp.NewJSONContainsExpression("doc", 1, "$.a", "$.b"))))
	esgs.True(goqu.IsInvalidExpression(render(exp.NewJSONContainsExpression("doc", []byte("{")))))
	esgs.True(goqu.IsInvalidExpression(render(exp.NewJSONContainsExpression("doc", exp.Ex{"a": 1}))))

	err = render(exp.NewJSONContainsExpression("doc", math.NaN()))
	esgs.True(goqu.IsInvalidExpression(err))
	var unsupportedValue *json.UnsupportedValueError
	esgs.ErrorAs(err, &unsupportedValue)

	err = render(exp.NewJSONContainsExpression("doc", jsonFailingValuer{}))
	esgs.True(goqu.IsInvalidExpression(err))
	var valueErr jsonValueError
	esgs.ErrorAs(err, &valueErr)

	err = render(unknownJSONExpression{exp.NewJSONExtractExpression("doc", "$")})
	esgs.EqualError(err, "goqu: unknown JSONFunction 99")
	esgs.True(goqu.IsInvalidExpression(err))
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExcludedExpressionOutsideConflictUpdate() {
	b := sb.NewSQLBuilder(false)
	sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()).
		Generate(b, exp.NewIdentifierExpression("", "", "a").Eq(exp.NewExcludedExpression("a")))
	_, _, err := b.ToSQL()
	esgs.EqualError(err, "goqu: Excluded is only valid in an OnConflict DoUpdate")
	esgs.True(goqu.IsInvalidExpression(err))
	esgs.False(goqu.IsUnsupported(err))
}

// Generates the sql for the WITH clauses for common table expressions (CTE)
func (esgs *expressionSQLGeneratorSuite) TestGenerate_CommonTableExpressionSlice() {
	ae := newTestAppendableExpression(`SELECT * FROM "b"`, emptyArgs, nil, nil)

	cteNoArgs := []exp.CommonTableExpression{
		exp.NewCommonTableExpression(false, "a", ae),
	}
	cteArgs := []exp.CommonTableExpression{
		exp.NewCommonTableExpression(false, "a(x,y)", ae),
	}

	cteRecursiveNoArgs := []exp.CommonTableExpression{
		exp.NewCommonTableExpression(true, "a", ae),
	}
	cteRecursiveArgs := []exp.CommonTableExpression{
		exp.NewCommonTableExpression(true, "a(x,y)", ae),
	}

	allCtes := []exp.CommonTableExpression{
		exp.NewCommonTableExpression(false, "a", ae),
		exp.NewCommonTableExpression(false, "a(x,y)", ae),
	}

	allRecursiveCtes := []exp.CommonTableExpression{
		exp.NewCommonTableExpression(true, "a", ae),
		exp.NewCommonTableExpression(true, "a(x,y)", ae),
	}

	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: cteNoArgs, sql: `WITH a AS (SELECT * FROM "b") `},
		expressionTestCase{val: cteNoArgs, sql: `WITH a AS (SELECT * FROM "b") `, isPrepared: true},

		expressionTestCase{val: cteArgs, sql: `WITH a(x,y) AS (SELECT * FROM "b") `},
		expressionTestCase{val: cteArgs, sql: `WITH a(x,y) AS (SELECT * FROM "b") `, isPrepared: true},

		expressionTestCase{val: cteRecursiveNoArgs, sql: `WITH RECURSIVE a AS (SELECT * FROM "b") `},
		expressionTestCase{val: cteRecursiveNoArgs, sql: `WITH RECURSIVE a AS (SELECT * FROM "b") `, isPrepared: true},

		expressionTestCase{val: cteRecursiveArgs, sql: `WITH RECURSIVE a(x,y) AS (SELECT * FROM "b") `},
		expressionTestCase{val: cteRecursiveArgs, sql: `WITH RECURSIVE a(x,y) AS (SELECT * FROM "b") `, isPrepared: true},

		expressionTestCase{val: allCtes, sql: `WITH a AS (SELECT * FROM "b"), a(x,y) AS (SELECT * FROM "b") `},
		expressionTestCase{val: allCtes, sql: `WITH a AS (SELECT * FROM "b"), a(x,y) AS (SELECT * FROM "b") `, isPrepared: true},

		expressionTestCase{val: allRecursiveCtes, sql: `WITH RECURSIVE a AS (SELECT * FROM "b"), a(x,y) AS (SELECT * FROM "b") `},
		expressionTestCase{
			val:        allRecursiveCtes,
			sql:        `WITH RECURSIVE a AS (SELECT * FROM "b"), a(x,y) AS (SELECT * FROM "b") `,
			isPrepared: true,
		},
	)
	opts := sqlgen.DefaultDialectOptions()
	opts.SupportsWithCTE = false
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", opts),
		expressionTestCase{val: cteNoArgs, err: "goqu: dialect does not support CTE WITH clause [dialect=test]"},
		expressionTestCase{val: cteNoArgs, err: "goqu: dialect does not support CTE WITH clause [dialect=test]", isPrepared: true},

		expressionTestCase{val: cteArgs, err: "goqu: dialect does not support CTE WITH clause [dialect=test]"},
		expressionTestCase{val: cteArgs, err: "goqu: dialect does not support CTE WITH clause [dialect=test]", isPrepared: true},

		expressionTestCase{val: cteRecursiveNoArgs, err: "goqu: dialect does not support CTE WITH clause [dialect=test]"},
		expressionTestCase{val: cteRecursiveNoArgs, err: "goqu: dialect does not support CTE WITH clause [dialect=test]", isPrepared: true},

		expressionTestCase{val: cteRecursiveArgs, err: "goqu: dialect does not support CTE WITH clause [dialect=test]"},
		expressionTestCase{val: cteRecursiveArgs, err: "goqu: dialect does not support CTE WITH clause [dialect=test]", isPrepared: true},
	)
	opts = sqlgen.DefaultDialectOptions()
	opts.SupportsWithCTERecursive = false
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", opts),
		expressionTestCase{val: cteNoArgs, sql: `WITH a AS (SELECT * FROM "b") `},
		expressionTestCase{val: cteNoArgs, sql: `WITH a AS (SELECT * FROM "b") `, isPrepared: true},

		expressionTestCase{val: cteArgs, sql: `WITH a(x,y) AS (SELECT * FROM "b") `},
		expressionTestCase{val: cteArgs, sql: `WITH a(x,y) AS (SELECT * FROM "b") `, isPrepared: true},

		expressionTestCase{
			val: cteRecursiveNoArgs,
			err: "goqu: dialect does not support CTE WITH RECURSIVE clause [dialect=test]",
		},
		expressionTestCase{
			val:        cteRecursiveNoArgs,
			err:        "goqu: dialect does not support CTE WITH RECURSIVE clause [dialect=test]",
			isPrepared: true,
		},

		expressionTestCase{
			val: cteRecursiveArgs,
			err: "goqu: dialect does not support CTE WITH RECURSIVE clause [dialect=test]",
		},
		expressionTestCase{
			val:        cteRecursiveArgs,
			err:        "goqu: dialect does not support CTE WITH RECURSIVE clause [dialect=test]",
			isPrepared: true,
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_CommonTableExpression() {
	ae := newTestAppendableExpression(`SELECT * FROM "b"`, emptyArgs, nil, nil)

	cteNoArgs := exp.NewCommonTableExpression(false, "a", ae)
	cteArgs := exp.NewCommonTableExpression(false, "a(x,y)", ae)

	cteRecursiveNoArgs := exp.NewCommonTableExpression(true, "a", ae)
	cteRecursiveArgs := exp.NewCommonTableExpression(true, "a(x,y)", ae)

	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: cteNoArgs, sql: `a AS (SELECT * FROM "b")`},
		expressionTestCase{val: cteNoArgs, sql: `a AS (SELECT * FROM "b")`, isPrepared: true},

		expressionTestCase{val: cteArgs, sql: `a(x,y) AS (SELECT * FROM "b")`},
		expressionTestCase{val: cteArgs, sql: `a(x,y) AS (SELECT * FROM "b")`, isPrepared: true},

		expressionTestCase{val: cteRecursiveNoArgs, sql: `a AS (SELECT * FROM "b")`},
		expressionTestCase{val: cteRecursiveNoArgs, sql: `a AS (SELECT * FROM "b")`, isPrepared: true},

		expressionTestCase{val: cteRecursiveArgs, sql: `a(x,y) AS (SELECT * FROM "b")`},
		expressionTestCase{val: cteRecursiveArgs, sql: `a(x,y) AS (SELECT * FROM "b")`, isPrepared: true},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_CompoundExpression() {
	ae := newTestAppendableExpression(`SELECT * FROM "b"`, emptyArgs, nil, nil)

	u := exp.NewCompoundExpression(exp.UnionCompoundType, ae)
	ua := exp.NewCompoundExpression(exp.UnionAllCompoundType, ae)

	i := exp.NewCompoundExpression(exp.IntersectCompoundType, ae)
	ia := exp.NewCompoundExpression(exp.IntersectAllCompoundType, ae)

	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: u, sql: ` UNION (SELECT * FROM "b")`},
		expressionTestCase{val: u, sql: ` UNION (SELECT * FROM "b")`, isPrepared: true},

		expressionTestCase{val: ua, sql: ` UNION ALL (SELECT * FROM "b")`},
		expressionTestCase{val: ua, sql: ` UNION ALL (SELECT * FROM "b")`, isPrepared: true},

		expressionTestCase{val: i, sql: ` INTERSECT (SELECT * FROM "b")`},
		expressionTestCase{val: i, sql: ` INTERSECT (SELECT * FROM "b")`, isPrepared: true},

		expressionTestCase{val: ia, sql: ` INTERSECT ALL (SELECT * FROM "b")`},
		expressionTestCase{val: ia, sql: ` INTERSECT ALL (SELECT * FROM "b")`, isPrepared: true},
	)

	opts := sqlgen.DefaultDialectOptions()
	opts.WrapCompoundsInParens = false
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", opts),
		expressionTestCase{val: u, sql: ` UNION SELECT * FROM "b"`},
		expressionTestCase{val: u, sql: ` UNION SELECT * FROM "b"`, isPrepared: true},

		expressionTestCase{val: ua, sql: ` UNION ALL SELECT * FROM "b"`},
		expressionTestCase{val: ua, sql: ` UNION ALL SELECT * FROM "b"`, isPrepared: true},

		expressionTestCase{val: i, sql: ` INTERSECT SELECT * FROM "b"`},
		expressionTestCase{val: i, sql: ` INTERSECT SELECT * FROM "b"`, isPrepared: true},

		expressionTestCase{val: ia, sql: ` INTERSECT ALL SELECT * FROM "b"`},
		expressionTestCase{val: ia, sql: ` INTERSECT ALL SELECT * FROM "b"`, isPrepared: true},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_IdentifierExpression() {
	col := exp.NewIdentifierExpression("", "", "col")
	colStar := exp.NewIdentifierExpression("", "", "*")
	table := exp.NewIdentifierExpression("", "table", "")
	schema := exp.NewIdentifierExpression("schema", "", "")
	tableCol := exp.NewIdentifierExpression("", "table", "col")
	schemaTableCol := exp.NewIdentifierExpression("schema", "table", "col")

	parsedCol := exp.ParseIdentifier("col")
	parsedTableCol := exp.ParseIdentifier("table.col")
	parsedSchemaTableCol := exp.ParseIdentifier("schema.table.col")

	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{
			val: exp.NewIdentifierExpression("", "", ""),
			err: `goqu: an empty identifier was encountered, please specify a "schema", "table" or "column"`,
		},
		expressionTestCase{
			val: exp.NewIdentifierExpression("", "", nil),
			err: `goqu: an empty identifier was encountered, please specify a "schema", "table" or "column"`,
		},
		expressionTestCase{
			val: exp.NewIdentifierExpression("", "", false),
			err: `goqu: unsupported column type bool, a string or LiteralExpression is required`,
		},

		expressionTestCase{val: col, sql: `"col"`},
		expressionTestCase{val: col, sql: `"col"`, isPrepared: true},

		expressionTestCase{val: exp.NewIdentifierExpression("", "", `a" OR 1=1 --`), sql: `"a"" OR 1=1 --"`},
		expressionTestCase{val: exp.NewIdentifierExpression("", "", `a" OR 1=1 --`), sql: `"a"" OR 1=1 --"`, isPrepared: true},
		expressionTestCase{val: exp.NewIdentifierExpression(`s"`, `t"`, `c"`), sql: `"s"""."t"""."c"""`},
		expressionTestCase{val: exp.NewIdentifierExpression(`s"`, `t"`, `c"`), sql: `"s"""."t"""."c"""`, isPrepared: true},
		expressionTestCase{val: exp.NewIdentifierExpression("", "", "a`b"), sql: "\"a`b\""},
		expressionTestCase{val: exp.NewIdentifierExpression("", "", "a`b"), sql: "\"a`b\"", isPrepared: true},

		expressionTestCase{val: col.Table("table"), sql: `"table"."col"`},
		expressionTestCase{val: col.Table("table"), sql: `"table"."col"`, isPrepared: true},

		expressionTestCase{val: col.Table("table").Schema("schema"), sql: `"schema"."table"."col"`},
		expressionTestCase{val: col.Table("table").Schema("schema"), sql: `"schema"."table"."col"`, isPrepared: true},

		expressionTestCase{val: colStar, sql: `*`},
		expressionTestCase{val: colStar, sql: `*`, isPrepared: true},

		expressionTestCase{val: colStar.Table("table"), sql: `"table".*`},
		expressionTestCase{val: colStar.Table("table"), sql: `"table".*`, isPrepared: true},

		expressionTestCase{val: colStar.Table("table").Schema("schema"), sql: `"schema"."table".*`},
		expressionTestCase{val: colStar.Table("table").Schema("schema"), sql: `"schema"."table".*`, isPrepared: true},

		expressionTestCase{val: table, sql: `"table"`},
		expressionTestCase{val: table, sql: `"table"`, isPrepared: true},

		expressionTestCase{val: table.Col("col"), sql: `"table"."col"`},
		expressionTestCase{val: table.Col("col"), sql: `"table"."col"`, isPrepared: true},

		expressionTestCase{val: table.Col(nil), sql: `"table"`},
		expressionTestCase{val: table.Col(nil), sql: `"table"`, isPrepared: true},

		expressionTestCase{val: table.Col("*"), sql: `"table".*`},
		expressionTestCase{val: table.Col("*"), sql: `"table".*`, isPrepared: true},

		expressionTestCase{val: table.Schema("schema").Col("col"), sql: `"schema"."table"."col"`},
		expressionTestCase{val: table.Schema("schema").Col("col"), sql: `"schema"."table"."col"`, isPrepared: true},

		expressionTestCase{val: schema, sql: `"schema"`},
		expressionTestCase{val: schema, sql: `"schema"`, isPrepared: true},

		expressionTestCase{val: schema.Table("table"), sql: `"schema"."table"`},
		expressionTestCase{val: schema.Table("table"), sql: `"schema"."table"`, isPrepared: true},

		expressionTestCase{val: schema.Table("table").Col("col"), sql: `"schema"."table"."col"`},
		expressionTestCase{val: schema.Table("table").Col("col"), sql: `"schema"."table"."col"`, isPrepared: true},

		expressionTestCase{val: schema.Table("table").Col(nil), sql: `"schema"."table"`},
		expressionTestCase{val: schema.Table("table").Col(nil), sql: `"schema"."table"`, isPrepared: true},

		expressionTestCase{val: schema.Table("table").Col("*"), sql: `"schema"."table".*`},
		expressionTestCase{val: schema.Table("table").Col("*"), sql: `"schema"."table".*`, isPrepared: true},

		expressionTestCase{val: tableCol, sql: `"table"."col"`},
		expressionTestCase{val: tableCol, sql: `"table"."col"`, isPrepared: true},

		expressionTestCase{val: schemaTableCol, sql: `"schema"."table"."col"`},
		expressionTestCase{val: schemaTableCol, sql: `"schema"."table"."col"`, isPrepared: true},

		expressionTestCase{val: parsedCol, sql: `"col"`},
		expressionTestCase{val: parsedCol, sql: `"col"`, isPrepared: true},

		expressionTestCase{val: parsedTableCol, sql: `"table"."col"`},
		expressionTestCase{val: parsedTableCol, sql: `"table"."col"`, isPrepared: true},

		expressionTestCase{val: parsedSchemaTableCol, sql: `"schema"."table"."col"`},
		expressionTestCase{val: parsedSchemaTableCol, sql: `"schema"."table"."col"`, isPrepared: true},
	)

	backtickOpts := sqlgen.DefaultDialectOptions()
	backtickOpts.QuoteRune = '`'
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", backtickOpts),
		expressionTestCase{val: exp.NewIdentifierExpression("", "t`", "a` OR 1=1 -- "), sql: "`t```.`a`` OR 1=1 -- `"},
		expressionTestCase{val: exp.NewIdentifierExpression("", "t`", "a` OR 1=1 -- "), sql: "`t```.`a`` OR 1=1 -- `", isPrepared: true},
		expressionTestCase{val: exp.NewIdentifierExpression("", "", `a"b`), sql: "`a\"b`"},
		expressionTestCase{val: exp.NewIdentifierExpression("", "", `a"b`), sql: "`a\"b`", isPrepared: true},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_LateralExpression() {
	lateralExp := exp.NewLateralExpression(newTestAppendableExpression(`SELECT * FROM "test"`, emptyArgs, nil, nil))

	do := sqlgen.DefaultDialectOptions()
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", do),
		expressionTestCase{val: lateralExp, sql: `LATERAL (SELECT * FROM "test")`},
		expressionTestCase{val: lateralExp, sql: `LATERAL (SELECT * FROM "test")`, isPrepared: true},
	)

	do = sqlgen.DefaultDialectOptions()
	do.LateralFragment = []byte("lateral ")
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", do),
		expressionTestCase{val: lateralExp, sql: `lateral (SELECT * FROM "test")`},
		expressionTestCase{val: lateralExp, sql: `lateral (SELECT * FROM "test")`, isPrepared: true},
	)
	do = sqlgen.DefaultDialectOptions()
	do.SupportsLateral = false
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", do),
		expressionTestCase{val: lateralExp, err: "goqu: dialect does not support lateral expressions [dialect=test]"},
		expressionTestCase{val: lateralExp, err: "goqu: dialect does not support lateral expressions [dialect=test]", isPrepared: true},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_CaseExpression() {
	ident := exp.NewIdentifierExpression("", "", "col")
	valueCase := exp.NewCaseExpression().
		Value(ident).
		When(true, "one").
		When(false, "two")
	valueElseCase := exp.NewCaseExpression().
		Value(ident).
		When(1, "one").
		When(2, "two").
		Else("three")
	searchCase := exp.NewCaseExpression().
		When(ident.Gt(1), exp.NewLiteralExpression("? - 1", ident)).
		When(ident.Lt(0), exp.NewLiteralExpression("? + 1", ident))
	searchElseCase := exp.NewCaseExpression().
		When(ident.Gt(1), exp.NewLiteralExpression("? - 1", ident)).
		When(ident.Lt(0), exp.NewLiteralExpression("? + 1", ident)).
		Else(ident)

	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: valueCase, sql: `CASE "col" WHEN TRUE THEN 'one' WHEN FALSE THEN 'two' END`},
		expressionTestCase{
			val:        valueCase,
			sql:        `CASE "col" WHEN ? THEN ? WHEN ? THEN ? END`,
			isPrepared: true,
			args:       []interface{}{true, "one", false, "two"},
		},

		expressionTestCase{val: valueElseCase, sql: `CASE "col" WHEN 1 THEN 'one' WHEN 2 THEN 'two' ELSE 'three' END`},
		expressionTestCase{
			val:        valueElseCase,
			sql:        `CASE "col" WHEN ? THEN ? WHEN ? THEN ? ELSE ? END`,
			isPrepared: true,
			args:       []interface{}{int64(1), "one", int64(2), "two", "three"},
		},

		expressionTestCase{val: searchCase, sql: `CASE  WHEN ("col" > 1) THEN "col" - 1 WHEN ("col" < 0) THEN "col" + 1 END`},
		expressionTestCase{
			val:        searchCase,
			sql:        `CASE  WHEN ("col" > ?) THEN "col" - 1 WHEN ("col" < ?) THEN "col" + 1 END`,
			isPrepared: true,
			args:       []interface{}{int64(1), int64(0)},
		},

		expressionTestCase{val: searchElseCase, sql: `CASE  WHEN ("col" > 1) THEN "col" - 1 WHEN ("col" < 0) THEN "col" + 1 ELSE "col" END`},
		expressionTestCase{
			val:        searchElseCase,
			sql:        `CASE  WHEN ("col" > ?) THEN "col" - 1 WHEN ("col" < ?) THEN "col" + 1 ELSE "col" END`,
			isPrepared: true,
			args:       []interface{}{int64(1), int64(0)},
		},
		expressionTestCase{
			val: exp.NewCaseExpression(),
			err: "goqu: when conditions not found for case statement",
		},
	)

	opts := sqlgen.DefaultDialectOptions()
	opts.CaseFragment = []byte("case ")
	opts.WhenFragment = []byte(" when ")
	opts.ThenFragment = []byte(" then ")
	opts.ElseFragment = []byte(" else ")
	opts.EndFragment = []byte(" end")
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", opts),
		expressionTestCase{val: valueCase, sql: `case "col" when TRUE then 'one' when FALSE then 'two' end`},
		expressionTestCase{
			val:        valueCase,
			sql:        `case "col" when ? then ? when ? then ? end`,
			isPrepared: true,
			args:       []interface{}{true, "one", false, "two"},
		},

		expressionTestCase{val: valueElseCase, sql: `case "col" when 1 then 'one' when 2 then 'two' else 'three' end`},
		expressionTestCase{
			val:        valueElseCase,
			sql:        `case "col" when ? then ? when ? then ? else ? end`,
			isPrepared: true,
			args:       []interface{}{int64(1), "one", int64(2), "two", "three"},
		},

		expressionTestCase{val: searchCase, sql: `case  when ("col" > 1) then "col" - 1 when ("col" < 0) then "col" + 1 end`},
		expressionTestCase{
			val:        searchCase,
			sql:        `case  when ("col" > ?) then "col" - 1 when ("col" < ?) then "col" + 1 end`,
			isPrepared: true,
			args:       []interface{}{int64(1), int64(0)},
		},

		expressionTestCase{val: searchElseCase, sql: `case  when ("col" > 1) then "col" - 1 when ("col" < 0) then "col" + 1 else "col" end`},
		expressionTestCase{
			val:        searchElseCase,
			sql:        `case  when ("col" > ?) then "col" - 1 when ("col" < ?) then "col" + 1 else "col" end`,
			isPrepared: true,
			args:       []interface{}{int64(1), int64(0)},
		},
		expressionTestCase{
			val: exp.NewCaseExpression(),
			err: "goqu: when conditions not found for case statement",
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExpressionMap() {
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: exp.Ex{"a": 1}, sql: `("a" = 1)`},
		expressionTestCase{val: exp.Ex{"a": 1}, sql: `("a" = ?)`, isPrepared: true, args: []interface{}{int64(1)}},

		expressionTestCase{val: exp.Ex{"a": true}, sql: `("a" IS TRUE)`},
		expressionTestCase{val: exp.Ex{"a": true}, sql: `("a" IS TRUE)`, isPrepared: true},

		expressionTestCase{val: exp.Ex{"a": false}, sql: `("a" IS FALSE)`},
		expressionTestCase{val: exp.Ex{"a": false}, sql: `("a" IS FALSE)`, isPrepared: true},

		expressionTestCase{val: exp.Ex{"a": nil}, sql: `("a" IS NULL)`},
		expressionTestCase{val: exp.Ex{"a": nil}, sql: `("a" IS NULL)`, isPrepared: true},

		expressionTestCase{val: exp.Ex{"a": []string{"a", "b", "c"}}, sql: `("a" IN ('a', 'b', 'c'))`},
		expressionTestCase{
			val:        exp.Ex{"a": []string{"a", "b", "c"}},
			sql:        `("a" IN (?, ?, ?))`,
			isPrepared: true,
			args:       []interface{}{"a", "b", "c"},
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExpressionMapWithABadOp() {
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{
			val: exp.Ex{"a": exp.Op{"badOp": true}},
			err: `goqu: unsupported Op key "badOp"`,
		},
		expressionTestCase{
			val:        exp.Ex{"a": exp.Op{"badOp": true}},
			isPrepared: true,
			err:        `goqu: unsupported Op key "badOp"`,
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExpressionMapWithNeqOp() {
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: exp.Ex{"a": exp.Op{"neq": 1}}, sql: `("a" != 1)`},
		expressionTestCase{val: exp.Ex{"a": exp.Op{"neq": 1}}, sql: `("a" != ?)`, isPrepared: true, args: []interface{}{
			int64(1),
		}},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExpressionMapWithIsNotOp() {
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: exp.Ex{"a": exp.Op{"isnot": true}}, sql: `("a" IS NOT TRUE)`},
		expressionTestCase{val: exp.Ex{"a": exp.Op{"isnot": true}}, sql: `("a" IS NOT TRUE)`, isPrepared: true},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExpressionMapWithGtOp() {
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: exp.Ex{"a": exp.Op{"gt": 1}}, sql: `("a" > 1)`},
		expressionTestCase{val: exp.Ex{"a": exp.Op{"gt": 1}}, sql: `("a" > ?)`, isPrepared: true, args: []interface{}{
			int64(1),
		}},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExpressionMapWithGteOp() {
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: exp.Ex{"a": exp.Op{"gte": 1}}, sql: `("a" >= 1)`},
		expressionTestCase{val: exp.Ex{"a": exp.Op{"gte": 1}}, sql: `("a" >= ?)`, isPrepared: true, args: []interface{}{
			int64(1),
		}},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExpressionMapWithLtOp() {
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: exp.Ex{"a": exp.Op{"lt": 1}}, sql: `("a" < 1)`},
		expressionTestCase{val: exp.Ex{"a": exp.Op{"lt": 1}}, sql: `("a" < ?)`, isPrepared: true, args: []interface{}{
			int64(1),
		}},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExpressionMapWithLteOp() {
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: exp.Ex{"a": exp.Op{"lte": 1}}, sql: `("a" <= 1)`},
		expressionTestCase{val: exp.Ex{"a": exp.Op{"lte": 1}}, sql: `("a" <= ?)`, isPrepared: true, args: []interface{}{
			int64(1),
		}},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExpressionMapWithLikeOp() {
	re := regexp.MustCompile("[ab]")
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: exp.Ex{"a": exp.Op{"like": "a%"}}, sql: `("a" LIKE 'a%')`},
		expressionTestCase{
			val:        exp.Ex{"a": exp.Op{"like": "a%"}},
			sql:        `("a" LIKE ?)`,
			isPrepared: true,
			args:       []interface{}{"a%"},
		},

		expressionTestCase{val: exp.Ex{"a": exp.Op{"like": re}}, sql: `("a" ~ '[ab]')`},
		expressionTestCase{
			val:        exp.Ex{"a": exp.Op{"like": re}},
			sql:        `("a" ~ ?)`,
			isPrepared: true,
			args:       []interface{}{"[ab]"},
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExpressionMapWithNotLikeOp() {
	re := regexp.MustCompile("[ab]")
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: exp.Ex{"a": exp.Op{"notLike": "a%"}}, sql: `("a" NOT LIKE 'a%')`},
		expressionTestCase{
			val:        exp.Ex{"a": exp.Op{"notLike": "a%"}},
			sql:        `("a" NOT LIKE ?)`,
			isPrepared: true,
			args:       []interface{}{"a%"},
		},

		expressionTestCase{val: exp.Ex{"a": exp.Op{"notLike": re}}, sql: `("a" !~ '[ab]')`},
		expressionTestCase{
			val:        exp.Ex{"a": exp.Op{"notLike": re}},
			sql:        `("a" !~ ?)`,
			isPrepared: true,
			args:       []interface{}{"[ab]"},
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExpressionMapWithILikeOp() {
	re := regexp.MustCompile("[ab]")
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: exp.Ex{"a": exp.Op{"iLike": "a%"}}, sql: `("a" ILIKE 'a%')`},
		expressionTestCase{
			val:        exp.Ex{"a": exp.Op{"iLike": "a%"}},
			sql:        `("a" ILIKE ?)`,
			isPrepared: true,
			args:       []interface{}{"a%"},
		},

		expressionTestCase{val: exp.Ex{"a": exp.Op{"iLike": re}}, sql: `("a" ~* '[ab]')`},
		expressionTestCase{
			val:        exp.Ex{"a": exp.Op{"iLike": re}},
			sql:        `("a" ~* ?)`,
			isPrepared: true,
			args:       []interface{}{"[ab]"},
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExpressionMapWithNotILikeOp() {
	re := regexp.MustCompile("[ab]")
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: exp.Ex{"a": exp.Op{"notILike": "a%"}}, sql: `("a" NOT ILIKE 'a%')`},
		expressionTestCase{
			val:        exp.Ex{"a": exp.Op{"notILike": "a%"}},
			sql:        `("a" NOT ILIKE ?)`,
			isPrepared: true,
			args:       []interface{}{"a%"},
		},

		expressionTestCase{val: exp.Ex{"a": exp.Op{"notILike": re}}, sql: `("a" !~* '[ab]')`},
		expressionTestCase{
			val:        exp.Ex{"a": exp.Op{"notILike": re}},
			sql:        `("a" !~* ?)`,
			isPrepared: true,
			args:       []interface{}{"[ab]"},
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExpressionMapWithRegExpLikeOp() {
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),

		expressionTestCase{val: exp.Ex{"a": exp.Op{"regexpLike": "[ab]"}}, sql: `("a" ~ '[ab]')`},
		expressionTestCase{
			val:        exp.Ex{"a": exp.Op{"regexpLike": "[ab]"}},
			sql:        `("a" ~ ?)`,
			isPrepared: true,
			args:       []interface{}{"[ab]"},
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExpressionMapWithRegExpILikeOp() {
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: exp.Ex{"a": exp.Op{"regexpILike": "[ab]"}}, sql: `("a" ~* '[ab]')`},
		expressionTestCase{
			val:        exp.Ex{"a": exp.Op{"regexpILike": "[ab]"}},
			sql:        `("a" ~* ?)`,
			isPrepared: true,
			args:       []interface{}{"[ab]"},
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExpressionMapWithRegExpNotLikeOp() {
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: exp.Ex{"a": exp.Op{"regexpNotLike": "[ab]"}}, sql: `("a" !~ '[ab]')`},
		expressionTestCase{
			val:        exp.Ex{"a": exp.Op{"regexpNotLike": "[ab]"}},
			sql:        `("a" !~ ?)`,
			isPrepared: true,
			args:       []interface{}{"[ab]"},
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExpressionMapWithInOp() {
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: exp.Ex{"a": exp.Op{"in": []string{"a", "b", "c"}}}, sql: `("a" IN ('a', 'b', 'c'))`},
		expressionTestCase{
			val:        exp.Ex{"a": exp.Op{"in": []string{"a", "b", "c"}}},
			sql:        `("a" IN (?, ?, ?))`,
			isPrepared: true,
			args:       []interface{}{"a", "b", "c"},
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExpressionMapWithNotInOp() {
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{
			val: exp.Ex{"a": exp.Op{"notIn": []string{"a", "b", "c"}}},
			sql: `("a" NOT IN ('a', 'b', 'c'))`,
		},
		expressionTestCase{
			val:        exp.Ex{"a": exp.Op{"notIn": []string{"a", "b", "c"}}},
			sql:        `("a" NOT IN (?, ?, ?))`,
			isPrepared: true,
			args:       []interface{}{"a", "b", "c"},
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExpressionMapBetweenOp() {
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{
			val: exp.Ex{"a": exp.Op{"between": exp.NewRangeVal("aaa", "zzz")}},
			sql: `("a" BETWEEN 'aaa' AND 'zzz')`,
		},
		expressionTestCase{
			val:        exp.Ex{"a": exp.Op{"between": exp.NewRangeVal("aaa", "zzz")}},
			sql:        `("a" BETWEEN ? AND ?)`,
			isPrepared: true,
			args:       []interface{}{"aaa", "zzz"},
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExpressionMapNotBetweenOp() {
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{
			val: exp.Ex{"a": exp.Op{"notBetween": exp.NewRangeVal("aaa", "zzz")}},
			sql: `("a" NOT BETWEEN 'aaa' AND 'zzz')`,
		},
		expressionTestCase{
			val:        exp.Ex{"a": exp.Op{"notBetween": exp.NewRangeVal("aaa", "zzz")}},
			sql:        `("a" NOT BETWEEN ? AND ?)`,
			isPrepared: true,
			args:       []interface{}{"aaa", "zzz"},
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExpressionMapIsOp() {
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("test", sqlgen.DefaultDialectOptions()),
		expressionTestCase{
			val: exp.Ex{"a": exp.Op{"is": nil, "eq": 10}},
			sql: `(("a" = 10) OR ("a" IS NULL))`,
		},
		expressionTestCase{
			val:        exp.Ex{"a": exp.Op{"is": nil, "eq": 10}},
			sql:        `(("a" = ?) OR ("a" IS NULL))`,
			isPrepared: true,
			args:       []interface{}{int64(10)},
		},
	)
}

func (esgs *expressionSQLGeneratorSuite) TestGenerate_ExpressionOrMap() {
	esgs.assertCases(
		sqlgen.NewExpressionSQLGenerator("default", sqlgen.DefaultDialectOptions()),
		expressionTestCase{val: exp.ExOr{"a": exp.Op{"regexpLike": "[ab]"}}, sql: `("a" ~ '[ab]')`},
		expressionTestCase{
			val:        exp.ExOr{"a": exp.Op{"regexpLike": "[ab]"}},
			sql:        `("a" ~ ?)`,
			isPrepared: true,
			args:       []interface{}{"[ab]"},
		},

		expressionTestCase{val: exp.ExOr{"a": exp.Op{"regexpNotLike": "[ab]"}}, sql: `("a" !~ '[ab]')`},
		expressionTestCase{
			val:        exp.ExOr{"a": exp.Op{"regexpNotLike": "[ab]"}},
			sql:        `("a" !~ ?)`,
			isPrepared: true,
			args:       []interface{}{"[ab]"},
		},

		expressionTestCase{val: exp.ExOr{"a": exp.Op{"regexpILike": "[ab]"}}, sql: `("a" ~* '[ab]')`},
		expressionTestCase{
			val:        exp.ExOr{"a": exp.Op{"regexpILike": "[ab]"}},
			sql:        `("a" ~* ?)`,
			isPrepared: true,
			args:       []interface{}{"[ab]"},
		},
		expressionTestCase{val: exp.ExOr{"a": exp.Op{"regexpNotILike": "[ab]"}}, sql: `("a" !~* '[ab]')`},
		expressionTestCase{
			val:        exp.ExOr{"a": exp.Op{"regexpNotILike": "[ab]"}},
			sql:        `("a" !~* ?)`,
			isPrepared: true,
			args:       []interface{}{"[ab]"},
		},

		expressionTestCase{
			val: exp.ExOr{"a": exp.Op{"badOp": true}},
			err: `goqu: unsupported Op key "badOp"`,
		},
		expressionTestCase{
			val:        exp.ExOr{"a": exp.Op{"badOp": true}},
			isPrepared: true,
			err:        `goqu: unsupported Op key "badOp"`,
		},

		expressionTestCase{val: exp.ExOr{"a": 1, "b": true}, sql: `(("a" = 1) OR ("b" IS TRUE))`},
		expressionTestCase{
			val:        exp.ExOr{"a": 1, "b": true},
			sql:        `(("a" = ?) OR ("b" IS TRUE))`,
			isPrepared: true,
			args:       []interface{}{int64(1)},
		},

		expressionTestCase{
			val: exp.ExOr{"a": 1, "b": []string{"a", "b", "c"}},
			sql: `(("a" = 1) OR ("b" IN ('a', 'b', 'c')))`,
		},
		expressionTestCase{
			val:        exp.ExOr{"a": 1, "b": []string{"a", "b", "c"}},
			sql:        `(("a" = ?) OR ("b" IN (?, ?, ?)))`,
			isPrepared: true,
			args:       []interface{}{int64(1), "a", "b", "c"},
		},
	)
}

func TestExpressionSQLGenerator(t *testing.T) {
	suite.Run(t, new(expressionSQLGeneratorSuite))
}

func ptrTo[T any](v T) *T { return &v }
