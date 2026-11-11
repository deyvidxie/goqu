package mysql_test

import (
	"regexp"
	"testing"
	"time"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/mysql"
	"github.com/doug-martin/goqu/v9/exp"
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

func (mds *mysqlDialectSuite) TestTimeFunctions() {
	for _, dialect := range []string{"mysql", "mysql8", "mysql8nano"} {
		ds := goqu.Dialect(dialect).From("test")
		mds.assertSQL(
			sqlTestCase{
				ds:  ds.Select(goqu.Now(), goqu.Now(6), goqu.UTCTimestamp(), goqu.UTCTimestamp(3)),
				sql: "SELECT NOW(), NOW(6), UTC_TIMESTAMP(), UTC_TIMESTAMP(3) FROM `test`",
			},
			sqlTestCase{
				ds:  ds.Where(goqu.C("at").Lt(goqu.Now(6).Add(goqu.Interval(1, "second")))),
				sql: "SELECT * FROM `test` WHERE (`at` < (NOW(6) + INTERVAL 1 SECOND))",
			},
			sqlTestCase{
				ds:         ds.Prepared(true).Where(goqu.C("at").Gt(goqu.UTCTimestamp().Sub(goqu.Interval(2, "quarter")))),
				sql:        "SELECT * FROM `test` WHERE (`at` > (UTC_TIMESTAMP() - INTERVAL ? QUARTER))",
				isPrepared: true,
				args:       []interface{}{int64(2)},
			},
		)
	}
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
				err: "goqu: dialect does not support USING for StraightJoinType, use an ON condition [dialect=" + dialect + "]",
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

func (mds *mysqlDialectSuite) TestFullJoinsAreUnsupported() {
	for _, dialect := range []string{"mysql", "mysql8", "mysql8nano"} {
		ds := goqu.Dialect(dialect).From("test")
		on := goqu.On(goqu.I("test.id").Eq(goqu.I("foo.test_id")))
		for joinType, joined := range map[string]*goqu.SelectDataset{
			"FullJoinType":        ds.FullJoin(goqu.T("foo"), on),
			"FullOuterJoinType":   ds.FullOuterJoin(goqu.T("foo"), on),
			"NaturalFullJoinType": ds.NaturalFullJoin(goqu.T("foo")),
		} {
			_, _, err := joined.ToSQL()
			mds.EqualError(err, "goqu: dialect does not support "+joinType+" [dialect="+dialect+"]")
			mds.True(goqu.IsUnsupported(err), joinType)
		}
	}
}

func (mds *mysqlDialectSuite) TestLateral() {
	lateral := func(dialect string) exp.LateralExpression {
		return goqu.Lateral(goqu.Dialect(dialect).From("other").Select("c").As("o"))
	}
	_, _, err := goqu.Dialect("mysql").From("test").Join(lateral("mysql"), goqu.On(goqu.V(true))).ToSQL()
	mds.EqualError(err, "goqu: dialect does not support lateral expressions [dialect=mysql]")
	mds.True(goqu.IsUnsupported(err))
	for _, dialect := range []string{"mysql8", "mysql8nano"} {
		mds.assertSQL(sqlTestCase{
			ds:  goqu.Dialect(dialect).From("test").Join(lateral(dialect), goqu.On(goqu.V(true))),
			sql: "SELECT * FROM `test` INNER JOIN LATERAL (SELECT `c` FROM `other`) AS `o` ON 1",
		})
	}
}

func (mds *mysqlDialectSuite) TestTruncate() {
	for _, d := range []string{"mysql", "mysql8", "mysql8nano"} {
		ds := goqu.Dialect(d).From("test")
		mds.assertSQL(
			sqlTestCase{ds: ds.Truncate(), sql: "TRUNCATE `test`"},
			sqlTestCase{
				ds:  ds.Truncate().Identity("restart"),
				err: "goqu: dialect does not support RESTART IDENTITY in TRUNCATE [dialect=" + d + "]",
			},
			sqlTestCase{
				ds:  goqu.Dialect(d).Truncate("a", "b"),
				err: "goqu: dialect does not support multiple tables in TRUNCATE [dialect=" + d + "]",
			},
			sqlTestCase{ds: ds.Truncate().Cascade(), err: "goqu: dialect does not support CASCADE in TRUNCATE [dialect=" + d + "]"},
			sqlTestCase{ds: ds.Truncate().Restrict(), err: "goqu: dialect does not support RESTRICT in TRUNCATE [dialect=" + d + "]"},
		)
	}
}

func (mds *mysqlDialectSuite) TestLimitAllAndOffset() {
	for _, d := range []string{"mysql", "mysql8", "mysql8nano"} {
		ds := goqu.Dialect(d).From("test")
		mds.assertSQL(
			sqlTestCase{ds: ds.LimitAll(), sql: "SELECT * FROM `test`"},
			sqlTestCase{ds: ds.Offset(2), sql: "SELECT * FROM `test` LIMIT 18446744073709551615 OFFSET 2"},
			sqlTestCase{ds: ds.LimitAll().Offset(2), sql: "SELECT * FROM `test` LIMIT 18446744073709551615 OFFSET 2"},
			sqlTestCase{ds: ds.Limit(3).Offset(2), sql: "SELECT * FROM `test` LIMIT 3 OFFSET 2"},
		)
	}
}

func (mds *mysqlDialectSuite) TestLimitZero() {
	d := goqu.Dialect("mysql")
	mds.assertSQL(
		sqlTestCase{ds: d.From("test").Limit(0), sql: "SELECT * FROM `test` LIMIT 0"},
		sqlTestCase{ds: d.From("test").Limit(0).Offset(2), sql: "SELECT * FROM `test` LIMIT 0 OFFSET 2"},
		sqlTestCase{ds: d.Update("test").Set(goqu.Record{"a": 1}).Limit(0), sql: "UPDATE `test` SET `a`=1 LIMIT 0"},
		sqlTestCase{ds: d.Delete("test").Where(goqu.C("a").Eq(1)).Limit(0), sql: "DELETE FROM `test` WHERE (`a` = 1) LIMIT 0"},
		sqlTestCase{
			ds:         d.Delete("test").Prepared(true).Where(goqu.C("a").Eq(1)).Limit(0),
			sql:        "DELETE FROM `test` WHERE (`a` = ?) LIMIT ?",
			isPrepared: true,
			args:       []interface{}{int64(1), int64(0)},
		},
	)
}

func (mds *mysqlDialectSuite) TestInsertRowAlias() {
	mds.assertSQL(
		sqlTestCase{
			ds:  goqu.Dialect("mysql").Insert("test").Rows(goqu.Record{"a": 1}).As("new"),
			err: "goqu: dialect does not support an INSERT row alias [dialect=mysql]",
		},
	)
	for _, d := range []string{"mysql8", "mysql8nano"} {
		src := goqu.Dialect(d).From("src").Select("a")
		mds.assertSQL(
			sqlTestCase{
				ds:  goqu.Dialect(d).Insert("test").Rows(goqu.Record{"a": 1}).As("new"),
				sql: "INSERT INTO `test` (`a`) VALUES (1) AS `new`",
			},
			sqlTestCase{
				ds:  goqu.Dialect(d).Insert("test").Cols("a").FromQuery(src).As("new"),
				sql: "INSERT INTO `test` (`a`) SELECT * FROM (SELECT `a` FROM `src`) AS `new` (`a`)",
			},
			sqlTestCase{
				ds: goqu.Dialect(d).Insert("test").FromQuery(src).As("new"),
				err: "goqu: dialect does not support an INSERT row alias with INSERT ... SELECT without Cols(...).FromQuery(...) " +
					"[dialect=" + d + "]",
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
			sql: "SELECT * FROM `test` WHERE (`t` = '2024-01-02 03:04:05.123456')",
		},
		sqlTestCase{
			ds:  goqu.Dialect("mysql8").From("test").Where(where),
			sql: "SELECT * FROM `test` WHERE (`t` = '2024-01-02 03:04:05.123456')",
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
	assert.EqualError(t, err, "goqu: dialect does not support conflict target clause [dialect=mysql]")
}

func TestInsertDoNothingIsANoOpUpdate(t *testing.T) {
	for _, dialect := range []string{"mysql", "mysql8"} {
		d := goqu.Dialect(dialect)
		sql, _, err := d.Insert("test").Rows(goqu.Record{"id": 1, "name": "x"}).OnConflict(goqu.DoNothing()).ToSQL()
		assert.NoError(t, err, dialect)
		assert.Equal(t, "INSERT INTO `test` (`id`, `name`) VALUES (1, 'x') ON DUPLICATE KEY UPDATE `id`=`id`", sql, dialect)

		_, _, err = d.Insert("test").FromQuery(d.From("src")).OnConflict(goqu.DoNothing()).ToSQL()
		assert.True(t, goqu.IsUnsupported(err), dialect)
	}
}

func TestInsertOnConflictDoUpdateTarget(t *testing.T) {
	for _, dialect := range []string{"mysql", "mysql8", "mysql8nano"} {
		ds := goqu.Dialect(dialect).Insert("test").Rows(goqu.Record{"a": "a", "b": 1})
		for _, upsert := range []*goqu.InsertDataset{
			ds.OnConflict(goqu.DoUpdate("a", goqu.Record{"b": 2})),
			ds.OnConflict(goqu.DoUpdateExcluded("a", "b")),
		} {
			_, _, err := upsert.ToSQL()
			assert.EqualError(t, err, "goqu: dialect does not support conflict target clause [dialect="+dialect+"]")
			assert.True(t, goqu.IsUnsupported(err), dialect)
		}
		sql, _, err := ds.OnConflict(goqu.DoUpdate("", goqu.Record{"b": 2})).ToSQL()
		assert.NoError(t, err, dialect)
		assert.Contains(t, sql, " ON DUPLICATE KEY UPDATE `b`=", dialect)
	}
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
			err: "goqu: dialect does not support Excluded with INSERT ... SELECT without Cols(...).FromQuery(...) [dialect=mysql8]",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sql, _, err := tc.ds(goqu.Dialect(tc.dialect)).ToSQL()
			if tc.err != "" {
				assert.EqualError(t, err, tc.err)
				assert.True(t, goqu.IsUnsupported(err))
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
		OnConflict(goqu.DoUpdate("", goqu.Record{
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
		OnConflict(goqu.DoUpdate("", goqu.Record{"b": goqu.Case().When(goqu.Excluded("b").In([]int{}), 1).Else(2)})).
		ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, "INSERT INTO `test` (`a`, `b`) VALUES ('a', 1) AS `excluded` "+
		"ON DUPLICATE KEY UPDATE `b`=CASE  WHEN (1=0) THEN 1 ELSE 2 END", sql)
	assert.Empty(t, args)
}

func TestInsertOnConflictEmptyWhere(t *testing.T) {
	ds := goqu.Dialect("mysql").Insert("test").Rows(goqu.Record{"a": 1})
	for _, where := range [][]goqu.Expression{{goqu.Ex{}}, {goqu.ExOr{}}, {}} {
		_, _, err := ds.OnConflict(goqu.DoUpdate("", goqu.Record{"a": 2}).Where(where...)).ToSQL()
		assert.EqualError(t, err, "goqu: dialect does not support upsert with where clause [dialect=mysql]", "%v", where)
		assert.True(t, goqu.IsUnsupported(err), "%v", where)
	}
}

func TestInsertWithoutRows(t *testing.T) {
	for _, d := range []string{"mysql", "mysql8", "mysql8nano"} {
		ds := goqu.Dialect(d).Insert("test")
		sql, _, err := ds.ToSQL()
		assert.NoError(t, err, d)
		assert.Equal(t, "INSERT INTO `test` () VALUES ()", sql, d)
		sql, _, err = ds.OnConflict(goqu.DoUpdate("", goqu.Record{"a": "b"})).ToSQL()
		assert.NoError(t, err, d)
		assert.Equal(t, "INSERT INTO `test` () VALUES () ON DUPLICATE KEY UPDATE `a`='b'", sql, d)
	}
	for d, want := range map[string]string{
		"mysql":      "INSERT INTO `test` () VALUES () ON DUPLICATE KEY UPDATE `a`=VALUES(`a`)",
		"mysql8":     "INSERT INTO `test` () VALUES () AS `excluded` ON DUPLICATE KEY UPDATE `a`=`excluded`.`a`",
		"mysql8nano": "INSERT INTO `test` () VALUES () AS `excluded` ON DUPLICATE KEY UPDATE `a`=`excluded`.`a`",
	} {
		sql, _, err := goqu.Dialect(d).Insert("test").OnConflict(goqu.DoUpdateExcluded("", "a")).ToSQL()
		assert.NoError(t, err, d)
		assert.Equal(t, want, sql, d)
	}
}
