package greptimedb_test

import (
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
)

var pgDialect = goqu.Dialect("greptimedb-pg")

func (gds *greptimedbDialectSuite) TestPGQuotingAndPlaceholders() {
	ds := pgDialect.From("m")
	ts := time.Date(2024, 1, 2, 3, 4, 5, 123456789, time.UTC)
	gds.assertSQL(
		sqlTestCase{ds: ds.Select("host"), sql: `SELECT "host" FROM "m"`},
		sqlTestCase{ds: ds.Where(goqu.C("host").Eq(`it's \ x`)), sql: `SELECT * FROM "m" WHERE ("host" = 'it''s \ x')`},
		sqlTestCase{ds: ds.Where(goqu.C("ok").Eq(true)), sql: `SELECT * FROM "m" WHERE ("ok" IS TRUE)`},
		sqlTestCase{ds: ds.Where(goqu.C("ts").Gte(ts)), sql: `SELECT * FROM "m" WHERE ("ts" >= '2024-01-02T03:04:05.123456789Z')`},
		sqlTestCase{ds: ds.Select(goqu.C("host").Collate("C")), sql: `SELECT "host" COLLATE "C" FROM "m"`},
		sqlTestCase{
			ds: ds.Prepared(true).
				Where(goqu.C("host").Eq("a"), goqu.C("ts").Gt(goqu.Now().Sub(goqu.Interval(2, exp.DayInterval))), goqu.C("cpu").Gt(1)).
				Order(goqu.C("host").Asc()).Limit(10).Offset(1),
			sql: `SELECT * FROM "m" WHERE (("host" = $1) AND ("ts" > (NOW() - INTERVAL 2 DAY)) AND ("cpu" > $2)) ` +
				`ORDER BY "host" ASC LIMIT $3 OFFSET $4`,
			isPrepared: true,
			args:       []interface{}{"a", int64(1), int64(10), int64(1)},
		},
		sqlTestCase{ds: ds.Offset(1), sql: `SELECT * FROM "m" OFFSET 1`},
	)
}

func (gds *greptimedbDialectSuite) TestPGOperators() {
	ds := pgDialect.From("m")
	gds.assertSQL(
		sqlTestCase{
			ds:  ds.Select(goqu.C("a").BitwiseXor(1), goqu.C("a").BitwiseOr(1)),
			sql: `SELECT ("a" # 1), ("a" | 1) FROM "m"`,
		},
		sqlTestCase{
			ds:  ds.Where(goqu.C("a").RegexpLike("^x"), goqu.C("b").ILike("x%")),
			sql: `SELECT * FROM "m" WHERE (("a" ~ '^x') AND ("b" ILIKE 'x%'))`,
		},
	)
}

func (gds *greptimedbDialectSuite) TestPGSelectFeatures() {
	ds := pgDialect.From("m")
	w := goqu.W().PartitionBy("host").OrderBy(goqu.C("ts").Desc())
	gds.assertSQL(
		sqlTestCase{
			ds: ds.Select("host").Window(goqu.W("w").PartitionBy("host")).Qualify(goqu.ROW_NUMBER().Over(w).Eq(1)),
			sql: `SELECT "host" FROM "m" WINDOW "w" AS (PARTITION BY "host") ` +
				`QUALIFY (ROW_NUMBER() OVER (PARTITION BY "host" ORDER BY "ts" DESC) = 1)`,
		},
		sqlTestCase{
			ds:  ds.Select("host", "ts").DistinctOn("host").Order(goqu.C("host").Asc(), goqu.C("ts").Desc()),
			sql: `SELECT DISTINCT ON ("host") "host", "ts" FROM "m" ORDER BY "host" ASC, "ts" DESC`,
		},
	)
}

func (gds *greptimedbDialectSuite) TestPGStatements() {
	row := goqu.Record{"host": "a", "cpu": 1.5}
	gds.assertSQL(
		sqlTestCase{
			ds:         pgDialect.Insert("m").Prepared(true).Rows(row),
			sql:        `INSERT INTO "m" ("cpu", "host") VALUES ($1, $2)`,
			isPrepared: true,
			args:       []interface{}{1.5, "a"},
		},
		sqlTestCase{
			ds:         pgDialect.Delete("m").Prepared(true).Where(goqu.C("host").Eq("a")),
			sql:        `DELETE FROM "m" WHERE ("host" = $1)`,
			isPrepared: true,
			args:       []interface{}{"a"},
		},
		sqlTestCase{ds: pgDialect.Truncate("m"), sql: `TRUNCATE TABLE "m"`},
	)
}
