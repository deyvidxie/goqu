package sqlgen_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/sb"
	"github.com/doug-martin/goqu/v9/sqlgen"
	"github.com/stretchr/testify/suite"
)

type (
	updateTestCase struct {
		clause     exp.UpdateClauses
		sql        string
		isPrepared bool
		args       []interface{}
		err        string
	}
	updateSQLGeneratorSuite struct {
		baseSQLGeneratorSuite
	}
)

func (usgs *updateSQLGeneratorSuite) assertCases(usg sqlgen.UpdateSQLGenerator, testCases ...updateTestCase) {
	for _, tc := range testCases {
		b := sb.NewSQLBuilder(tc.isPrepared)
		usg.Generate(b, tc.clause)
		switch {
		case tc.err != "":
			usgs.assertErrorSQL(b, tc.err)
		case tc.isPrepared:
			usgs.assertPreparedSQL(b, tc.sql, tc.args)
		default:
			usgs.assertNotPreparedSQL(b, tc.sql)
		}
	}
}

func (usgs *updateSQLGeneratorSuite) TestDialect() {
	opts := sqlgen.DefaultDialectOptions()
	d := sqlgen.NewUpdateSQLGenerator("test", "db", opts)
	usgs.Equal("test", d.Dialect())

	opts2 := sqlgen.DefaultDialectOptions()
	d2 := sqlgen.NewUpdateSQLGenerator("test2", "db", opts2)
	usgs.Equal("test2", d2.Dialect())
}

func (usgs *updateSQLGeneratorSuite) TestGenerate_unsupportedFragment() {
	opts := sqlgen.DefaultDialectOptions()
	opts.UpdateSQLOrder = []sqlgen.SQLFragmentType{sqlgen.InsertBeingSQLFragment}

	uc := exp.NewUpdateClauses().
		SetTable(exp.NewIdentifierExpression("", "test", "")).
		SetSetValues(exp.Record{"a": "b", "b": "c"})
	expectedErr := "goqu: dialect does not support UPDATE SQL fragment InsertBeingSQLFragment [dialect=test]"
	usgs.assertCases(
		sqlgen.NewUpdateSQLGenerator("test", "db", opts),
		updateTestCase{clause: uc, err: expectedErr},
		updateTestCase{clause: uc, err: expectedErr, isPrepared: true},
	)
}

func (usgs *updateSQLGeneratorSuite) TestGenerate_empty() {
	uc := exp.NewUpdateClauses()
	usgs.assertCases(
		sqlgen.NewUpdateSQLGenerator("test", "db", sqlgen.DefaultDialectOptions()),
		updateTestCase{clause: uc, err: "goqu: no source found when generating update sql"},
		updateTestCase{clause: uc, err: "goqu: no source found when generating update sql", isPrepared: true},
	)
}

func (usgs *updateSQLGeneratorSuite) TestGenerate_withBadUpdateValues() {
	uc := exp.NewUpdateClauses().
		SetTable(exp.NewIdentifierExpression("", "test", "")).
		SetSetValues(true)

	expectedErr := "goqu: unsupported update value type bool, a map, goqu.Record or struct is required"
	usgs.assertCases(
		sqlgen.NewUpdateSQLGenerator("test", "db", sqlgen.DefaultDialectOptions()),
		updateTestCase{clause: uc, err: expectedErr},
		updateTestCase{clause: uc, err: expectedErr, isPrepared: true},
	)
}

func (usgs *updateSQLGeneratorSuite) TestGenerate_valuePositions() {
	opts := sqlgen.DefaultDialectOptions()
	opts.DefaultFragment = []byte("default")
	uc := exp.NewUpdateClauses().SetTable(exp.NewIdentifierExpression("", "test", ""))
	set := func(val interface{}) exp.UpdateClauses { return uc.SetSetValues(exp.Record{"a": val}) }

	usgs.assertCases(
		sqlgen.NewUpdateSQLGenerator("test", "db", opts),
		updateTestCase{clause: set(exp.Default()), sql: `UPDATE "test" SET "a"=default`},
		updateTestCase{clause: set(exp.Default()), sql: `UPDATE "test" SET "a"=default`, isPrepared: true},
		updateTestCase{
			clause: uc.SetSetValues(exp.Record{"a": 1}).WhereAppend(exp.NewIdentifierExpression("", "", "a").Eq(exp.Default())),
			err:    "goqu: Default is only valid as an INSERT or UPDATE SET value",
		},
		updateTestCase{clause: set([]byte("x")), sql: `UPDATE "test" SET "a"='x'`},
		updateTestCase{clause: set(arrayLiteralValuer{1}), sql: `UPDATE "test" SET "a"='{1}'`},
		updateTestCase{clause: set(arrayLiteralValuer{1}), sql: `UPDATE "test" SET "a"=?`, isPrepared: true, args: []interface{}{"{1}"}},
		updateTestCase{
			clause: set([]int{}),
			err:    "goqu: a list ([]int) is not an INSERT or UPDATE value; wrap it in a driver.Valuer to store it as one value",
		},
		updateTestCase{
			clause: set([2]string{"x", "y"}),
			err:    "goqu: a list ([2]string) is not an INSERT or UPDATE value; wrap it in a driver.Valuer to store it as one value",
		},
	)
}

func (usgs *updateSQLGeneratorSuite) TestGenerate_noSetValues() {
	uc := exp.NewUpdateClauses().SetTable(exp.NewIdentifierExpression("", "test", ""))

	expectedErr := "goqu: no set values found when generating update sql"
	usgs.assertCases(
		sqlgen.NewUpdateSQLGenerator("test", "db", sqlgen.DefaultDialectOptions()),
		updateTestCase{clause: uc, err: expectedErr},
		updateTestCase{clause: uc, err: expectedErr, isPrepared: true},
	)
}

func (usgs *updateSQLGeneratorSuite) TestGenerate_withFrom() {
	uc := exp.NewUpdateClauses().
		SetTable(exp.NewIdentifierExpression("", "test", "")).
		SetSetValues(exp.Record{"foo": "bar"}).
		SetFrom(exp.NewColumnListExpression(nil, "db", "other_test"))

	ucNullSet := exp.NewUpdateClauses().
		SetTable(exp.NewIdentifierExpression("", "test", "")).
		SetSetValues(exp.Record{"foo": nil}).
		SetFrom(exp.NewColumnListExpression(nil, "db", "other_test"))

	opts := sqlgen.DefaultDialectOptions()
	usgs.assertCases(
		sqlgen.NewUpdateSQLGenerator("test", "db", opts),
		updateTestCase{clause: uc, sql: `UPDATE "test" SET "foo"='bar' FROM "other_test"`},
		updateTestCase{clause: uc, sql: `UPDATE "test" SET "foo"=? FROM "other_test"`, isPrepared: true, args: []interface{}{"bar"}},

		updateTestCase{clause: ucNullSet, sql: `UPDATE "test" SET "foo"=NULL FROM "other_test"`},
		updateTestCase{clause: ucNullSet, sql: `UPDATE "test" SET "foo"=? FROM "other_test"`, isPrepared: true, args: []interface{}{nil}},
	)

	opts = sqlgen.DefaultDialectOptions()
	opts.UseFromClauseForMultipleUpdateTables = false
	usgs.assertCases(
		sqlgen.NewUpdateSQLGenerator("test", "db", opts),
		updateTestCase{clause: uc, sql: `UPDATE "test","other_test" SET "foo"='bar'`},
		updateTestCase{clause: uc, sql: `UPDATE "test","other_test" SET "foo"=?`, isPrepared: true, args: []interface{}{"bar"}},

		updateTestCase{clause: ucNullSet, sql: `UPDATE "test","other_test" SET "foo"=NULL`},
		updateTestCase{clause: ucNullSet, sql: `UPDATE "test","other_test" SET "foo"=?`, isPrepared: true, args: []interface{}{nil}},
	)

	opts = sqlgen.DefaultDialectOptions()
	opts.SupportsMultipleUpdateTables = false
	expectedErr := "goqu: dialect does not support multiple tables in UPDATE [dialect=test]"
	usgs.assertCases(
		sqlgen.NewUpdateSQLGenerator("test", "db", opts),
		updateTestCase{clause: uc, err: expectedErr},
		updateTestCase{clause: uc, err: expectedErr, isPrepared: true},
	)
}

func (usgs *updateSQLGeneratorSuite) TestGenerate_withUpdateExpression() {
	opts := sqlgen.DefaultDialectOptions()
	// make sure the fragments are used
	opts.SetFragment = []byte(" set ")
	uc := exp.NewUpdateClauses().
		SetTable(exp.NewIdentifierExpression("", "test", ""))
	ucRecord := uc.SetSetValues(exp.Record{"a": "b", "b": "c"})
	ucRecordNullVal := uc.SetSetValues(exp.Record{"a": "b", "b": nil})
	ucRecordBoolVals := uc.SetSetValues(exp.Record{"a": true, "b": false})
	ucEmptyRecord := uc.SetSetValues(exp.Record{})

	usgs.assertCases(
		sqlgen.NewUpdateSQLGenerator("test", "db", opts),
		updateTestCase{clause: ucRecord, sql: `UPDATE "test" set "a"='b',"b"='c'`},
		updateTestCase{clause: ucRecord, sql: `UPDATE "test" set "a"=?,"b"=?`, isPrepared: true, args: []interface{}{"b", "c"}},

		updateTestCase{clause: ucRecordNullVal, sql: `UPDATE "test" set "a"='b',"b"=NULL`},
		updateTestCase{clause: ucRecordNullVal, sql: `UPDATE "test" set "a"=?,"b"=?`, isPrepared: true, args: []interface{}{"b", nil}},

		updateTestCase{clause: ucRecordBoolVals, sql: `UPDATE "test" set "a"=TRUE,"b"=FALSE`},
		updateTestCase{clause: ucRecordBoolVals, sql: `UPDATE "test" set "a"=?,"b"=?`, isPrepared: true, args: []interface{}{true, false}},

		updateTestCase{clause: ucEmptyRecord, err: "goqu: no update values provided"},
		updateTestCase{clause: ucEmptyRecord, err: "goqu: no update values provided", isPrepared: true},
	)
}

func (usgs *updateSQLGeneratorSuite) TestGenerate_withOrder() {
	uc := exp.NewUpdateClauses().
		SetTable(exp.NewIdentifierExpression("", "test", "")).
		SetSetValues(exp.Record{"a": "b", "b": "c"}).
		SetOrder(
			exp.NewIdentifierExpression("", "", "a").Asc(),
			exp.NewIdentifierExpression("", "", "b").Desc(),
		)

	opts := sqlgen.DefaultDialectOptions()
	opts.SupportsOrderByOnUpdate = true

	usgs.assertCases(
		sqlgen.NewUpdateSQLGenerator("test", "db", opts),
		updateTestCase{clause: uc, sql: `UPDATE "test" SET "a"='b',"b"='c' ORDER BY "a" ASC, "b" DESC`},
		updateTestCase{
			clause:     uc,
			sql:        `UPDATE "test" SET "a"=?,"b"=? ORDER BY "a" ASC, "b" DESC`,
			isPrepared: true,
			args:       []interface{}{"b", "c"},
		},
	)
}

func (usgs *updateSQLGeneratorSuite) TestGenerate_withOrderUnsupported() {
	opts := sqlgen.DefaultDialectOptions()
	opts.SupportsOrderByOnUpdate = false
	uc := exp.NewUpdateClauses().
		SetTable(exp.NewIdentifierExpression("", "test", "")).
		SetSetValues(exp.Record{"a": "b"})

	usgs.assertCases(
		sqlgen.NewUpdateSQLGenerator("test", "db", opts),
		updateTestCase{
			clause: uc.SetOrder(exp.NewIdentifierExpression("", "", "a").Asc()),
			err:    "goqu: dialect does not support ORDER BY on UPDATE [dialect=test]",
		},
		updateTestCase{
			clause:     uc.SetOrder(exp.NewIdentifierExpression("", "", "a").Asc()),
			err:        "goqu: dialect does not support ORDER BY on UPDATE [dialect=test]",
			isPrepared: true,
		},
		updateTestCase{clause: uc, sql: `UPDATE "test" SET "a"='b'`},
	)
}

func (usgs *updateSQLGeneratorSuite) TestGenerate_emptyAppends() {
	uc := exp.NewUpdateClauses().
		SetTable(exp.NewIdentifierExpression("", "test", "")).
		SetSetValues(exp.Record{"a": "b"})
	usgs.assertCases(
		sqlgen.NewUpdateSQLGenerator("test", "db", sqlgen.DefaultDialectOptions()),
		updateTestCase{clause: uc.WhereAppend(), err: "goqu: WhereAppend requires at least one expression"},
		updateTestCase{
			clause: uc.WhereAppend(exp.NewIdentifierExpression("", "", "a").Eq(1)).WhereAppend(),
			err:    "goqu: WhereAppend requires at least one expression",
		},
	)
	opts := sqlgen.DefaultDialectOptions()
	opts.SupportsOrderByOnUpdate = true
	a := exp.NewIdentifierExpression("", "", "a")
	usgs.assertCases(
		sqlgen.NewUpdateSQLGenerator("test", "db", opts),
		updateTestCase{clause: uc.OrderAppend(), err: "goqu: OrderAppend requires at least one expression"},
		updateTestCase{clause: uc.OrderPrepend(), err: "goqu: OrderPrepend requires at least one expression"},
		updateTestCase{clause: uc.SetOrder(a.Asc()).OrderAppend(), err: "goqu: OrderAppend requires at least one expression"},
		updateTestCase{clause: uc.SetOrder(a.Asc()).OrderPrepend(), err: "goqu: OrderPrepend requires at least one expression"},
		updateTestCase{clause: uc.SetOrder().OrderAppend(a.Asc()), err: "goqu: Order requires at least one expression"},
		updateTestCase{clause: uc.SetOrder().OrderPrepend(a.Asc()), err: "goqu: Order requires at least one expression"},
		updateTestCase{clause: uc.SetOrder().OrderAppend(), err: "goqu: Order requires at least one expression"},
		updateTestCase{clause: uc.SetOrder().OrderPrepend(), err: "goqu: Order requires at least one expression"},
	)
}

func (usgs *updateSQLGeneratorSuite) TestGenerate_withLimitUnsupported() {
	opts := sqlgen.DefaultDialectOptions()
	uc := exp.NewUpdateClauses().
		SetTable(exp.NewIdentifierExpression("", "test", "")).
		SetSetValues(exp.Record{"a": "b"})

	usgs.assertCases(
		sqlgen.NewUpdateSQLGenerator("test", "db", opts),
		updateTestCase{clause: uc.SetLimit(10), err: "goqu: dialect does not support LIMIT on UPDATE [dialect=test]"},
		updateTestCase{
			clause:     uc.SetLimit(10),
			err:        "goqu: dialect does not support LIMIT on UPDATE [dialect=test]",
			isPrepared: true,
		},
		updateTestCase{clause: uc.SetLimit(uint(0)), err: "goqu: dialect does not support LIMIT on UPDATE [dialect=test]"},
		updateTestCase{clause: uc.SetLimit(exp.NewLiteralExpression("ALL")), sql: `UPDATE "test" SET "a"='b'`},
		updateTestCase{clause: uc, sql: `UPDATE "test" SET "a"='b'`},
	)
}

func (usgs *updateSQLGeneratorSuite) TestGenerate_withLimit() {
	uc := exp.NewUpdateClauses().
		SetTable(exp.NewIdentifierExpression("", "test", "")).
		SetSetValues(exp.Record{"a": "b", "b": "c"}).
		SetLimit(10)

	opts := sqlgen.DefaultDialectOptions()
	opts.SupportsLimitOnUpdate = true

	usgs.assertCases(
		sqlgen.NewUpdateSQLGenerator("test", "db", opts),
		updateTestCase{clause: uc, sql: `UPDATE "test" SET "a"='b',"b"='c' LIMIT 10`},
		updateTestCase{clause: uc, sql: `UPDATE "test" SET "a"=?,"b"=? LIMIT ?`, isPrepared: true, args: []interface{}{"b", "c", int64(10)}},
	)
}

func (usgs *updateSQLGeneratorSuite) TestGenerate_withCommonTables() {
	tse := newTestAppendableExpression("select * from foo", emptyArgs, nil, nil)
	uc := exp.NewUpdateClauses().
		SetTable(exp.NewIdentifierExpression("", "test_cte", "")).
		SetSetValues(exp.Record{"a": "b", "b": "c"})
	ucCte1 := uc.CommonTablesAppend(exp.NewCommonTableExpression(false, "test_cte", tse))
	ucCte2 := uc.CommonTablesAppend(exp.NewCommonTableExpression(true, "test_cte", tse))

	usgs.assertCases(
		sqlgen.NewUpdateSQLGenerator("test", "db", sqlgen.DefaultDialectOptions()),
		updateTestCase{
			clause: ucCte1,
			sql:    `WITH test_cte AS (select * from foo) UPDATE "test_cte" SET "a"='b',"b"='c'`,
		},
		updateTestCase{
			clause:     ucCte1,
			sql:        `WITH test_cte AS (select * from foo) UPDATE "test_cte" SET "a"=?,"b"=?`,
			isPrepared: true,
			args:       []interface{}{"b", "c"},
		},

		updateTestCase{
			clause: ucCte2,
			sql:    `WITH RECURSIVE test_cte AS (select * from foo) UPDATE "test_cte" SET "a"='b',"b"='c'`,
		},
		updateTestCase{
			clause:     ucCte2,
			sql:        `WITH RECURSIVE test_cte AS (select * from foo) UPDATE "test_cte" SET "a"=?,"b"=?`,
			isPrepared: true,
			args:       []interface{}{"b", "c"},
		},
	)
}

func TestUpdateSQLGenerator(t *testing.T) {
	suite.Run(t, new(updateSQLGeneratorSuite))
}
