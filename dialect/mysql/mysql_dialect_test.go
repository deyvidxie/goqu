package mysql_test

import (
	"regexp"
	"testing"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/sqlgen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type (
	mysqlDialectSuite struct {
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

func (mds *mysqlDialectSuite) GetDs(table string) *goqu.SelectDataset {
	return goqu.Dialect("mysql").From(table)
}

func (mds *mysqlDialectSuite) assertSQL(cases ...sqlTestCase) {
	for i, c := range cases {
		actualSQL, actualArgs, err := c.ds.ToSQL()
		if c.err == "" {
			mds.NoError(err, "test case %d failed", i)
		} else {
			mds.EqualError(err, c.err, "test case %d failed", i)
		}
		mds.Equal(c.sql, actualSQL, "test case %d failed", i)
		if c.isPrepared && c.args != nil || len(c.args) > 0 {
			mds.Equal(c.args, actualArgs, "test case %d failed", i)
		} else {
			mds.Empty(actualArgs, "test case %d failed", i)
		}
	}
}

func (mds *mysqlDialectSuite) TestIdentifiers() {
	ds := mds.GetDs("test")
	mds.assertSQL(
		sqlTestCase{ds: ds.Select(
			"a",
			goqu.I("a.b.c"),
			goqu.I("c.d"),
			goqu.C("test").As("test"),
		), sql: "SELECT `a`, `a`.`b`.`c`, `c`.`d`, `test` AS `test` FROM `test`"},
	)
}

func (mds *mysqlDialectSuite) TestLiteralString() {
	ds := mds.GetDs("test")
	col := goqu.C("a")
	mds.assertSQL(
		sqlTestCase{ds: ds.Where(col.Eq("test")), sql: "SELECT * FROM `test` WHERE (`a` = 'test')"},
		sqlTestCase{ds: ds.Where(col.Eq("test'test")), sql: "SELECT * FROM `test` WHERE (`a` = 'test''test')"},
		sqlTestCase{ds: ds.Where(col.Eq("') OR 1=1 --")), sql: "SELECT * FROM `test` WHERE (`a` = ''') OR 1=1 --')"},
		sqlTestCase{ds: ds.Where(col.Eq("\\' OR 1=1 --")), sql: "SELECT * FROM `test` WHERE (`a` = '\\\\'' OR 1=1 --')"},
		sqlTestCase{ds: ds.Where(col.Eq(`test"test`)), sql: "SELECT * FROM `test` WHERE (`a` = 'test\"test')"},
		sqlTestCase{ds: ds.Where(col.Eq(`test\test`)), sql: "SELECT * FROM `test` WHERE (`a` = 'test\\\\test')"},
		sqlTestCase{ds: ds.Where(col.Eq("test\ntest")), sql: "SELECT * FROM `test` WHERE (`a` = 'test\ntest')"},
		sqlTestCase{ds: ds.Where(col.Eq("test\rtest")), sql: "SELECT * FROM `test` WHERE (`a` = 'test\rtest')"},
		sqlTestCase{ds: ds.Where(col.Eq("test\x00test")), sql: "SELECT * FROM `test` WHERE (`a` = 'test\\0test')"},
		sqlTestCase{ds: ds.Where(col.Eq("test\x1atest")), sql: "SELECT * FROM `test` WHERE (`a` = 'test\\Ztest')"},
	)
}

func (mds *mysqlDialectSuite) TestLiteralBytes() {
	col := goqu.C("a")
	ds := mds.GetDs("test")
	mds.assertSQL(
		sqlTestCase{ds: ds.Where(col.Eq([]byte("test"))), sql: "SELECT * FROM `test` WHERE (`a` = 'test')"},
		sqlTestCase{ds: ds.Where(col.Eq([]byte("test'test"))), sql: "SELECT * FROM `test` WHERE (`a` = 'test''test')"},
		sqlTestCase{ds: ds.Where(col.Eq([]byte(`test"test`))), sql: "SELECT * FROM `test` WHERE (`a` = 'test\"test')"},
		sqlTestCase{ds: ds.Where(col.Eq([]byte(`test\test`))), sql: "SELECT * FROM `test` WHERE (`a` = 'test\\\\test')"},
		sqlTestCase{ds: ds.Where(col.Eq([]byte("test\ntest"))), sql: "SELECT * FROM `test` WHERE (`a` = 'test\ntest')"},
		sqlTestCase{ds: ds.Where(col.Eq([]byte("test\rtest"))), sql: "SELECT * FROM `test` WHERE (`a` = 'test\rtest')"},
		sqlTestCase{ds: ds.Where(col.Eq([]byte("test\x00test"))), sql: "SELECT * FROM `test` WHERE (`a` = 'test\\0test')"},
		sqlTestCase{ds: ds.Where(col.Eq([]byte("test\x1atest"))), sql: "SELECT * FROM `test` WHERE (`a` = 'test\\Ztest')"},
	)
}

func (mds *mysqlDialectSuite) TestBooleanOperations() {
	col := goqu.C("a")
	ds := mds.GetDs("test")
	mds.assertSQL(
		sqlTestCase{ds: ds.Where(col.Eq(true)), sql: "SELECT * FROM `test` WHERE (`a` IS TRUE)"},
		sqlTestCase{ds: ds.Where(col.Eq(false)), sql: "SELECT * FROM `test` WHERE (`a` IS FALSE)"},
		sqlTestCase{ds: ds.Where(col.Is(true)), sql: "SELECT * FROM `test` WHERE (`a` IS TRUE)"},
		sqlTestCase{ds: ds.Where(col.Is(false)), sql: "SELECT * FROM `test` WHERE (`a` IS FALSE)"},
		sqlTestCase{ds: ds.Where(col.IsTrue()), sql: "SELECT * FROM `test` WHERE (`a` IS TRUE)"},
		sqlTestCase{ds: ds.Where(col.IsFalse()), sql: "SELECT * FROM `test` WHERE (`a` IS FALSE)"},
		sqlTestCase{ds: ds.Where(col.Neq(true)), sql: "SELECT * FROM `test` WHERE (`a` IS NOT TRUE)"},
		sqlTestCase{ds: ds.Where(col.Neq(false)), sql: "SELECT * FROM `test` WHERE (`a` IS NOT FALSE)"},
		sqlTestCase{ds: ds.Where(col.IsNot(true)), sql: "SELECT * FROM `test` WHERE (`a` IS NOT TRUE)"},
		sqlTestCase{ds: ds.Where(col.IsNot(false)), sql: "SELECT * FROM `test` WHERE (`a` IS NOT FALSE)"},
		sqlTestCase{ds: ds.Where(col.IsNotTrue()), sql: "SELECT * FROM `test` WHERE (`a` IS NOT TRUE)"},
		sqlTestCase{ds: ds.Where(col.IsNotFalse()), sql: "SELECT * FROM `test` WHERE (`a` IS NOT FALSE)"},
		sqlTestCase{ds: ds.Where(col.Like("a%")), sql: "SELECT * FROM `test` WHERE (`a` LIKE BINARY 'a%')"},
		sqlTestCase{ds: ds.Where(col.NotLike("a%")), sql: "SELECT * FROM `test` WHERE (`a` NOT LIKE BINARY 'a%')"},
		sqlTestCase{ds: ds.Where(col.ILike("a%")), sql: "SELECT * FROM `test` WHERE (`a` LIKE 'a%')"},
		sqlTestCase{ds: ds.Where(col.NotILike("a%")), sql: "SELECT * FROM `test` WHERE (`a` NOT LIKE 'a%')"},
		sqlTestCase{ds: ds.Where(col.Like(regexp.MustCompile("[ab]"))), sql: "SELECT * FROM `test` WHERE (`a` REGEXP BINARY '[ab]')"},
		sqlTestCase{ds: ds.Where(col.NotLike(regexp.MustCompile("[ab]"))), sql: "SELECT * FROM `test` WHERE (`a` NOT REGEXP BINARY '[ab]')"},
		sqlTestCase{ds: ds.Where(col.ILike(regexp.MustCompile("[ab]"))), sql: "SELECT * FROM `test` WHERE (`a` REGEXP '[ab]')"},
		sqlTestCase{ds: ds.Where(col.NotILike(regexp.MustCompile("[ab]"))), sql: "SELECT * FROM `test` WHERE (`a` NOT REGEXP '[ab]')"},
	)
}

func (mds *mysqlDialectSuite) TestArithmeticOperations() {
	col := goqu.C("a")
	ds := mds.GetDs("test")
	mds.assertSQL(
		sqlTestCase{ds: ds.Where(col.Add(1).Eq(2)), sql: "SELECT * FROM `test` WHERE ((`a` + 1) = 2)"},
		sqlTestCase{ds: ds.Where(col.Sub(1).Gte(3)), sql: "SELECT * FROM `test` WHERE ((`a` - 1) >= 3)"},
		sqlTestCase{ds: ds.Where(col.Mul(1)), sql: "SELECT * FROM `test` WHERE (`a` * 1)"},
		sqlTestCase{ds: ds.Where(col.Div(1)), sql: "SELECT * FROM `test` WHERE (`a` / 1)"},
	)
}

func (mds *mysqlDialectSuite) TestJSONFunctions() {
	for _, dialect := range []string{"mysql", "mysql8", "mysql8nano"} {
		ds := goqu.Dialect(dialect).From("test")
		mds.assertSQL(
			sqlTestCase{
				ds:  ds.Select(goqu.JSONExtract("doc", "$.a")),
				sql: "SELECT JSON_EXTRACT(`doc`, '$.a') FROM `test`",
			},
			sqlTestCase{
				ds:  ds.Where(goqu.JSONValue("t.doc", "$.name").Eq("bob")),
				sql: "SELECT * FROM `test` WHERE (JSON_UNQUOTE(JSON_EXTRACT(`t`.`doc`, '$.name')) = 'bob')",
			},
			sqlTestCase{
				ds:  ds.Where(goqu.JSONContains("doc", "a\x01\"b", "$.tags")),
				sql: "SELECT * FROM `test` WHERE JSON_CONTAINS(`doc`, '\"a\\\\u0001\\\\\"b\"', '$.tags')",
			},
			sqlTestCase{
				ds:         ds.Prepared(true).Where(goqu.JSONContains("doc", []int{1, 2})),
				sql:        "SELECT * FROM `test` WHERE JSON_CONTAINS(`doc`, ?)",
				isPrepared: true,
				args:       []interface{}{"[1,2]"},
			},
		)
	}
}

func (mds *mysqlDialectSuite) TestBitwiseOperations() {
	col := goqu.C("a")
	ds := mds.GetDs("test")
	mds.assertSQL(
		sqlTestCase{ds: ds.Where(col.BitwiseInversion()), sql: "SELECT * FROM `test` WHERE (~ `a`)"},
		sqlTestCase{ds: ds.Where(col.BitwiseAnd(1)), sql: "SELECT * FROM `test` WHERE (`a` & 1)"},
		sqlTestCase{ds: ds.Where(col.BitwiseOr(1)), sql: "SELECT * FROM `test` WHERE (`a` | 1)"},
		sqlTestCase{ds: ds.Where(col.BitwiseXor(1)), sql: "SELECT * FROM `test` WHERE (`a` ^ 1)"},
		sqlTestCase{ds: ds.Where(col.BitwiseLeftShift(1)), sql: "SELECT * FROM `test` WHERE (`a` << 1)"},
		sqlTestCase{ds: ds.Where(col.BitwiseRightShift(1)), sql: "SELECT * FROM `test` WHERE (`a` >> 1)"},
	)
}

func (mds *mysqlDialectSuite) TestFuns() {
	col := goqu.C("a")
	ds := mds.GetDs("test")
	mds.assertSQL(
		sqlTestCase{ds: ds.Select(goqu.GREATEST(col, 2)), sql: "SELECT GREATEST(`a`, 2) FROM `test`"},
		sqlTestCase{ds: ds.Select(goqu.LEAST(col, 2)), sql: "SELECT LEAST(`a`, 2) FROM `test`"},
		sqlTestCase{ds: ds.Select(goqu.UPPER(col)), sql: "SELECT UCASE(`a`) FROM `test`"},
		sqlTestCase{ds: ds.Select(goqu.UPPER("Val")), sql: "SELECT UCASE('Val') FROM `test`"},
		sqlTestCase{ds: ds.Select(goqu.LOWER(col)), sql: "SELECT LCASE(`a`) FROM `test`"},
		sqlTestCase{ds: ds.Select(goqu.LOWER("Val")), sql: "SELECT LCASE('Val') FROM `test`"},
	)
}

func (mds *mysqlDialectSuite) TestUpdateSQL() {
	ds := mds.GetDs("test").Update()
	mds.assertSQL(
		sqlTestCase{
			ds: ds.
				Set(goqu.Record{"foo": "bar"}).
				From("test_2").
				Where(goqu.I("test.id").Eq(goqu.I("test_2.test_id"))),
			sql: "UPDATE `test`,`test_2` SET `foo`='bar' WHERE (`test`.`id` = `test_2`.`test_id`)",
		},
	)
}

func (mds *mysqlDialectSuite) TestStraightJoin() {
	on := goqu.On(goqu.I("test.id").Eq(goqu.I("foo.test_id")))
	for _, dialect := range []string{"mysql", "mysql8"} {
		ds := goqu.Dialect(dialect).From("test")
		mds.assertSQL(
			sqlTestCase{
				ds:  ds.StraightJoin(goqu.T("foo"), on),
				sql: "SELECT * FROM `test` STRAIGHT_JOIN `foo` ON (`test`.`id` = `foo`.`test_id`)",
			},
			sqlTestCase{
				// MySQL 5.7 does not accept USING for STRAIGHT_JOIN
				ds:  ds.StraightJoin(goqu.T("foo"), goqu.Using("test_id")),
				err: "goqu: USING is not supported for StraightJoinType, use an ON condition",
			},
			sqlTestCase{
				ds:         ds.Prepared(true).StraightJoin(goqu.T("foo"), goqu.On(goqu.I("test.id").Eq(5))),
				sql:        "SELECT * FROM `test` STRAIGHT_JOIN `foo` ON (`test`.`id` = ?)",
				isPrepared: true,
				args:       []any{int64(5)},
			},
			sqlTestCase{
				ds:  ds.Join(goqu.T("a"), goqu.Using("id")).StraightJoin(goqu.T("b"), goqu.On(goqu.I("a.id").Eq(goqu.I("b.id")))),
				sql: "SELECT * FROM `test` INNER JOIN `a` USING (`id`) STRAIGHT_JOIN `b` ON (`a`.`id` = `b`.`id`)",
			},
			sqlTestCase{
				ds:  ds.StraightJoin(goqu.T("foo"), nil),
				err: "goqu: join condition required for conditioned join StraightJoinType",
			},
		)
	}
}

func (mds *mysqlDialectSuite) TestWithCTE() {
	cte := func(dialect string) *goqu.SelectDataset {
		return goqu.Dialect(dialect).From("a").Select("x")
	}
	// MySQL 8 supports common table expressions, plain mysql (5.x) does not.
	mds.assertSQL(
		sqlTestCase{
			ds:  goqu.Dialect("mysql8").From("c").With("c", cte("mysql8")),
			sql: "WITH c AS (SELECT `x` FROM `a`) SELECT * FROM `c`",
		},
		sqlTestCase{
			ds:  goqu.Dialect("mysql8").From("c").WithRecursive("c", cte("mysql8")),
			sql: "WITH RECURSIVE c AS (SELECT `x` FROM `a`) SELECT * FROM `c`",
		},
		sqlTestCase{
			ds:  goqu.Dialect("mysql").From("c").With("c", cte("mysql")),
			err: "goqu: dialect does not support CTE WITH clause [dialect=mysql]",
		},
	)
}

func (mds *mysqlDialectSuite) TestTimeLiteralPrecision() {
	where := goqu.C("t").Eq(time.Date(2024, 1, 2, 3, 4, 5, 123456789, time.UTC))
	mds.assertSQL(
		sqlTestCase{
			ds:  goqu.Dialect("mysql").From("test").Where(where),
			sql: "SELECT * FROM `test` WHERE (`t` = '2024-01-02 03:04:05')",
		},
		sqlTestCase{
			ds:  goqu.Dialect("mysql8").From("test").Where(where),
			sql: "SELECT * FROM `test` WHERE (`t` = '2024-01-02 03:04:05')",
		},
		sqlTestCase{
			ds:  goqu.Dialect("mysql8nano").From("test").Where(where),
			sql: "SELECT * FROM `test` WHERE (`t` = '2024-01-02 03:04:05.123456789')",
		},
	)
}

func (mds *mysqlDialectSuite) TestLockClauses() {
	mysql := goqu.Dialect("mysql").From("test")
	mysql8 := goqu.Dialect("mysql8").From("test")
	mds.assertSQL(
		sqlTestCase{ds: mysql.ForUpdate(goqu.Wait), sql: "SELECT * FROM `test` FOR UPDATE "},
		sqlTestCase{ds: mysql.ForShare(goqu.Wait), sql: "SELECT * FROM `test` LOCK IN SHARE MODE "},
		sqlTestCase{ds: mysql.ForUpdate(goqu.NoWait), err: "goqu: dialect does not support NOWAIT [dialect=mysql]"},
		sqlTestCase{ds: mysql.ForShare(goqu.SkipLocked), err: "goqu: dialect does not support SKIP LOCKED [dialect=mysql]"},
		sqlTestCase{
			ds:  mysql.ForUpdate(goqu.Wait, goqu.T("test")),
			err: "goqu: dialect does not support OF in a locking clause [dialect=mysql]",
		},
		sqlTestCase{ds: mysql.ForNoKeyUpdate(goqu.Wait), err: "goqu: dialect does not support FOR NO KEY UPDATE [dialect=mysql]"},
		sqlTestCase{ds: mysql.ForKeyShare(goqu.Wait), err: "goqu: dialect does not support FOR KEY SHARE [dialect=mysql]"},
		sqlTestCase{ds: mysql8.ForUpdate(goqu.SkipLocked), sql: "SELECT * FROM `test` FOR UPDATE SKIP LOCKED"},
		sqlTestCase{ds: mysql8.ForShare(goqu.NoWait), sql: "SELECT * FROM `test` FOR SHARE NOWAIT"},
		sqlTestCase{ds: mysql8.ForUpdate(goqu.Wait, goqu.T("test")), sql: "SELECT * FROM `test` FOR UPDATE OF `test` "},
		sqlTestCase{ds: mysql8.ForNoKeyUpdate(goqu.Wait), err: "goqu: dialect does not support FOR NO KEY UPDATE [dialect=mysql8]"},
		sqlTestCase{ds: mysql8.ForKeyShare(goqu.Wait), err: "goqu: dialect does not support FOR KEY SHARE [dialect=mysql8]"},
	)
}

func TestDatasetAdapterSuite(t *testing.T) {
	suite.Run(t, new(mysqlDialectSuite))
}

func TestInsertOnConflictDoNothingTargetNotSupported(t *testing.T) {
	_, _, err := goqu.Dialect("mysql").Insert("test").
		Rows(goqu.Record{"a": "a"}).
		OnConflict(goqu.DoNothing().SetCols(exp.NewColumnListExpression(nil, "db", "a"))).
		ToSQL()
	assert.EqualError(t, err, "goqu: dialect does not support Conflict Target clause [dialect=mysql]")

	// no target is still fine
	_, _, err = goqu.Dialect("mysql").Insert("test").
		Rows(goqu.Record{"a": "a"}).
		OnConflict(goqu.DoNothing()).
		ToSQL()
	assert.NoError(t, err)
}

func TestInsertOnConflictDoUpdateExcluded(t *testing.T) {
	cases := []struct {
		name    string
		dialect string
		ds      func(d goqu.DialectWrapper) *goqu.InsertDataset
		sql     string
		err     string
	}{
		{
			name:    "mysql values func",
			dialect: "mysql",
			ds: func(d goqu.DialectWrapper) *goqu.InsertDataset {
				return d.Insert("test").Rows(goqu.Record{"a": "a", "b": 1}).OnConflict(goqu.DoUpdateExcluded("", "b"))
			},
			sql: "INSERT INTO `test` (`a`, `b`) VALUES ('a', 1) ON DUPLICATE KEY UPDATE `b`=VALUES(`b`)",
		},
		{
			name:    "mysql insert select with cols",
			dialect: "mysql",
			ds: func(d goqu.DialectWrapper) *goqu.InsertDataset {
				return d.Insert("test").Cols("a", "b").FromQuery(d.From("src").Select("x", "y")).
					OnConflict(goqu.DoUpdateExcluded("", "b"))
			},
			sql: "INSERT INTO `test` (`a`, `b`) SELECT `x`, `y` FROM `src` ON DUPLICATE KEY UPDATE `b`=VALUES(`b`)",
		},
		{
			name:    "mysql insert select",
			dialect: "mysql",
			ds: func(d goqu.DialectWrapper) *goqu.InsertDataset {
				return d.Insert("test").FromQuery(d.From("src").Select("x", "y")).OnConflict(goqu.DoUpdateExcluded("", "b"))
			},
			sql: "INSERT INTO `test` SELECT `x`, `y` FROM `src` ON DUPLICATE KEY UPDATE `b`=VALUES(`b`)",
		},
		{
			name:    "mysql8 row alias",
			dialect: "mysql8",
			ds: func(d goqu.DialectWrapper) *goqu.InsertDataset {
				return d.Insert("test").Rows(goqu.Record{"a": "a", "b": 1}).OnConflict(goqu.DoUpdateExcluded("", "b"))
			},
			sql: "INSERT INTO `test` (`a`, `b`) VALUES ('a', 1) AS `excluded` ON DUPLICATE KEY UPDATE `b`=`excluded`.`b`",
		},
		{
			name:    "mysql8nano reuses As alias",
			dialect: "mysql8nano",
			ds: func(d goqu.DialectWrapper) *goqu.InsertDataset {
				return d.Insert("test").Rows(goqu.Record{"a": "a", "b": 1}).As("new").
					OnConflict(goqu.DoUpdateExcluded("", "b"))
			},
			sql: "INSERT INTO `test` (`a`, `b`) VALUES ('a', 1) AS `new` ON DUPLICATE KEY UPDATE `b`=`new`.`b`",
		},
		{
			name:    "mysql8 without Excluded adds no alias",
			dialect: "mysql8",
			ds: func(d goqu.DialectWrapper) *goqu.InsertDataset {
				return d.Insert("test").Rows(goqu.Record{"a": "a", "b": 1}).
					OnConflict(goqu.DoUpdate("", goqu.Record{"b": goqu.L("b + 1")}))
			},
			sql: "INSERT INTO `test` (`a`, `b`) VALUES ('a', 1) ON DUPLICATE KEY UPDATE `b`=b + 1",
		},
		{
			name:    "mysql8 insert select derived table",
			dialect: "mysql8",
			ds: func(d goqu.DialectWrapper) *goqu.InsertDataset {
				return d.Insert("test").Cols("a", "b").FromQuery(d.From("src").Select("x", "y")).OnConflict(goqu.DoUpdateExcluded("", "b"))
			},
			sql: "INSERT INTO `test` (`a`, `b`) SELECT * FROM (SELECT `x`, `y` FROM `src`) AS `excluded` (`a`, `b`) " +
				"ON DUPLICATE KEY UPDATE `b`=`excluded`.`b`",
		},
		{
			name:    "mysql8 insert select without Cols",
			dialect: "mysql8",
			ds: func(d goqu.DialectWrapper) *goqu.InsertDataset {
				return d.Insert("test").FromQuery(d.From("src").Select("x", "y")).OnConflict(goqu.DoUpdateExcluded("", "b"))
			},
			err: "goqu: dialect requires Cols(...).FromQuery(...) to use Excluded with INSERT ... SELECT [dialect=mysql8]",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sql, _, err := tc.ds(goqu.Dialect(tc.dialect)).ToSQL()
			if tc.err != "" {
				assert.EqualError(t, err, tc.err)
				assert.True(t, sqlgen.IsUnsupported(err))
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.sql, sql)
		})
	}
}

func TestEmptyInList(t *testing.T) {
	for _, dialect := range []string{"mysql", "mysql8"} {
		ds := goqu.Dialect(dialect).From("test").Where(goqu.C("a").In([]int{}), goqu.C("b").NotIn([]int{}))
		for _, prepared := range []bool{false, true} {
			sql, args, err := ds.Prepared(prepared).ToSQL()
			assert.NoError(t, err, dialect)
			assert.Equal(t, "SELECT * FROM `test` WHERE ((1=0) AND (1=1))", sql, dialect)
			assert.Empty(t, args, dialect)
		}
	}
}

func TestInsertOnConflictExcludedBesideEmptyIn(t *testing.T) {
	sql, args, err := goqu.Dialect("mysql8").Insert("test").
		Rows(goqu.Record{"a": "a", "b": 1, "c": 1}).
		OnConflict(goqu.DoUpdate("a", goqu.Record{
			"b": goqu.Excluded("b"),
			"c": goqu.Case().When(goqu.C("c").In([]int{}), 1).Else(2),
		})).
		ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, "INSERT INTO `test` (`a`, `b`, `c`) VALUES ('a', 1, 1) AS `excluded` "+
		"ON DUPLICATE KEY UPDATE `b`=`excluded`.`b`,`c`=CASE  WHEN (1=0) THEN 1 ELSE 2 END", sql)
	assert.Empty(t, args)
}

func TestInsertOnConflictEmptyInOnExcluded(t *testing.T) {
	sql, args, err := goqu.Dialect("mysql8").Insert("test").
		Rows(goqu.Record{"a": "a", "b": 1}).
		OnConflict(goqu.DoUpdate("a", goqu.Record{"b": goqu.Case().When(goqu.Excluded("b").In([]int{}), 1).Else(2)})).
		ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, "INSERT INTO `test` (`a`, `b`) VALUES ('a', 1) AS `excluded` "+
		"ON DUPLICATE KEY UPDATE `b`=CASE  WHEN (1=0) THEN 1 ELSE 2 END", sql)
	assert.Empty(t, args)
}
