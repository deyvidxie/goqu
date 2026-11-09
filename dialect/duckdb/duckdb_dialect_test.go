package duckdb_test

import (
	"regexp"
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/stretchr/testify/suite"
)

type (
	duckdbDialectSuite struct {
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

func (sds *duckdbDialectSuite) GetDs(table string) *goqu.SelectDataset {
	return goqu.Dialect("duckdb").From(table)
}

func (sds *duckdbDialectSuite) assertSQL(cases ...sqlTestCase) {
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

func (sds *duckdbDialectSuite) TestTimeFunctions() {
	sds.assertSQL(
		sqlTestCase{
			ds: sds.GetDs("t").Select(goqu.Now(3), goqu.UTCTimestamp()).Where(goqu.C("at").Lt(goqu.Now().Sub(goqu.Interval(1, "day")))),
			sql: `SELECT CAST(CAST(now() AS TIMESTAMP) AS TIMESTAMP_MS), make_timestamp(epoch_us(now())) FROM "t" ` +
				`WHERE ("at" < (CAST(now() AS TIMESTAMP) - (1 * INTERVAL '1 DAY')))`,
		},
		sqlTestCase{
			ds: sds.GetDs("t").Prepared(true).Select(goqu.Now(3), goqu.UTCTimestamp()).
				Where(goqu.C("at").Lt(goqu.Now().Sub(goqu.Interval(1, "day")))),
			sql: `SELECT CAST(CAST(now() AS TIMESTAMP) AS TIMESTAMP_MS), make_timestamp(epoch_us(now())) FROM "t" ` +
				`WHERE ("at" < (CAST(now() AS TIMESTAMP) - (? * INTERVAL '1 DAY')))`,
			isPrepared: true,
			args:       []interface{}{int64(1)},
		},
	)
}

func (sds *duckdbDialectSuite) TestIdentifiers() {
	ds := sds.GetDs("test")
	sds.assertSQL(
		sqlTestCase{ds: ds.Select(
			"a",
			goqu.I("a.b.c"),
			goqu.I("c.d"),
			goqu.C("test").As("test"),
		), sql: "SELECT \"a\", \"a\".\"b\".\"c\", \"c\".\"d\", \"test\" AS \"test\" FROM \"test\""},
	)
}

func (sds *duckdbDialectSuite) TestUpdateSQL_multipleTables() {
	ds := sds.GetDs("test").Update()
	sds.assertSQL(
		sqlTestCase{
			ds: ds.
				Set(goqu.Record{"foo": "bar"}).
				From("test_2").
				Where(goqu.I("test.id").Eq(goqu.I("test_2.test_id"))),
			sql: "UPDATE \"test\" SET \"foo\"='bar' FROM \"test_2\" WHERE (\"test\".\"id\" = \"test_2\".\"test_id\")",
		},
	)
}

func (sds *duckdbDialectSuite) TestInsertOnConflict() {
	ds := goqu.Dialect("duckdb").Insert("test").Rows(goqu.Record{"a": "a"})
	doNothing := ds.OnConflict(goqu.DoNothing())
	doNothingTarget := ds.OnConflict(goqu.DoNothing().SetCols(exp.NewColumnListExpression(nil, "db", "a")))
	doUpdate := ds.OnConflict(goqu.DoUpdate("a", goqu.Record{"a": "b"}))
	sds.assertSQL(
		sqlTestCase{ds: doNothing, sql: "INSERT INTO \"test\" (\"a\") VALUES ('a') ON CONFLICT  DO NOTHING "},
		sqlTestCase{
			ds:         doNothing.Prepared(true),
			sql:        "INSERT INTO \"test\" (\"a\") VALUES (?) ON CONFLICT  DO NOTHING ",
			isPrepared: true,
			args:       []interface{}{"a"},
		},
		sqlTestCase{ds: doNothingTarget, sql: "INSERT INTO \"test\" (\"a\") VALUES ('a') ON CONFLICT  (\"a\") DO NOTHING "},
		sqlTestCase{
			ds:         doNothingTarget.Prepared(true),
			sql:        "INSERT INTO \"test\" (\"a\") VALUES (?) ON CONFLICT  (\"a\") DO NOTHING ",
			isPrepared: true,
			args:       []interface{}{"a"},
		},
		sqlTestCase{ds: doUpdate, sql: "INSERT INTO \"test\" (\"a\") VALUES ('a') ON CONFLICT  (a) DO UPDATE SET \"a\"='b'"},
		sqlTestCase{
			ds: goqu.Dialect("duckdb").Insert("test").Rows(goqu.Record{"a": "a", "b": "b"}).
				OnConflict(goqu.DoUpdateExcluded("a", "b")),
			sql: "INSERT INTO \"test\" (\"a\", \"b\") VALUES ('a', 'b') ON CONFLICT  (a) DO UPDATE SET \"b\"=EXCLUDED.\"b\"",
		},
		sqlTestCase{
			ds:  ds.OnConflict(goqu.DoUpdate("a", goqu.Record{"a": "b"}).Where(goqu.C("a").Neq("b"))),
			sql: "INSERT INTO \"test\" (\"a\") VALUES ('a') ON CONFLICT  (a) DO UPDATE SET \"a\"='b' WHERE (\"a\" != 'b')",
		},
		sqlTestCase{
			ds:         doUpdate.Prepared(true),
			sql:        "INSERT INTO \"test\" (\"a\") VALUES (?) ON CONFLICT  (a) DO UPDATE SET \"a\"=?",
			isPrepared: true,
			args:       []interface{}{"a", "b"},
		},
	)
}

func (sds *duckdbDialectSuite) TestDistinctOn() {
	sds.assertSQL(sqlTestCase{
		ds:  sds.GetDs("test").Select("a").Distinct("b"),
		sql: "SELECT DISTINCT ON (\"b\") \"a\" FROM \"test\"",
	})
}

func (sds *duckdbDialectSuite) TestWindowBeforeQualify() {
	sds.assertSQL(sqlTestCase{
		ds: sds.GetDs("test").Select("a").
			Window(goqu.W("w").PartitionBy("b")).
			Qualify(goqu.ROW_NUMBER().OverName(goqu.I("w")).Eq(1)).
			Order(goqu.C("a").Asc()),
		sql: "SELECT \"a\" FROM \"test\" WINDOW \"w\" AS (PARTITION BY \"b\") QUALIFY (ROW_NUMBER() OVER \"w\" = 1) ORDER BY \"a\" ASC",
	})
}

func (sds *duckdbDialectSuite) TestLateral() {
	sds.assertSQL(sqlTestCase{
		ds:  sds.GetDs("test").Join(goqu.Lateral(goqu.From("other").Select("c").As("o")), goqu.On(goqu.V(true))),
		sql: "SELECT * FROM \"test\" INNER JOIN LATERAL (SELECT \"c\" FROM \"other\") AS \"o\" ON TRUE",
	})
}

func (sds *duckdbDialectSuite) TestCompoundExpressions() {
	ds1 := sds.GetDs("test").Select("a")
	ds2 := sds.GetDs("test2").Select("b")
	sds.assertSQL(
		sqlTestCase{ds: ds1.Union(ds2), sql: "SELECT \"a\" FROM \"test\" UNION SELECT \"b\" FROM \"test2\""},
		sqlTestCase{ds: ds1.UnionAll(ds2), sql: "SELECT \"a\" FROM \"test\" UNION ALL SELECT \"b\" FROM \"test2\""},
		sqlTestCase{ds: ds1.Intersect(ds2), sql: "SELECT \"a\" FROM \"test\" INTERSECT SELECT \"b\" FROM \"test2\""},
	)
}

func (sds *duckdbDialectSuite) TestLiteralString() {
	ds := sds.GetDs("test")
	sds.assertSQL(
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq("test")), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq("test'test")), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test''test')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq(`test"test`)), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test\"test')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq(`test\test`)), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test\\test')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq("test\ntest")), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test\ntest')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq("test\rtest")), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test\rtest')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq("test\x00test")), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test\x00test')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq("test\x1atest")), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test\x1atest')"},
	)
}

func (sds *duckdbDialectSuite) TestLiteralBytes() {
	ds := sds.GetDs("test")
	sds.assertSQL(
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq([]byte("test"))), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq([]byte("test'test"))), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test''test')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq([]byte(`test"test`))), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test\"test')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq([]byte(`test\test`))), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test\\test')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq([]byte("test\ntest"))), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test\ntest')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq([]byte("test\rtest"))), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test\rtest')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq([]byte("test\x00test"))), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test\x00test')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq([]byte("test\x1atest"))), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 'test\x1atest')"},
	)
}

func (sds *duckdbDialectSuite) TestInsertWithoutValues() {
	sds.assertSQL(
		sqlTestCase{ds: goqu.Dialect("duckdb").Insert("test"), sql: `INSERT INTO "test" DEFAULT VALUES`},
	)
}

func (sds *duckdbDialectSuite) TestUpdateAndDeleteWithOrderAndLimit() {
	ds := sds.GetDs("test").Where(goqu.C("a").Gt(1)).Order(goqu.C("a").Asc()).Limit(1)
	sds.assertSQL(
		sqlTestCase{ds: ds.Update().Set(goqu.Record{"a": 2}), sql: `UPDATE "test" SET "a"=2 WHERE ("a" > 1) ORDER BY "a" ASC LIMIT 1`},
		sqlTestCase{ds: ds.Delete(), err: "goqu: dialect does not support ORDER BY on DELETE [dialect=duckdb]"},
		sqlTestCase{ds: sds.GetDs("test").Where(goqu.C("a").Gt(1)).Limit(1).Delete(), sql: `DELETE FROM "test" WHERE ("a" > 1) LIMIT 1`},
	)
}

func (sds *duckdbDialectSuite) TestBooleanOperations() {
	ds := sds.GetDs("test")
	sds.assertSQL(
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq(true)), sql: "SELECT * FROM \"test\" WHERE (\"a\" IS TRUE)"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Eq(false)), sql: "SELECT * FROM \"test\" WHERE (\"a\" IS FALSE)"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Is(true)), sql: "SELECT * FROM \"test\" WHERE (\"a\" IS TRUE)"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Is(false)), sql: "SELECT * FROM \"test\" WHERE (\"a\" IS FALSE)"},
		sqlTestCase{ds: ds.Prepared(true).Where(goqu.C("a").IsTrue()), sql: "SELECT * FROM \"test\" WHERE (\"a\" IS TRUE)", isPrepared: true},
		sqlTestCase{ds: ds.Prepared(true).Where(goqu.C("a").IsFalse()), sql: "SELECT * FROM \"test\" WHERE (\"a\" IS FALSE)", isPrepared: true},
		sqlTestCase{ds: ds.Where(goqu.C("a").IsTrue()), sql: "SELECT * FROM \"test\" WHERE (\"a\" IS TRUE)"},
		sqlTestCase{ds: ds.Where(goqu.C("a").IsFalse()), sql: "SELECT * FROM \"test\" WHERE (\"a\" IS FALSE)"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Neq(true)), sql: "SELECT * FROM \"test\" WHERE (\"a\" IS NOT TRUE)"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Neq(false)), sql: "SELECT * FROM \"test\" WHERE (\"a\" IS NOT FALSE)"},
		sqlTestCase{ds: ds.Where(goqu.C("a").IsNot(true)), sql: "SELECT * FROM \"test\" WHERE (\"a\" IS NOT TRUE)"},
		sqlTestCase{ds: ds.Where(goqu.C("a").IsNot(false)), sql: "SELECT * FROM \"test\" WHERE (\"a\" IS NOT FALSE)"},
		sqlTestCase{ds: ds.Where(goqu.C("a").IsNotTrue()), sql: "SELECT * FROM \"test\" WHERE (\"a\" IS NOT TRUE)"},
		sqlTestCase{ds: ds.Where(goqu.C("a").IsNotFalse()), sql: "SELECT * FROM \"test\" WHERE (\"a\" IS NOT FALSE)"},
		sqlTestCase{ds: ds.Where(goqu.C("a").Like("a%")), sql: "SELECT * FROM \"test\" WHERE (\"a\" LIKE 'a%' ESCAPE '\\')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").NotLike("a%")), sql: "SELECT * FROM \"test\" WHERE (\"a\" NOT LIKE 'a%' ESCAPE '\\')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").ILike("a%")), sql: "SELECT * FROM \"test\" WHERE (\"a\" ILIKE 'a%' ESCAPE '\\')"},
		sqlTestCase{ds: ds.Where(goqu.C("a").NotILike("a%")), sql: "SELECT * FROM \"test\" WHERE (\"a\" NOT ILIKE 'a%' ESCAPE '\\')"},
		sqlTestCase{
			ds:  ds.Where(goqu.C("a").Like(regexp.MustCompile("[ab]"))),
			err: "goqu: dialect does not support boolean operator \"regexplike\" [dialect=duckdb]",
		},
		sqlTestCase{
			ds:  ds.Where(goqu.C("a").NotLike(regexp.MustCompile("[ab]"))),
			err: "goqu: dialect does not support boolean operator \"regexpnotlike\" [dialect=duckdb]",
		},
		sqlTestCase{
			ds:  ds.Where(goqu.C("a").ILike(regexp.MustCompile("[ab]"))),
			err: "goqu: dialect does not support boolean operator \"regexpilike\" [dialect=duckdb]",
		},
		sqlTestCase{
			ds:  ds.Where(goqu.C("a").NotILike(regexp.MustCompile("[ab]"))),
			err: "goqu: dialect does not support boolean operator \"regexpnotilike\" [dialect=duckdb]",
		},
	)
}

func (sds *duckdbDialectSuite) TestBitwiseOperations() {
	col := goqu.C("a")
	ds := sds.GetDs("test")
	sds.assertSQL(
		sqlTestCase{ds: ds.Where(col.BitwiseInversion()), sql: "SELECT * FROM \"test\" WHERE (~ \"a\")"},
		sqlTestCase{ds: ds.Where(col.BitwiseAnd(1)), sql: "SELECT * FROM \"test\" WHERE (\"a\" & 1)"},
		sqlTestCase{ds: ds.Where(col.BitwiseOr(1)), sql: "SELECT * FROM \"test\" WHERE (\"a\" | 1)"},
		sqlTestCase{ds: ds.Where(col.BitwiseXor(1)), err: "goqu: dialect does not support bitwise operator \"XOR\" [dialect=duckdb]"},
		sqlTestCase{ds: ds.Where(col.BitwiseLeftShift(1)), sql: "SELECT * FROM \"test\" WHERE (\"a\" << 1)"},
		sqlTestCase{ds: ds.Where(col.BitwiseRightShift(1)), sql: "SELECT * FROM \"test\" WHERE (\"a\" >> 1)"},
	)
}

func (sds *duckdbDialectSuite) TestFuns() {
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

func (sds *duckdbDialectSuite) TestTruncate() {
	ds := goqu.Dialect("duckdb").From("test")
	sds.assertSQL(
		sqlTestCase{ds: ds.Truncate(), sql: "TRUNCATE \"test\""},
		sqlTestCase{ds: ds.Truncate().Identity("restart"), err: "goqu: dialect does not support RESTART IDENTITY in TRUNCATE [dialect=duckdb]"},
		sqlTestCase{
			ds:  goqu.Dialect("duckdb").Truncate("a", "b"),
			err: "goqu: dialect does not support multiple tables in TRUNCATE [dialect=duckdb]",
		},
		sqlTestCase{ds: ds.Truncate().Cascade(), sql: "TRUNCATE \"test\" CASCADE"},
		sqlTestCase{ds: ds.Truncate().Restrict(), sql: "TRUNCATE \"test\" RESTRICT"},
	)
}

func (sds *duckdbDialectSuite) TestInsertRowAlias() {
	sds.assertSQL(
		sqlTestCase{
			ds:  goqu.Dialect("duckdb").Insert("test").Rows(goqu.Record{"a": 1}).As("new"),
			err: "goqu: dialect does not support an INSERT row alias [dialect=duckdb]",
		},
	)
}

func (sds *duckdbDialectSuite) TestForUpdate() {
	ds := sds.GetDs("test").Where(goqu.C("a").Eq(1))
	sds.assertSQL(
		sqlTestCase{ds: ds.ForUpdate(goqu.Wait), sql: "SELECT * FROM \"test\" WHERE (\"a\" = 1)"},
		sqlTestCase{ds: ds.ForUpdate(goqu.NoWait), err: "goqu: dialect does not support NOWAIT [dialect=duckdb]"},
		sqlTestCase{ds: ds.ForUpdate(goqu.SkipLocked), err: "goqu: dialect does not support SKIP LOCKED [dialect=duckdb]"},
		sqlTestCase{ds: ds.ForUpdate(goqu.Wait, goqu.T("test")), err: "goqu: dialect does not support OF in a locking clause [dialect=duckdb]"},
	)
}

func (sds *duckdbDialectSuite) TestUnsupportedLockStrengthsAreErrors() {
	ds := sds.GetDs("test")
	sds.assertSQL(
		sqlTestCase{ds: ds.ForNoKeyUpdate(goqu.Wait), err: "goqu: dialect does not support FOR NO KEY UPDATE [dialect=duckdb]"},
		sqlTestCase{ds: ds.ForShare(goqu.Wait), err: "goqu: dialect does not support FOR SHARE [dialect=duckdb]"},
		sqlTestCase{ds: ds.ForKeyShare(goqu.SkipLocked), err: "goqu: dialect does not support FOR KEY SHARE [dialect=duckdb]"},
	)
}

func (sds *duckdbDialectSuite) TestEmptyInList() {
	ds := sds.GetDs("test").Where(goqu.C("a").In([]int{}), goqu.C("b").NotIn([]int{}))
	sds.assertSQL(
		sqlTestCase{ds: ds, sql: "SELECT * FROM \"test\" WHERE ((1=0) AND (1=1))"},
		sqlTestCase{ds: ds.Prepared(true), sql: "SELECT * FROM \"test\" WHERE ((1=0) AND (1=1))", isPrepared: true},
	)
}

func TestDatasetAdapterSuite(t *testing.T) {
	suite.Run(t, new(duckdbDialectSuite))
}
