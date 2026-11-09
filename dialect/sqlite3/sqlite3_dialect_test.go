package sqlite3_test

import (
	"regexp"
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type (
	sqlite3DialectSuite struct {
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

func (sds *sqlite3DialectSuite) GetDs(table string) *goqu.SelectDataset {
	return goqu.Dialect("sqlite3").From(table)
}

func (sds *sqlite3DialectSuite) assertSQL(cases ...sqlTestCase) {
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

func (sds *sqlite3DialectSuite) TestTimeFunctions() {
	sds.assertSQL(
		sqlTestCase{
			ds:  sds.GetDs("t").Select(goqu.Now(3), goqu.UTCTimestamp()),
			sql: "SELECT strftime('%Y-%m-%d %H:%M:%f', 'now', 'localtime'), datetime('now') FROM `t`",
		},
		sqlTestCase{
			ds:  sds.GetDs("t").Select(goqu.Now(), goqu.UTCTimestamp(0)),
			sql: "SELECT datetime('now', 'localtime'), datetime('now') FROM `t`",
		},
		sqlTestCase{
			ds:         sds.GetDs("t").Prepared(true).Select(goqu.Now(3), goqu.UTCTimestamp()),
			sql:        "SELECT strftime('%Y-%m-%d %H:%M:%f', 'now', 'localtime'), datetime('now') FROM `t`",
			isPrepared: true,
			args:       []interface{}{},
		},
	)
}

func (sds *sqlite3DialectSuite) TestIdentifiers() {
	ds := sds.GetDs("test")
	sds.assertSQL(
		sqlTestCase{ds: ds.Select(
			"a",
			goqu.I("a.b.c"),
			goqu.I("c.d"),
			goqu.C("test").As("test"),
		), sql: "SELECT `a`, `a`.`b`.`c`, `c`.`d`, `test` AS `test` FROM `test`"},
	)
}

func (sds *sqlite3DialectSuite) TestUpdateSQL_multipleTables() {
	ds := sds.GetDs("test").Update().
		Set(goqu.Record{"foo": "bar"}).
		From("test_2").
		Where(goqu.I("test.id").Eq(goqu.I("test_2.test_id")))
	subquery := sds.GetDs("test_2").Select("test_id").Where(goqu.C("foo").Eq("baz")).As("other")
	dsWithSubquery := sds.GetDs("test").Update().
		Set(goqu.Record{"foo": "bar"}).
		From(subquery).
		Where(goqu.I("test.id").Eq(goqu.I("other.test_id")), goqu.I("test.id").Gt(10))
	sds.assertSQL(
		sqlTestCase{
			ds:  ds,
			sql: "UPDATE `test` SET `foo`='bar' FROM `test_2` WHERE (`test`.`id` = `test_2`.`test_id`)",
		},
		sqlTestCase{
			ds:         ds.Prepared(true),
			sql:        "UPDATE `test` SET `foo`=? FROM `test_2` WHERE (`test`.`id` = `test_2`.`test_id`)",
			isPrepared: true,
			args:       []interface{}{"bar"},
		},
		sqlTestCase{
			ds: dsWithSubquery,
			sql: "UPDATE `test` SET `foo`='bar' FROM " +
				"(SELECT `test_id` FROM `test_2` WHERE (`foo` = 'baz')) AS `other` " +
				"WHERE ((`test`.`id` = `other`.`test_id`) AND (`test`.`id` > 10))",
		},
		sqlTestCase{
			ds: dsWithSubquery.Prepared(true),
			sql: "UPDATE `test` SET `foo`=? FROM " +
				"(SELECT `test_id` FROM `test_2` WHERE (`foo` = ?)) AS `other` " +
				"WHERE ((`test`.`id` = `other`.`test_id`) AND (`test`.`id` > ?))",
			isPrepared: true,
			args:       []interface{}{"bar", "baz", int64(10)},
		},
	)
}

func (sds *sqlite3DialectSuite) TestTruncate() {
	ds := goqu.Dialect("sqlite3").From("test")
	sds.assertSQL(
		sqlTestCase{ds: ds.Truncate(), sql: "DELETE FROM `test`"},
		sqlTestCase{ds: ds.Truncate().Identity("restart"), err: "goqu: dialect does not support RESTART IDENTITY in TRUNCATE [dialect=sqlite3]"},
		sqlTestCase{
			ds:  goqu.Dialect("sqlite3").Truncate("a", "b"),
			err: "goqu: dialect does not support TRUNCATE of more than one table [dialect=sqlite3]",
		},
		sqlTestCase{ds: ds.Truncate().Cascade(), err: "goqu: dialect does not support CASCADE in TRUNCATE [dialect=sqlite3]"},
		sqlTestCase{ds: ds.Truncate().Restrict(), err: "goqu: dialect does not support RESTRICT in TRUNCATE [dialect=sqlite3]"},
	)
}

func (sds *sqlite3DialectSuite) TestLimitAllAndOffset() {
	ds := goqu.Dialect("sqlite3").From("test")
	sds.assertSQL(
		sqlTestCase{ds: ds.LimitAll(), sql: "SELECT * FROM `test`"},
		sqlTestCase{ds: ds.Offset(2), sql: "SELECT * FROM `test` LIMIT -1 OFFSET 2"},
		sqlTestCase{ds: ds.LimitAll().Offset(2), sql: "SELECT * FROM `test` LIMIT -1 OFFSET 2"},
		sqlTestCase{ds: ds.Limit(3).Offset(2), sql: "SELECT * FROM `test` LIMIT 3 OFFSET 2"},
	)
}

func (sds *sqlite3DialectSuite) TestInsertRowAlias() {
	sds.assertSQL(
		sqlTestCase{
			ds:  goqu.Dialect("sqlite3").Insert("test").Rows(goqu.Record{"a": 1}).As("new"),
			err: "goqu: dialect does not support an INSERT row alias [dialect=sqlite3]",
		},
	)
}

func (sds *sqlite3DialectSuite) TestCompoundExpressions() {
	ds1 := sds.GetDs("test").Select("a")
	ds2 := sds.GetDs("test2").Select("b")
	sds.assertSQL(
		sqlTestCase{ds: ds1.Union(ds2), sql: "SELECT `a` FROM `test` UNION SELECT `b` FROM `test2`"},
		sqlTestCase{ds: ds1.UnionAll(ds2), sql: "SELECT `a` FROM `test` UNION ALL SELECT `b` FROM `test2`"},
		sqlTestCase{ds: ds1.Intersect(ds2), sql: "SELECT `a` FROM `test` INTERSECT SELECT `b` FROM `test2`"},
	)
}

func (sds *sqlite3DialectSuite) TestLiteralString() {
	ds := sds.GetDs("test")
	sds.assertSQL(
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq("test")), sql: "SELECT * FROM `test` WHERE (`a` = 'test')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq("test'test")), sql: "SELECT * FROM `test` WHERE (`a` = 'test''test')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq(`test"test`)), sql: "SELECT * FROM `test` WHERE (`a` = 'test\"test')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq(`test\test`)), sql: "SELECT * FROM `test` WHERE (`a` = 'test\\test')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq("test\ntest")), sql: "SELECT * FROM `test` WHERE (`a` = 'test\ntest')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq("test\rtest")), sql: "SELECT * FROM `test` WHERE (`a` = 'test\rtest')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq("test\x00test")), sql: "SELECT * FROM `test` WHERE (`a` = 'test\x00test')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq("test\x1atest")), sql: "SELECT * FROM `test` WHERE (`a` = 'test\x1atest')"},
	)
}

func (sds *sqlite3DialectSuite) TestLiteralBytes() {
	ds := sds.GetDs("test")
	sds.assertSQL(
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq([]byte("test"))), sql: "SELECT * FROM `test` WHERE (`a` = 'test')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq([]byte("test'test"))), sql: "SELECT * FROM `test` WHERE (`a` = 'test''test')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq([]byte(`test"test`))), sql: "SELECT * FROM `test` WHERE (`a` = 'test\"test')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq([]byte(`test\test`))), sql: "SELECT * FROM `test` WHERE (`a` = 'test\\test')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq([]byte("test\ntest"))), sql: "SELECT * FROM `test` WHERE (`a` = 'test\ntest')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq([]byte("test\rtest"))), sql: "SELECT * FROM `test` WHERE (`a` = 'test\rtest')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq([]byte("test\x00test"))), sql: "SELECT * FROM `test` WHERE (`a` = 'test\x00test')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq([]byte("test\x1atest"))), sql: "SELECT * FROM `test` WHERE (`a` = 'test\x1atest')"},
	)
}

func (sds *sqlite3DialectSuite) TestBooleanOperations() {
	ds := sds.GetDs("test")
	sds.assertSQL(
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq(true)), sql: "SELECT * FROM `test` WHERE (`a` IS 1)"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq(false)), sql: "SELECT * FROM `test` WHERE (`a` IS 0)"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Is(true)), sql: "SELECT * FROM `test` WHERE (`a` IS 1)"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Is(false)), sql: "SELECT * FROM `test` WHERE (`a` IS 0)"},
		sqlTestCase{ds: ds.Where(goqu.C("a").IsTrue()), sql: "SELECT * FROM `test` WHERE (`a` IS 1)"},
		sqlTestCase{ds: ds.Where(goqu.C("a").IsFalse()), sql: "SELECT * FROM `test` WHERE (`a` IS 0)"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Neq(true)), sql: "SELECT * FROM `test` WHERE (`a` IS NOT 1)"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Neq(false)), sql: "SELECT * FROM `test` WHERE (`a` IS NOT 0)"},
		sqlTestCase{ds: ds.Where(goqu.C("a").IsNot(true)), sql: "SELECT * FROM `test` WHERE (`a` IS NOT 1)"},
		sqlTestCase{ds: ds.Where(goqu.C("a").IsNot(false)), sql: "SELECT * FROM `test` WHERE (`a` IS NOT 0)"},
		sqlTestCase{ds: ds.Where(goqu.C("a").IsNotTrue()), sql: "SELECT * FROM `test` WHERE (`a` IS NOT 1)"},
		sqlTestCase{ds: ds.Where(goqu.C("a").IsNotFalse()), sql: "SELECT * FROM `test` WHERE (`a` IS NOT 0)"},
		sqlTestCase{
			ds:  ds.Where(goqu.C("a").Like("a%")),
			err: "goqu: dialect does not support boolean operator \"like\", use ILike/NotILike [dialect=sqlite3]",
		},
		sqlTestCase{
			ds:  ds.Where(goqu.C("a").NotLike("a%")),
			err: "goqu: dialect does not support boolean operator \"notlike\", use ILike/NotILike [dialect=sqlite3]",
		},
		sqlTestCase{ds: ds.Where(goqu.C("a").ILike("a%")), sql: "SELECT * FROM `test` WHERE (`a` LIKE 'a%' ESCAPE '\\')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").NotILike("a%")), sql: "SELECT * FROM `test` WHERE (`a` NOT LIKE 'a%' ESCAPE '\\')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Like(regexp.MustCompile("[ab]"))), sql: "SELECT * FROM `test` WHERE (`a` REGEXP '[ab]')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").NotLike(regexp.MustCompile("[ab]"))), sql: "SELECT * FROM `test` WHERE (`a` NOT REGEXP '[ab]')"},
		sqlTestCase{
			ds: ds.Where(goqu.C("a").ILike(regexp.MustCompile("[ab]"))), sql: "SELECT * FROM `test` WHERE (`a` REGEXP '(?i)' || '[ab]')",
		},
		sqlTestCase{
			ds:  ds.Where(goqu.C("a").NotILike(regexp.MustCompile("[ab]"))),
			sql: "SELECT * FROM `test` WHERE (`a` NOT REGEXP '(?i)' || '[ab]')",
		},
		sqlTestCase{
			ds:         ds.Prepared(true).Where(goqu.C("a").ILike(regexp.MustCompile("[ab]"))),
			sql:        "SELECT * FROM `test` WHERE (`a` REGEXP '(?i)' || ?)",
			isPrepared: true,
			args:       []interface{}{"[ab]"},
		},
		sqlTestCase{
			ds:         ds.Prepared(true).Where(goqu.C("a").NotILike(regexp.MustCompile("[ab]"))),
			sql:        "SELECT * FROM `test` WHERE (`a` NOT REGEXP '(?i)' || ?)",
			isPrepared: true,
			args:       []interface{}{"[ab]"},
		},
	)
}

func (sds *sqlite3DialectSuite) TestBitwiseOperations() {
	col := goqu.C("a")
	ds := sds.GetDs("test")
	sds.assertSQL(
		sqlTestCase{ds: ds.Where(col.BitwiseInversion()), err: "goqu: dialect does not support bitwise operator \"Inversion\" [dialect=sqlite3]"},
		sqlTestCase{ds: ds.Where(col.BitwiseAnd(1)), sql: "SELECT * FROM `test` WHERE (`a` & 1)"},
		sqlTestCase{ds: ds.Where(col.BitwiseOr(1)), sql: "SELECT * FROM `test` WHERE (`a` | 1)"},
		sqlTestCase{ds: ds.Where(col.BitwiseXor(1)), err: "goqu: dialect does not support bitwise operator \"XOR\" [dialect=sqlite3]"},
		sqlTestCase{ds: ds.Where(col.BitwiseLeftShift(1)), sql: "SELECT * FROM `test` WHERE (`a` << 1)"},
		sqlTestCase{ds: ds.Where(col.BitwiseRightShift(1)), sql: "SELECT * FROM `test` WHERE (`a` >> 1)"},
	)
}

func (sds *sqlite3DialectSuite) TestFuns() {
	col := goqu.C("a")
	ds := sds.GetDs("test")
	sds.assertSQL(
		sqlTestCase{ds: ds.Select(goqu.GREATEST(col, 2)), sql: "SELECT MAX(`a`, 2) FROM `test`"},
		sqlTestCase{ds: ds.Select(goqu.LEAST(col, 2)), sql: "SELECT MIN(`a`, 2) FROM `test`"},
		sqlTestCase{ds: ds.Select(goqu.UPPER(col)), sql: "SELECT UPPER(`a`) FROM `test`"},
		sqlTestCase{ds: ds.Select(goqu.UPPER("Val")), sql: "SELECT UPPER('Val') FROM `test`"},
		sqlTestCase{ds: ds.Select(goqu.LOWER(col)), sql: "SELECT LOWER(`a`) FROM `test`"},
		sqlTestCase{ds: ds.Select(goqu.LOWER("Val")), sql: "SELECT LOWER('Val') FROM `test`"},
	)
}

func (sds *sqlite3DialectSuite) TestForUpdate() {
	ds := sds.GetDs("test").Where(goqu.C("a").Eq(1))
	sds.assertSQL(
		sqlTestCase{ds: ds.ForUpdate(goqu.Wait), sql: "SELECT * FROM `test` WHERE (`a` = 1)"},
		sqlTestCase{ds: ds.ForUpdate(goqu.NoWait), err: "goqu: dialect does not support NOWAIT [dialect=sqlite3]"},
		sqlTestCase{ds: ds.ForUpdate(goqu.SkipLocked), err: "goqu: dialect does not support SKIP LOCKED [dialect=sqlite3]"},
		sqlTestCase{ds: ds.ForUpdate(goqu.Wait, goqu.T("test")), err: "goqu: dialect does not support OF in a locking clause [dialect=sqlite3]"},
	)
}

func (sds *sqlite3DialectSuite) TestUnsupportedLockStrengthsAreErrors() {
	ds := sds.GetDs("test")
	sds.assertSQL(
		sqlTestCase{ds: ds.ForNoKeyUpdate(goqu.Wait), err: "goqu: dialect does not support FOR NO KEY UPDATE [dialect=sqlite3]"},
		sqlTestCase{ds: ds.ForShare(goqu.Wait), err: "goqu: dialect does not support FOR SHARE [dialect=sqlite3]"},
		sqlTestCase{ds: ds.ForKeyShare(goqu.SkipLocked), err: "goqu: dialect does not support FOR KEY SHARE [dialect=sqlite3]"},
	)
}

func TestDatasetAdapterSuite(t *testing.T) {
	suite.Run(t, new(sqlite3DialectSuite))
}

func TestInsertOnConflictDoNothingTarget(t *testing.T) {
	ds := goqu.Dialect("sqlite3").Insert("test").
		Rows(goqu.Record{"a": "a"}).
		OnConflict(goqu.DoNothing().SetCols(exp.NewColumnListExpression(nil, "db", "a")))
	sql, args, err := ds.ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, "INSERT INTO `test` (`a`) VALUES ('a') ON CONFLICT  (`a`) DO NOTHING ", sql)
	assert.Empty(t, args)

	sql, args, err = ds.Prepared(true).ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, "INSERT INTO `test` (`a`) VALUES (?) ON CONFLICT  (`a`) DO NOTHING ", sql)
	assert.Equal(t, []any{"a"}, args)
}

func TestInsertOnConflictDoUpdateExcluded(t *testing.T) {
	sql, _, err := goqu.Dialect("sqlite3").Insert("test").
		Rows(goqu.Record{"a": "a", "b": 1}).
		OnConflict(goqu.DoUpdateExcluded("a", "b")).
		ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, "INSERT INTO `test` (`a`, `b`) VALUES ('a', 1) ON CONFLICT  (a) DO UPDATE SET `b`=EXCLUDED.`b`", sql)
}

func TestEmptyInList(t *testing.T) {
	ds := goqu.Dialect("sqlite3").From("test").Where(goqu.C("a").In([]int{}), goqu.C("b").NotIn([]int{}))
	for _, prepared := range []bool{false, true} {
		sql, args, err := ds.Prepared(prepared).ToSQL()
		assert.NoError(t, err)
		assert.Equal(t, "SELECT * FROM `test` WHERE ((1=0) AND (1=1))", sql)
		assert.Empty(t, args)
	}
}

func TestJSONFunctionsAreErrors(t *testing.T) {
	_, _, err := goqu.Dialect("sqlite3").From("t").Where(goqu.JSONValue("doc", "$.a").Eq(1)).ToSQL()
	assert.EqualError(t, err, "goqu: dialect does not support JSONValue [dialect=sqlite3]")
	assert.True(t, goqu.IsUnsupported(err))
}

func TestIntervalIsError(t *testing.T) {
	_, _, err := goqu.Dialect("sqlite3").From("t").Where(goqu.C("at").Lt(goqu.Now().Sub(goqu.Interval(1, "day")))).ToSQL()
	assert.EqualError(t, err, `goqu: dialect does not support Interval [dialect=sqlite3]`)
	assert.True(t, goqu.IsUnsupported(err))
}

func TestInsertWithoutRows(t *testing.T) {
	ds := goqu.Dialect("sqlite3").Insert("test")
	sql, _, err := ds.ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, "INSERT INTO `test` DEFAULT VALUES", sql)
	sql, _, err = ds.Returning("id").ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, "INSERT INTO `test` DEFAULT VALUES RETURNING `id`", sql)
	sql, _, err = ds.OnConflict(goqu.DoNothing()).ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, "INSERT OR IGNORE INTO  `test` DEFAULT VALUES", sql)
	for name, conflict := range map[string]exp.ConflictExpression{
		"do nothing on a target": goqu.DoNothing().SetCols(exp.NewColumnListExpression(nil, "db", "id")),
		"do update":              goqu.DoUpdate("id", goqu.Record{"a": "b"}),
	} {
		_, _, err = ds.OnConflict(conflict).ToSQL()
		assert.EqualError(t, err, "goqu: dialect does not support upsert with DEFAULT VALUES [dialect=sqlite3]", name)
		assert.True(t, goqu.IsUnsupported(err), name)
	}
}
