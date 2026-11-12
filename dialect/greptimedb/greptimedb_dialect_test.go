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
		sqlTestCase{
			ds:  ds.Select(goqu.C("a").BitwiseInversion()),
			err: `goqu: dialect does not support bitwise operator "Inversion" [dialect=greptimedb]`,
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
			ds:  ds.Join(goqu.Lateral(dialect.From("n").Where(goqu.I("n.host").Eq(goqu.I("m.host")))).As("x"), goqu.On(goqu.L("true"))),
			err: "goqu: dialect does not support lateral expressions [dialect=greptimedb]",
		},
		sqlTestCase{
			ds:  ds.Select(goqu.JSONExtract("doc", "$.a")),
			err: "goqu: dialect does not support JSONExtract [dialect=greptimedb]",
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
			ds:         ds.Prepared(true).Select(bucket).Where(goqu.C("ts").Gt(goqu.UTCTimestamp().Sub(goqu.Interval(1.5, exp.SecondInterval)))),
			sql:        "SELECT date_bin(INTERVAL 5 MINUTE, `ts`) FROM `m` WHERE (`ts` > (NOW() - INTERVAL 1.5 SECOND))",
			isPrepared: true,
		},
		sqlTestCase{ds: ds.Select(goqu.Now(3)), err: "goqu: dialect does not support fsp [dialect=greptimedb]"},
		sqlTestCase{
			ds:  ds.Select(goqu.Interval(1, exp.QuarterInterval)),
			err: "goqu: dialect does not support Interval unit QUARTER [dialect=greptimedb]",
		},
	)
}

func (gds *greptimedbDialectSuite) TestLocking() {
	ds := dialect.From("m")
	gds.assertSQL(
		sqlTestCase{ds: ds.ForUpdate(goqu.Wait), err: "goqu: dialect does not support FOR UPDATE [dialect=greptimedb]"},
		sqlTestCase{ds: ds.ForShare(goqu.Wait), err: "goqu: dialect does not support FOR SHARE [dialect=greptimedb]"},
		sqlTestCase{ds: ds.ForNoKeyUpdate(goqu.Wait), err: "goqu: dialect does not support FOR NO KEY UPDATE [dialect=greptimedb]"},
		sqlTestCase{ds: ds.ForKeyShare(goqu.Wait), err: "goqu: dialect does not support FOR KEY SHARE [dialect=greptimedb]"},
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
		sqlTestCase{ds: ds.Rows(rows[0]).OnConflict(goqu.DoNothing()), err: "goqu: dialect does not support upsert [dialect=greptimedb]"},
		sqlTestCase{
			ds:  ds.Rows(rows[0]).OnConflict(goqu.DoUpdate("host", goqu.Record{"cpu": 1})),
			err: "goqu: dialect does not support upsert [dialect=greptimedb]",
		},
		sqlTestCase{ds: ds.Rows(rows[0]).Returning("host"), err: "goqu: dialect does not support RETURNING clause [dialect=greptimedb]"},
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
		sqlTestCase{ds: ds.Order(goqu.C("ts").Asc()), err: "goqu: dialect does not support ORDER BY on DELETE [dialect=greptimedb]"},
		sqlTestCase{ds: ds.Limit(1), err: "goqu: dialect does not support LIMIT on DELETE [dialect=greptimedb]"},
		sqlTestCase{ds: ds.Returning("host"), err: "goqu: dialect does not support RETURNING clause [dialect=greptimedb]"},
	)
}

func (gds *greptimedbDialectSuite) TestTruncate() {
	ds := dialect.Truncate("m")
	gds.assertSQL(
		sqlTestCase{ds: ds, sql: "TRUNCATE TABLE `m`"},
		sqlTestCase{ds: dialect.Truncate("m", "n"), err: "goqu: dialect does not support multiple tables in TRUNCATE [dialect=greptimedb]"},
		sqlTestCase{ds: ds.Cascade(), err: "goqu: dialect does not support CASCADE in TRUNCATE [dialect=greptimedb]"},
		sqlTestCase{ds: ds.Restrict(), err: "goqu: dialect does not support RESTRICT in TRUNCATE [dialect=greptimedb]"},
		sqlTestCase{ds: ds.Identity("RESTART"), err: "goqu: dialect does not support RESTART IDENTITY in TRUNCATE [dialect=greptimedb]"},
	)
}

func (gds *greptimedbDialectSuite) TestUpdate() {
	gds.assertSQL(
		sqlTestCase{
			ds:  dialect.Update("m").Set(goqu.Record{"cpu": 1}),
			err: "goqu: dialect does not support UPDATE statements [dialect=greptimedb]",
		},
	)
}

func TestGreptimeDBDialectSuite(t *testing.T) {
	suite.Run(t, new(greptimedbDialectSuite))
}
