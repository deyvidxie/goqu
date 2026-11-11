package sqlserver_test

import (
	"regexp"
	"testing"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type (
	sqlserverDialectSuite struct {
		suite.Suite
	}
	sqlTestCase struct {
		ds         exp.SQLExpression
		sql        string
		err        string
		isPrepared bool
		args       []interface{}
	}
)

func (sds *sqlserverDialectSuite) GetDs(table string) *goqu.SelectDataset {
	return goqu.Dialect("sqlserver").From(table)
}

func (sds *sqlserverDialectSuite) assertSQL(cases ...sqlTestCase) {
	for i, c := range cases {
		actualSQL, actualArgs, err := c.ds.ToSQL()
		if c.err == "" {
			sds.NoError(err, "test case %d failed", i)
		} else {
			sds.EqualError(err, c.err, "test case %d failed", i)
		}
		sds.Equal(c.sql, actualSQL, "test case %d failed", i)
		if c.isPrepared && c.args != nil || len(c.args) > 0 {
			sds.Equal(c.args, actualArgs, "test case %d failed", i)
		} else {
			sds.Empty(actualArgs, "test case %d failed", i)
		}
	}
}

func (sds *sqlserverDialectSuite) TestTimeFunctions() {
	sds.assertSQL(
		sqlTestCase{
			ds:  sds.GetDs("t").Select(goqu.Now(3), goqu.UTCTimestamp()),
			sql: `SELECT CAST(SYSDATETIME() AS DATETIME2(3)), SYSUTCDATETIME() FROM "t"`,
		},
		sqlTestCase{
			ds:         sds.GetDs("t").Prepared(true).Select(goqu.Now(3), goqu.UTCTimestamp()),
			sql:        `SELECT CAST(SYSDATETIME() AS DATETIME2(3)), SYSUTCDATETIME() FROM "t"`,
			isPrepared: true,
			args:       []interface{}{},
		},
	)
}

func (sds *sqlserverDialectSuite) TestLiteralString() {
	ds := sds.GetDs("test")
	col := goqu.C("a")
	sds.assertSQL(
		sqlTestCase{ds: ds.Where(col.Eq("test")), sql: `SELECT * FROM "test" WHERE ("a" = 'test')`},
		sqlTestCase{ds: ds.Where(col.Eq("test'test")), sql: `SELECT * FROM "test" WHERE ("a" = 'test''test')`},
		sqlTestCase{ds: ds.Where(col.Eq("') OR 1=1 --")), sql: `SELECT * FROM "test" WHERE ("a" = ''') OR 1=1 --')`},
		sqlTestCase{ds: ds.Where(col.Eq(`test"test`)), sql: `SELECT * FROM "test" WHERE ("a" = 'test"test')`},
		sqlTestCase{ds: ds.Where(col.Eq(`test\test`)), sql: `SELECT * FROM "test" WHERE ("a" = 'test\test')`},
		sqlTestCase{ds: ds.Where(col.Eq("test\ntest")), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test\ntest')"},
		sqlTestCase{ds: ds.Where(col.Eq("test\rtest")), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test\rtest')"},
		sqlTestCase{ds: ds.Where(col.Eq("test\x00test")), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test\x00test')"},
		sqlTestCase{ds: ds.Where(col.Eq("test\x1atest")), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test\x1atest')"},
	)
}

func (sds *sqlserverDialectSuite) TestLiteralBytes() {
	ds := sds.GetDs("test")
	col := goqu.C("a")
	sds.assertSQL(
		sqlTestCase{ds: ds.Where(col.Eq([]byte("test"))), sql: `SELECT * FROM "test" WHERE ("a" = 'test')`},
		sqlTestCase{ds: ds.Where(col.Eq([]byte("test'test"))), sql: `SELECT * FROM "test" WHERE ("a" = 'test''test')`},
		sqlTestCase{ds: ds.Where(col.Eq([]byte(`test"test`))), sql: `SELECT * FROM "test" WHERE ("a" = 'test"test')`},
		sqlTestCase{ds: ds.Where(col.Eq([]byte(`test\test`))), sql: `SELECT * FROM "test" WHERE ("a" = 'test\test')`},
		sqlTestCase{ds: ds.Where(col.Eq([]byte("test\ntest"))), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test\ntest')"},
		sqlTestCase{ds: ds.Where(col.Eq([]byte("test\rtest"))), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test\rtest')"},
		sqlTestCase{ds: ds.Where(col.Eq([]byte("test\x00test"))), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test\x00test')"},
		sqlTestCase{ds: ds.Where(col.Eq([]byte("test\x1atest"))), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test\x1atest')"},
	)
}

func (sds *sqlserverDialectSuite) TestBitwiseOperations() {
	col := goqu.C("a")
	ds := sds.GetDs("test")
	sds.assertSQL(
		sqlTestCase{ds: ds.Where(col.BitwiseInversion()), sql: "SELECT * FROM \"test\" WHERE (~ \"a\")"},
		sqlTestCase{ds: ds.Where(col.BitwiseAnd(1)), sql: "SELECT * FROM \"test\" WHERE (\"a\" & 1)"},
		sqlTestCase{ds: ds.Where(col.BitwiseOr(1)), sql: "SELECT * FROM \"test\" WHERE (\"a\" | 1)"},
		sqlTestCase{ds: ds.Where(col.BitwiseXor(1)), sql: "SELECT * FROM \"test\" WHERE (\"a\" ^ 1)"},
		sqlTestCase{
			ds:  ds.Where(col.BitwiseLeftShift(1)),
			err: "goqu: dialect does not support bitwise operator \"Left Shift\" [dialect=sqlserver]",
		},
		sqlTestCase{
			ds:  ds.Where(col.BitwiseRightShift(1)),
			err: "goqu: dialect does not support bitwise operator \"Right Shift\" [dialect=sqlserver]",
		},
	)
}

func (sds *sqlserverDialectSuite) TestFuns() {
	col := goqu.C("a")
	ds := sds.GetDs("test")
	sds.assertSQL(
		sqlTestCase{ds: ds.Select(goqu.GREATEST(col, 2)), sql: "SELECT GREATEST(\"a\", 2) FROM \"test\""},
		sqlTestCase{ds: ds.Select(goqu.LEAST(col, 2)), sql: "SELECT LEAST(\"a\", 2) FROM \"test\""},
		sqlTestCase{ds: ds.Select(goqu.UPPER(col)), sql: "SELECT UPPER(\"a\") FROM \"test\""},
		sqlTestCase{ds: ds.Select(goqu.UPPER("Val")), sql: "SELECT UPPER('Val') FROM \"test\""},
		sqlTestCase{ds: ds.Select(goqu.LOWER(col)), sql: "SELECT LOWER(\"a\") FROM \"test\""},
		sqlTestCase{ds: ds.Select(goqu.LOWER("Val")), sql: "SELECT LOWER('Val') FROM \"test\""},
	)
}

func (sds *sqlserverDialectSuite) TestTimeLiteral() {
	ds := goqu.Dialect("sqlserver").From("test")
	sds.assertSQL(
		sqlTestCase{
			ds:  ds.Where(goqu.C("t").Eq(time.Date(2024, 1, 2, 3, 4, 5, 123456789, time.UTC))),
			sql: `SELECT * FROM "test" WHERE ("t" = '2024-01-02 03:04:05.1234567')`,
		},
		sqlTestCase{
			ds:  ds.Where(goqu.C("t").Eq(time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC))),
			sql: `SELECT * FROM "test" WHERE ("t" = '2024-01-02 03:04:05')`,
		},
	)
}

func (sds *sqlserverDialectSuite) TestSelectDistinctWithLimit() {
	ds := sds.GetDs("test")
	sds.assertSQL(
		// DISTINCT has to come before TOP.
		sqlTestCase{ds: ds.Select("a").Distinct().Limit(1), sql: `SELECT DISTINCT  TOP (1) "a" FROM "test"`},
		sqlTestCase{ds: ds.Distinct().Limit(5), sql: `SELECT DISTINCT  TOP (5) * FROM "test"`},
		sqlTestCase{
			ds:         ds.Select("a").Distinct().Limit(1).Prepared(true),
			sql:        `SELECT DISTINCT  TOP (@p1) "a" FROM "test"`,
			isPrepared: true,
			args:       []interface{}{int64(1)},
		},
		// Without a limit, or with an offset (OFFSET/FETCH instead of TOP), nothing changes.
		sqlTestCase{ds: ds.Select("a").Distinct(), sql: `SELECT DISTINCT "a" FROM "test"`},
		sqlTestCase{
			ds:  ds.Select("a").Distinct().Order(goqu.C("a").Asc()).Limit(1).Offset(2),
			sql: `SELECT DISTINCT "a" FROM "test" ORDER BY "a" ASC OFFSET 2 ROWS FETCH FIRST 1 ROWS ONLY`,
		},
		// DISTINCT ON is still not supported.
		sqlTestCase{
			ds:  ds.Select("a").DistinctOn("a").Limit(1),
			err: "goqu: dialect does not support DISTINCT ON clause [dialect=sqlserver]",
		},
	)
}

func (sds *sqlserverDialectSuite) TestLimitZero() {
	d := goqu.Dialect("sqlserver")
	sel := d.From("test")
	upd := d.Update("test").Set(goqu.Record{"a": 1})
	del := d.Delete("test").Where(goqu.C("a").Eq(1))
	const zeroFetch = "goqu: dialect does not support LIMIT 0 with an OFFSET [dialect=sqlserver]"
	sds.assertSQL(
		sqlTestCase{ds: sel.Limit(0), sql: `SELECT  TOP (0) * FROM "test"`},
		sqlTestCase{ds: sel.Prepared(true).Limit(0), sql: `SELECT  TOP (@p1) * FROM "test"`, isPrepared: true, args: []interface{}{int64(0)}},
		sqlTestCase{ds: upd.Limit(0), sql: `UPDATE TOP (0) "test" SET "a"=1`},
		sqlTestCase{
			ds:         upd.Prepared(true).Limit(0),
			sql:        `UPDATE TOP (@p1) "test" SET "a"=@p2`,
			isPrepared: true,
			args:       []interface{}{int64(0), int64(1)},
		},
		sqlTestCase{ds: del.Limit(0), sql: `DELETE TOP (0) FROM "test" WHERE ("a" = 1)`},
		sqlTestCase{
			ds:         del.Prepared(true).Limit(0),
			sql:        `DELETE TOP (@p1) FROM "test" WHERE ("a" = @p2)`,
			isPrepared: true,
			args:       []interface{}{int64(0), int64(1)},
		},
		sqlTestCase{ds: sel.Order(goqu.C("a").Asc()).Limit(0).Offset(20), err: zeroFetch},
		sqlTestCase{ds: sel.Prepared(true).Order(goqu.C("a").Asc()).Limit(0).Offset(20), err: zeroFetch, isPrepared: true},
	)
	for _, prepared := range []bool{false, true} {
		_, _, err := sel.Prepared(prepared).Order(goqu.C("a").Asc()).Limit(0).Offset(20).ToSQL()
		sds.True(goqu.IsUnsupported(err), "prepared=%v: %v", prepared, err)
	}
}

func (sds *sqlserverDialectSuite) TestOffsetRequiresOrder() {
	ds := goqu.Dialect("sqlserver").From("test")
	for _, offset := range []*goqu.SelectDataset{ds.Offset(20), ds.Limit(10).Offset(20), ds.Order(goqu.C("a").Asc()).ClearOrder().Offset(20)} {
		_, _, err := offset.ToSQL()
		sds.EqualError(err, "goqu: dialect does not support OFFSET without ORDER BY [dialect=sqlserver]")
		sds.True(goqu.IsUnsupported(err))
	}
	sds.assertSQL(
		sqlTestCase{ds: ds.Limit(10).Offset(0), sql: `SELECT  TOP (10) * FROM "test"`},
		sqlTestCase{
			ds:  ds.Order(goqu.C("a").Asc()).Limit(10).Offset(20),
			sql: `SELECT * FROM "test" ORDER BY "a" ASC OFFSET 20 ROWS FETCH FIRST 10 ROWS ONLY`,
		},
	)
}

func (sds *sqlserverDialectSuite) TestTruncate() {
	ds := goqu.Dialect("sqlserver").From("test")
	sds.assertSQL(
		sqlTestCase{ds: ds.Truncate(), sql: "TRUNCATE TABLE \"test\""},
		sqlTestCase{
			ds:  ds.Truncate().Identity("restart"),
			err: "goqu: dialect does not support RESTART IDENTITY in TRUNCATE [dialect=sqlserver]",
		},
		sqlTestCase{
			ds:  goqu.Dialect("sqlserver").Truncate("a", "b"),
			err: "goqu: dialect does not support multiple tables in TRUNCATE [dialect=sqlserver]",
		},
		sqlTestCase{ds: ds.Truncate().Cascade(), err: "goqu: dialect does not support CASCADE in TRUNCATE [dialect=sqlserver]"},
		sqlTestCase{ds: ds.Truncate().Restrict(), err: "goqu: dialect does not support RESTRICT in TRUNCATE [dialect=sqlserver]"},
	)
}

func (sds *sqlserverDialectSuite) TestInsertRowAlias() {
	sds.assertSQL(
		sqlTestCase{
			ds:  goqu.Dialect("sqlserver").Insert("test").Rows(goqu.Record{"a": 1}).As("new"),
			err: "goqu: dialect does not support an INSERT row alias [dialect=sqlserver]",
		},
	)
}

func (sds *sqlserverDialectSuite) TestSelectLimitAll() {
	ds := goqu.Dialect("sqlserver").From("test")
	sds.assertSQL(
		sqlTestCase{ds: ds.LimitAll(), sql: `SELECT * FROM "test"`},
		sqlTestCase{
			ds:  ds.Order(goqu.C("a").Asc()).Offset(2).LimitAll(),
			sql: `SELECT * FROM "test" ORDER BY "a" ASC OFFSET 2 ROWS`,
		},
	)
}

func (sds *sqlserverDialectSuite) TestDeleteWithLimit() {
	ds := goqu.Dialect("sqlserver").Delete("test")
	sds.assertSQL(
		sqlTestCase{
			ds:  ds.Where(goqu.C("a").Gt(1)).Limit(10),
			sql: `DELETE TOP (10) FROM "test" WHERE ("a" > 1)`,
		},
		sqlTestCase{
			ds:  ds.Limit(10),
			sql: `DELETE TOP (10) FROM "test"`,
		},
		sqlTestCase{
			ds:  ds.LimitAll(),
			sql: `DELETE FROM "test"`,
		},
		// Without a limit the statement is unchanged.
		sqlTestCase{
			ds:  ds.Where(goqu.C("a").Gt(1)),
			sql: `DELETE FROM "test" WHERE ("a" > 1)`,
		},
		sqlTestCase{
			ds:         ds.Where(goqu.C("a").Gt(1)).Limit(10).Prepared(true),
			sql:        `DELETE TOP (@p1) FROM "test" WHERE ("a" > @p2)`,
			isPrepared: true,
			args:       []interface{}{int64(10), int64(1)},
		},
	)
}

func (sds *sqlserverDialectSuite) TestDeleteWithOrderIsAnError() {
	ds := goqu.Dialect("sqlserver").Delete("test")
	sds.assertSQL(
		sqlTestCase{
			ds:  ds.Order(goqu.C("a").Asc()),
			err: "goqu: dialect does not support ORDER BY on DELETE [dialect=sqlserver]",
		},
		sqlTestCase{
			ds:  ds.Order(goqu.C("a").Asc()).Limit(10),
			err: "goqu: dialect does not support ORDER BY on DELETE [dialect=sqlserver]",
		},
	)
}

func (sds *sqlserverDialectSuite) TestUpdateWithLimit() {
	ds := goqu.Dialect("sqlserver").Update("test").Set(goqu.Record{"a": 2})
	sds.assertSQL(
		sqlTestCase{
			ds:  ds.Where(goqu.C("a").Gt(1)).Limit(1),
			sql: `UPDATE TOP (1) "test" SET "a"=2 WHERE ("a" > 1)`,
		},
		sqlTestCase{
			ds:  ds.Where(goqu.C("a").Gt(1)),
			sql: `UPDATE "test" SET "a"=2 WHERE ("a" > 1)`,
		},
		sqlTestCase{
			ds:  ds.Where(goqu.C("a").Gt(1)).LimitAll(),
			sql: `UPDATE "test" SET "a"=2 WHERE ("a" > 1)`,
		},
		sqlTestCase{
			ds:  ds.From("other").Where(goqu.C("a").Gt(1)).Limit(1),
			sql: `UPDATE TOP (1) "test" SET "a"=2 FROM "other" WHERE ("a" > 1)`,
		},
		sqlTestCase{
			ds:         ds.Where(goqu.C("a").Gt(1)).Limit(1).Prepared(true),
			sql:        `UPDATE TOP (@p1) "test" SET "a"=@p2 WHERE ("a" > @p3)`,
			isPrepared: true,
			args:       []interface{}{int64(1), int64(2), int64(1)},
		},
	)
}

func (sds *sqlserverDialectSuite) TestUpdateWithOrderIsAnError() {
	ds := goqu.Dialect("sqlserver").Update("test").Set(goqu.Record{"a": 2})
	sds.assertSQL(
		sqlTestCase{
			ds:  ds.Order(goqu.C("a").Asc()),
			err: "goqu: dialect does not support ORDER BY on UPDATE [dialect=sqlserver]",
		},
		sqlTestCase{
			ds:  ds.Order(goqu.C("a").Asc()).Limit(1),
			err: "goqu: dialect does not support ORDER BY on UPDATE [dialect=sqlserver]",
		},
	)
}

func (sds *sqlserverDialectSuite) TestLockClausesAreErrors() {
	ds := goqu.Dialect("sqlserver").From("test")
	sds.assertSQL(
		sqlTestCase{ds: ds.ForUpdate(goqu.Wait), err: "goqu: dialect does not support FOR UPDATE [dialect=sqlserver]"},
		sqlTestCase{ds: ds.ForUpdate(goqu.SkipLocked), err: "goqu: dialect does not support FOR UPDATE [dialect=sqlserver]"},
		sqlTestCase{ds: ds.ForNoKeyUpdate(goqu.Wait), err: "goqu: dialect does not support FOR NO KEY UPDATE [dialect=sqlserver]"},
		sqlTestCase{ds: ds.ForShare(goqu.NoWait), err: "goqu: dialect does not support FOR SHARE [dialect=sqlserver]"},
		sqlTestCase{ds: ds.ForKeyShare(goqu.Wait), err: "goqu: dialect does not support FOR KEY SHARE [dialect=sqlserver]"},
	)
}

func TestDatasetAdapterSuite(t *testing.T) {
	suite.Run(t, new(sqlserverDialectSuite))
}

func TestInsertOnConflictNotSupported(t *testing.T) {
	for name, conflict := range map[string]exp.ConflictExpression{
		"do nothing":           goqu.DoNothing(),
		"do nothing with cols": goqu.DoNothing().SetCols(exp.NewColumnListExpression(nil, "db", "a")),
		"do update":            goqu.DoUpdate("", goqu.Record{"a": "b"}),
		"do update where":      goqu.DoUpdate("", goqu.Record{"a": "b"}).Where(goqu.C("a").Eq(1)),
		"do update nil":        goqu.DoUpdate("", nil),
		"do update excluded":   goqu.DoUpdateExcluded("", "a"),
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := goqu.Dialect("sqlserver").Insert("test").Rows(goqu.Record{"a": "a"}).OnConflict(conflict).ToSQL()
			assert.EqualError(t, err, "goqu: dialect does not support upsert [dialect=sqlserver]")
		})
	}
}

func TestEmptyInList(t *testing.T) {
	ds := goqu.Dialect("sqlserver").From("test").Where(goqu.C("a").In([]int{}), goqu.C("b").NotIn([]int{}))
	for _, prepared := range []bool{false, true} {
		sql, args, err := ds.Prepared(prepared).ToSQL()
		assert.NoError(t, err)
		assert.Equal(t, "SELECT * FROM \"test\" WHERE ((1=0) AND (1=1))", sql)
		assert.Empty(t, args)
	}
}

func TestJSONFunctionsAreErrors(t *testing.T) {
	_, _, err := goqu.Dialect("sqlserver").From("t").Where(goqu.JSONValue("doc", "$.a").Eq(1)).ToSQL()
	assert.EqualError(t, err, "goqu: dialect does not support JSONValue [dialect=sqlserver]")
	assert.True(t, goqu.IsUnsupported(err))
}

func TestIntervalIsError(t *testing.T) {
	_, _, err := goqu.Dialect("sqlserver").From("t").Where(goqu.C("at").Lt(goqu.Now().Sub(goqu.Interval(1, "day")))).ToSQL()
	assert.EqualError(t, err, `goqu: dialect does not support Interval [dialect=sqlserver]`)
	assert.True(t, goqu.IsUnsupported(err))
}

func TestLateralIsError(t *testing.T) {
	lateral := goqu.Lateral(goqu.From("other").Select("c").As("o"))
	for _, ds := range []*goqu.SelectDataset{
		goqu.Dialect("sqlserver").From(goqu.T("test"), lateral),
		goqu.Dialect("sqlserver").From("test").Join(lateral, goqu.On(goqu.V(true))),
	} {
		_, _, err := ds.ToSQL()
		assert.EqualError(t, err, "goqu: dialect does not support lateral expressions [dialect=sqlserver]")
		assert.True(t, goqu.IsUnsupported(err))
	}
}

func TestRegexpIsError(t *testing.T) {
	re := regexp.MustCompile("[ab]")
	a := goqu.C("a")
	for op, condition := range map[string]exp.BooleanExpression{
		"regexplike": a.Like(re), "regexpnotlike": a.NotLike(re), "regexpilike": a.ILike(re), "regexpnotilike": a.NotILike(re),
	} {
		_, _, err := goqu.Dialect("sqlserver").From("t").Where(condition).ToSQL()
		assert.EqualError(t, err, "goqu: dialect does not support boolean operator \""+op+"\" [dialect=sqlserver]")
		assert.True(t, goqu.IsUnsupported(err))
	}
}

func TestInsertWithoutRows(t *testing.T) {
	sql, _, err := goqu.Dialect("sqlserver").Insert("test").ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, `INSERT INTO "test" DEFAULT VALUES`, sql)
}
