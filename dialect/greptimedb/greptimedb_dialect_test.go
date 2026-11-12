package greptimedb_test

import (
	"testing"
	"time"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/greptimedb"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/stretchr/testify/suite"
)

type (
	greptimedbDialectSuite struct {
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

var dialect = goqu.Dialect("greptimedb")

func (gds *greptimedbDialectSuite) assertSQL(cases ...sqlTestCase) {
	for i, c := range cases {
		actualSQL, actualArgs, err := c.ds.ToSQL()
		if c.err == "" {
			gds.NoError(err, "test case %d failed", i)
		} else {
			gds.EqualError(err, c.err, "test case %d failed", i)
		}
		gds.Equal(c.sql, actualSQL, "test case %d failed", i)
		if c.isPrepared && c.args != nil || len(c.args) > 0 {
			gds.Equal(c.args, actualArgs, "test case %d failed", i)
		} else {
			gds.Empty(actualArgs, "test case %d failed", i)
		}
	}
}

func (gds *greptimedbDialectSuite) TestQuotingAndLiterals() {
	ds := dialect.From("m")
	gds.assertSQL(
		sqlTestCase{ds: ds.Select("host"), sql: "SELECT `host` FROM `m`"},
		sqlTestCase{ds: ds.Where(goqu.C("host").Eq("it's \\ \x00 \x1a")), sql: "SELECT * FROM `m` WHERE (`host` = 'it''s \\\\ \\0 \\Z')"},
		sqlTestCase{
			ds:  ds.Where(goqu.C("ok").Eq(true), goqu.C("bad").Eq(false)),
			sql: "SELECT * FROM `m` WHERE ((`ok` IS TRUE) AND (`bad` IS FALSE))",
		},
		sqlTestCase{ds: ds.Select(goqu.V(true), goqu.V(false)), sql: "SELECT TRUE, FALSE FROM `m`"},
		sqlTestCase{
			ds:  ds.Where(goqu.C("ts").Gte(time.Date(2024, 1, 2, 3, 4, 5, 123456789, time.UTC))),
			sql: "SELECT * FROM `m` WHERE (`ts` >= '2024-01-02T03:04:05.123456789Z')",
		},
		sqlTestCase{
			ds:         ds.Prepared(true).Where(goqu.C("host").Eq("a"), goqu.C("cpu").Gt(1)).Limit(10),
			sql:        "SELECT * FROM `m` WHERE ((`host` = ?) AND (`cpu` > ?)) LIMIT ?",
			isPrepared: true,
			args:       []interface{}{"a", int64(1), int64(10)},
		},
	)
}

func (gds *greptimedbDialectSuite) TestLimitOffset() {
	ds := dialect.From("m").Order(goqu.C("ts").Desc())
	gds.assertSQL(
		sqlTestCase{ds: ds.Limit(2).Offset(1), sql: "SELECT * FROM `m` ORDER BY `ts` DESC LIMIT 2 OFFSET 1"},
		sqlTestCase{ds: ds.Offset(1), sql: "SELECT * FROM `m` ORDER BY `ts` DESC OFFSET 1"},
	)
}

func (gds *greptimedbDialectSuite) TestOperators() {
	ds := dialect.From("m")
	gds.assertSQL(
		sqlTestCase{
			ds: ds.Where(
				goqu.C("a").RegexpLike("^x"), goqu.C("b").RegexpNotLike("^x"),
				goqu.C("c").RegexpILike("^x"), goqu.C("d").RegexpNotILike("^x"),
			),
			sql: "SELECT * FROM `m` WHERE ((`a` ~ '^x') AND (`b` !~ '^x') AND (`c` ~* '^x') AND (`d` !~* '^x'))",
		},
		sqlTestCase{
			ds:  ds.Where(goqu.C("a").Like("x%"), goqu.C("b").NotLike("x%"), goqu.C("c").ILike("x%"), goqu.C("d").NotILike("x%")),
			sql: "SELECT * FROM `m` WHERE ((`a` LIKE 'x%') AND (`b` NOT LIKE 'x%') AND (`c` ILIKE 'x%') AND (`d` NOT ILIKE 'x%'))",
		},
		sqlTestCase{
			ds:  ds.Select(goqu.C("a").BitwiseXor(1), goqu.C("a").BitwiseOr(1), goqu.C("a").BitwiseLeftShift(1)),
			sql: "SELECT (`a` ^ 1), (`a` | 1), (`a` << 1) FROM `m`",
		},
	)
}

func (gds *greptimedbDialectSuite) TestSelectFeatures() {
	ds := dialect.From("m")
	w := goqu.W().PartitionBy("host").OrderBy(goqu.C("ts").Desc())
	gds.assertSQL(
		sqlTestCase{
			ds:  ds.Select("host", goqu.ROW_NUMBER().Over(w).As("rn")),
			sql: "SELECT `host`, ROW_NUMBER() OVER (PARTITION BY `host` ORDER BY `ts` DESC) AS `rn` FROM `m`",
		},
		sqlTestCase{
			ds:  ds.Select(goqu.ROW_NUMBER().OverName(goqu.I("w"))).Window(goqu.W("w").PartitionBy("host")),
			sql: "SELECT ROW_NUMBER() OVER `w` FROM `m` WINDOW `w` AS (PARTITION BY `host`)",
		},
		sqlTestCase{
			ds:  ds.Select("host").Qualify(goqu.ROW_NUMBER().Over(w).Eq(1)),
			sql: "SELECT `host` FROM `m` QUALIFY (ROW_NUMBER() OVER (PARTITION BY `host` ORDER BY `ts` DESC) = 1)",
		},
		sqlTestCase{
			ds:  ds.Select("host").Window(goqu.W("w").PartitionBy("host")).Qualify(goqu.C("host").Eq("a")),
			sql: "SELECT `host` FROM `m` WINDOW `w` AS (PARTITION BY `host`) QUALIFY (`host` = 'a')",
		},
		sqlTestCase{
			ds:  ds.Select("host", "ts").DistinctOn("host").Order(goqu.C("host").Asc(), goqu.C("ts").Desc()),
			sql: "SELECT DISTINCT ON (`host`) `host`, `ts` FROM `m` ORDER BY `host` ASC, `ts` DESC",
		},
		sqlTestCase{
			ds:  dialect.From("t").With("t", ds.Select("host")),
			sql: "WITH t AS (SELECT `host` FROM `m`) SELECT * FROM `t`",
		},
		sqlTestCase{
			ds: dialect.From("r").WithRecursive("r",
				dialect.Select(goqu.L("1").As("n")).UnionAll(dialect.From("r").Select(goqu.L("n + 1")).Where(goqu.C("n").Lt(3))),
			),
			sql: "WITH RECURSIVE r AS (SELECT 1 AS `n` UNION ALL (SELECT n + 1 FROM `r` WHERE (`n` < 3))) SELECT * FROM `r`",
		},
		sqlTestCase{
			ds:  ds.Select("host").Intersect(ds.Select("host")),
			sql: "SELECT `host` FROM `m` INTERSECT (SELECT `host` FROM `m`)",
		},
		sqlTestCase{
			ds:  ds.FullJoin(goqu.T("n"), goqu.On(goqu.I("m.host").Eq(goqu.I("n.host")))),
			sql: "SELECT * FROM `m` FULL JOIN `n` ON (`m`.`host` = `n`.`host`)",
		},
	)
}

func (gds *greptimedbDialectSuite) TestTimeFunctions() {
	ds := dialect.From("m")
	bucket := goqu.Func("date_bin", goqu.Interval(5, exp.MinuteInterval), goqu.C("ts"))
	gds.assertSQL(
		sqlTestCase{
			ds:  ds.Select(bucket).Where(goqu.C("ts").Gt(goqu.Now().Sub(goqu.Interval(2, exp.DayInterval)))),
			sql: "SELECT date_bin(INTERVAL 5 MINUTE, `ts`) FROM `m` WHERE (`ts` > (NOW() - INTERVAL 2 DAY))",
		},
		sqlTestCase{
			ds:         ds.Prepared(true).Select(bucket).Where(goqu.C("ts").Gt(goqu.Now().Sub(goqu.Interval(1.5, exp.SecondInterval)))),
			sql:        "SELECT date_bin(INTERVAL 5 MINUTE, `ts`) FROM `m` WHERE (`ts` > (NOW() - INTERVAL 1.5 SECOND))",
			isPrepared: true,
		},
	)
}

func (gds *greptimedbDialectSuite) TestInsert() {
	ts := time.Date(2024, 1, 2, 3, 4, 5, 6, time.UTC)
	ds := dialect.Insert("m")
	rows := []interface{}{goqu.Record{"ts": ts, "host": "a", "cpu": 1.5}, goqu.Record{"ts": ts, "host": "b", "cpu": 2}}
	gds.assertSQL(
		sqlTestCase{
			ds: ds.Rows(rows...),
			sql: "INSERT INTO `m` (`cpu`, `host`, `ts`) VALUES " +
				"(1.5, 'a', '2024-01-02T03:04:05.000000006Z'), (2, 'b', '2024-01-02T03:04:05.000000006Z')",
		},
		sqlTestCase{
			ds:         ds.Prepared(true).Rows(rows[0]),
			sql:        "INSERT INTO `m` (`cpu`, `host`, `ts`) VALUES (?, ?, ?)",
			isPrepared: true,
			args:       []interface{}{1.5, "a", ts},
		},
		sqlTestCase{
			ds:  ds.FromQuery(dialect.From("n").Select("cpu", "host", "ts")).Cols("cpu", "host", "ts"),
			sql: "INSERT INTO `m` (`cpu`, `host`, `ts`) SELECT `cpu`, `host`, `ts` FROM `n`",
		},
	)
}

func (gds *greptimedbDialectSuite) TestDelete() {
	ds := dialect.Delete("m")
	gds.assertSQL(
		sqlTestCase{ds: ds.Where(goqu.C("host").Eq("a")), sql: "DELETE FROM `m` WHERE (`host` = 'a')"},
		sqlTestCase{
			ds:         ds.Prepared(true).Where(goqu.C("host").Eq("a")),
			sql:        "DELETE FROM `m` WHERE (`host` = ?)",
			isPrepared: true,
			args:       []interface{}{"a"},
		},
	)
}

func (gds *greptimedbDialectSuite) TestTruncate() {
	ds := dialect.Truncate("m")
	gds.assertSQL(
		sqlTestCase{ds: ds, sql: "TRUNCATE TABLE `m`"},
	)
}

func (gds *greptimedbDialectSuite) TestRangeQuery() {
	query := func(d goqu.DialectWrapper) *goqu.SelectDataset {
		return d.From("m").Prepared(true).
			Select("ts", goqu.RangeAggregate(goqu.AVG("cpu"), 10*time.Second).Fill(goqu.FillLinear()).As("a")).
			Where(goqu.C("host").Eq("a")).
			Align(goqu.Align(10 * time.Second).To(time.Unix(0, 0)).By("host").Fill(goqu.FillPrev())).
			Order(goqu.C("ts").Asc()).Limit(5)
	}
	gds.assertSQL(
		sqlTestCase{
			ds: query(dialect),
			sql: "SELECT `ts`, (AVG(`cpu`) RANGE '10s' FILL LINEAR) AS `a` FROM `m` WHERE (`host` = ?) " +
				"ALIGN '10s' TO '1970-01-01T00:00:00Z' BY (`host`) FILL PREV ORDER BY `ts` ASC LIMIT ?",
			isPrepared: true,
			args:       []interface{}{"a", int64(5)},
		},
		sqlTestCase{
			ds: query(pgDialect),
			sql: `SELECT "ts", (AVG("cpu") RANGE '10s' FILL LINEAR) AS "a" FROM "m" WHERE ("host" = $1) ` +
				`ALIGN '10s' TO '1970-01-01T00:00:00Z' BY ("host") FILL PREV ORDER BY "ts" ASC LIMIT $2`,
			isPrepared: true,
			args:       []interface{}{"a", int64(5)},
		},
	)
}

func (gds *greptimedbDialectSuite) TestAggregateOrderBy() {
	lastPrice := func() exp.SQLFunctionExpression {
		return goqu.Func("first_value", goqu.C("price")).OrderBy(goqu.C("ts").Desc())
	}
	gds.assertSQL(
		sqlTestCase{
			ds:  dialect.From("m").Select(goqu.RangeAggregate(lastPrice(), 5*time.Minute)).Align(goqu.Align(5 * time.Minute)),
			sql: "SELECT (first_value(`price` ORDER BY `ts` DESC) RANGE '5m') FROM `m` ALIGN '5m'",
		},
		sqlTestCase{
			ds:  pgDialect.From("m").Select(lastPrice()).GroupBy("host"),
			sql: `SELECT first_value("price" ORDER BY "ts" DESC) FROM "m" GROUP BY "host"`,
		},
	)
}

func (gds *greptimedbDialectSuite) TestWithinGroup() {
	median := func() exp.SQLFunctionExpression {
		return goqu.Func("percentile_cont", 0.5).WithinGroup(goqu.C("cpu").Asc())
	}
	gds.assertSQL(
		sqlTestCase{
			ds:  dialect.From("m").Select(goqu.RangeAggregate(median(), 5*time.Minute)).Align(goqu.Align(5 * time.Minute)),
			sql: "SELECT (percentile_cont(0.5) WITHIN GROUP (ORDER BY `cpu` ASC) RANGE '5m') FROM `m` ALIGN '5m'",
		},
		sqlTestCase{
			ds:         pgDialect.From("m").Prepared(true).Select(median()).Where(goqu.C("host").Eq("a")),
			sql:        `SELECT percentile_cont($1) WITHIN GROUP (ORDER BY "cpu" ASC) FROM "m" WHERE ("host" = $2)`,
			isPrepared: true,
			args:       []interface{}{0.5, "a"},
		},
	)
}

func (gds *greptimedbDialectSuite) TestRejectedFeatures() {
	for _, name := range []string{"greptimedb", "greptimedb-pg"} {
		gds.Run(name, func() {
			d := goqu.Dialect(name)
			ds := d.From("m")
			row := goqu.Record{"host": "a"}
			rejected := []struct {
				ds      exp.SQLExpression
				feature string
			}{
				{ds.Select(goqu.C("a").BitwiseInversion()), `bitwise operator "Inversion"`},
				{ds.Join(goqu.Lateral(d.From("n")).As("x"), goqu.On(goqu.L("true"))), "lateral expressions"},
				{ds.Select(goqu.JSONExtract("doc", "$.a")), "JSONExtract"},
				{ds.Select(goqu.Now(3)), "fsp 3"},
				{ds.Select(goqu.UTCTimestamp()), "UTCTimestamp"},
				{ds.ForUpdate(goqu.Wait), "FOR UPDATE"},
				{ds.ForShare(goqu.Wait), "FOR SHARE"},
				{ds.ForNoKeyUpdate(goqu.Wait), "FOR NO KEY UPDATE"},
				{ds.ForKeyShare(goqu.Wait), "FOR KEY SHARE"},
				{d.Insert("m").Rows(row).OnConflict(goqu.DoNothing()), "upsert"},
				{d.Insert("m").Rows(row).OnConflict(goqu.DoUpdate("host", goqu.Record{"cpu": 1})), "upsert"},
				{d.Insert("m").Rows(row).Returning("host"), "RETURNING clause"},
				{d.Delete("m").Order(goqu.C("ts").Asc()), "ORDER BY on DELETE"},
				{d.Delete("m").Limit(1), "LIMIT on DELETE"},
				{d.Delete("m").Returning("host"), "RETURNING clause"},
				{d.Truncate("m", "n"), "multiple tables in TRUNCATE"},
				{d.Truncate("m").Cascade(), "CASCADE in TRUNCATE"},
				{d.Truncate("m").Restrict(), "RESTRICT in TRUNCATE"},
				{d.Truncate("m").Identity("RESTART"), "RESTART IDENTITY in TRUNCATE"},
				{d.Update("m").Set(goqu.Record{"cpu": 1}), "UPDATE statements"},
			}
			cases := make([]sqlTestCase, 0, len(rejected))
			for _, c := range rejected {
				cases = append(cases, sqlTestCase{ds: c.ds, err: "goqu: dialect does not support " + c.feature + " [dialect=" + name + "]"})
			}
			cases = append(cases, sqlTestCase{
				ds:  ds.Select(goqu.Interval(goqu.C("n"), exp.QuarterInterval)),
				err: "goqu: dialect renders Interval unit QUARTER as MONTH, which needs an integer count [dialect=" + name + "]",
			})
			gds.assertSQL(cases...)
		})
	}
}

func TestGreptimeDBDialectSuite(t *testing.T) {
	suite.Run(t, new(greptimedbDialectSuite))
}
