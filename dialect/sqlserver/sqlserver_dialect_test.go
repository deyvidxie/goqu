package sqlserver_test

import (
	"testing"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/sqlgen"
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
		sqlTestCase{ds: ds.Where(col.BitwiseLeftShift(1)), err: "goqu: bitwise operator 'Left Shift' not supported"},
		sqlTestCase{ds: ds.Where(col.BitwiseRightShift(1)), err: "goqu: bitwise operator 'Right Shift' not supported"},
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
			ds:  ds.Select("a").Distinct("a").Limit(1),
			err: "goqu: dialect does not support DISTINCT ON clause [dialect=sqlserver]",
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
		"do update excluded":   goqu.DoUpdateExcluded("", "a"),
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := goqu.Dialect("sqlserver").Insert("test").Rows(goqu.Record{"a": "a"}).OnConflict(conflict).ToSQL()
			assert.EqualError(t, err, "goqu: dialect does not support upsert [dialect=sqlserver]")
			assert.True(t, sqlgen.IsUnsupported(err))
		})
	}
}
