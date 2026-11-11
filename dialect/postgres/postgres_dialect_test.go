package postgres_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInsertOnConflictDoNothingTarget(t *testing.T) {
	ds := goqu.Dialect("postgres").Insert("test").
		Rows(goqu.Record{"a": "a"}).
		OnConflict(goqu.DoNothing().SetCols(exp.NewColumnListExpression(nil, "db", "a")))
	sql, args, err := ds.ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, `INSERT INTO "test" ("a") VALUES ('a') ON CONFLICT ("a") DO NOTHING`, sql)
	assert.Empty(t, args)

	sql, args, err = ds.Prepared(true).ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, `INSERT INTO "test" ("a") VALUES ($1) ON CONFLICT ("a") DO NOTHING`, sql)
	assert.Equal(t, []any{"a"}, args)
}

func TestInsertOnConflictDoUpdateExcluded(t *testing.T) {
	ds := goqu.Dialect("postgres").Insert("test").
		Rows(goqu.Record{"a": "a", "b": 1}).
		OnConflict(goqu.DoUpdateExcluded("a", "b").Where(goqu.T("test").Col("b").Lt(goqu.Excluded("b"))))
	sql, args, err := ds.Prepared(true).ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, `INSERT INTO "test" ("a", "b") VALUES ($1, $2) ON CONFLICT (a) DO UPDATE SET "b"=EXCLUDED."b" `+
		`WHERE ("test"."b" < EXCLUDED."b")`, sql)
	assert.Equal(t, []any{"a", int64(1)}, args)

	_, _, err = goqu.Dialect("postgres").Insert("test").
		Rows(goqu.Record{"a": "a"}).
		OnConflict(goqu.DoUpdateExcluded("a", 1)).
		ToSQL()
	assert.EqualError(t, err, "goqu: Excluded requires a column name")
	assert.True(t, goqu.IsInvalidExpression(err))
}

func TestUpdateAndDeleteLimitAndOrderAreErrors(t *testing.T) {
	update := goqu.Dialect("postgres").Update("test").Set(goqu.Record{"a": 1}).Where(goqu.C("a").Gt(1))
	del := goqu.Dialect("postgres").Delete("test").Where(goqu.C("a").Gt(1))
	selectToUpdate := goqu.Dialect("postgres").From("test").Order(goqu.C("a").Asc()).Update().Set(goqu.Record{"a": 1})
	for _, c := range []struct {
		sql interface {
			ToSQL() (string, []interface{}, error)
		}
		err string
	}{
		{update.Limit(1), "goqu: dialect does not support LIMIT on UPDATE [dialect=postgres]"},
		{update.Limit(0), "goqu: dialect does not support LIMIT on UPDATE [dialect=postgres]"},
		{update.Order(goqu.C("a").Asc()), "goqu: dialect does not support ORDER BY on UPDATE [dialect=postgres]"},
		{del.Limit(1), "goqu: dialect does not support LIMIT on DELETE [dialect=postgres]"},
		{del.Limit(0), "goqu: dialect does not support LIMIT on DELETE [dialect=postgres]"},
		{del.Order(goqu.C("a").Asc()), "goqu: dialect does not support ORDER BY on DELETE [dialect=postgres]"},
		{selectToUpdate, "goqu: dialect does not support ORDER BY on UPDATE [dialect=postgres]"},
	} {
		_, _, err := c.sql.ToSQL()
		assert.EqualError(t, err, c.err)
		assert.True(t, goqu.IsUnsupported(err), c.err)
	}
}

func TestUpdateAndDeleteLimitAllIsDropped(t *testing.T) {
	sql, _, err := goqu.Dialect("postgres").Update("test").Set(goqu.Record{"a": 1}).LimitAll().ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, `UPDATE "test" SET "a"=1`, sql)

	sql, _, err = goqu.Dialect("postgres").Delete("test").LimitAll().ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, `DELETE FROM "test"`, sql)
}

func TestTruncateOptions(t *testing.T) {
	sql, _, err := goqu.Dialect("postgres").Truncate("a", "b").Identity("restart").Cascade().ToSQL()
	require.NoError(t, err)
	assert.Equal(t, `TRUNCATE "a", "b" RESTART IDENTITY CASCADE`, sql)
}

func TestInsertRowAlias(t *testing.T) {
	_, _, err := goqu.Dialect("postgres").Insert("test").Rows(goqu.Record{"a": 1}).As("new").ToSQL()
	assert.EqualError(t, err, "goqu: dialect does not support an INSERT row alias [dialect=postgres]")
	assert.True(t, goqu.IsUnsupported(err))
}

func TestEmptyInList(t *testing.T) {
	ds := goqu.Dialect("postgres").From("test").Where(goqu.C("a").In([]int{}), goqu.C("b").NotIn([]int{}))
	for _, prepared := range []bool{false, true} {
		sql, args, err := ds.Prepared(prepared).ToSQL()
		assert.NoError(t, err)
		assert.Equal(t, "SELECT * FROM \"test\" WHERE ((1=0) AND (1=1))", sql)
		assert.Empty(t, args)
	}
}

func TestInsertOnConflictWhereEmptyInOnExcluded(t *testing.T) {
	sql, args, err := goqu.Dialect("postgres").Insert("test").
		Rows(goqu.Record{"a": "a", "b": 1}).
		OnConflict(goqu.DoUpdateExcluded("a", "b").Where(goqu.Excluded("b").In([]int{}))).
		ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, `INSERT INTO "test" ("a", "b") VALUES ('a', 1) ON CONFLICT (a) DO UPDATE SET "b"=EXCLUDED."b" WHERE (1=0)`, sql)
	assert.Empty(t, args)
}

func TestInsertOnConflictEmptyWhere(t *testing.T) {
	ds := goqu.Dialect("postgres").Insert("test").Rows(goqu.Record{"a": 1})
	for name, c := range map[string]struct {
		where []goqu.Expression
		err   string
	}{
		"empty Ex":     {[]goqu.Expression{goqu.Ex{}}, "goqu: empty AND condition (And() or Ex{})"},
		"empty ExOr":   {[]goqu.Expression{goqu.ExOr{}}, "goqu: empty OR condition (Or() or ExOr{})"},
		"no arguments": {nil, "goqu: DoUpdate Where requires at least one expression"},
	} {
		_, _, err := ds.OnConflict(goqu.DoUpdate("a", goqu.Record{"a": 2}).Where(c.where...)).ToSQL()
		assert.EqualError(t, err, c.err, name)
		assert.True(t, goqu.IsInvalidExpression(err), name)
	}
}

func TestJSONFunctionsAreErrors(t *testing.T) {
	_, _, err := goqu.Dialect("postgres").From("t").Where(goqu.JSONValue("doc", "$.a").Eq(1)).ToSQL()
	assert.EqualError(t, err, "goqu: dialect does not support JSONValue [dialect=postgres]")
	assert.True(t, goqu.IsUnsupported(err))
}

func TestTimeFunctions(t *testing.T) {
	ds := goqu.Dialect("postgres").From("t").Select(goqu.Now(6), goqu.UTCTimestamp()).
		Where(goqu.C("at").Lt(goqu.Now().Sub(goqu.Interval(1, "day"))))
	sql, args, err := ds.ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, `SELECT CURRENT_TIMESTAMP(6), (NOW() AT TIME ZONE 'UTC') FROM "t" WHERE ("at" < (NOW() - (1 * INTERVAL '1 DAY')))`, sql)
	assert.Empty(t, args)

	sql, args, err = ds.Prepared(true).ToSQL()
	assert.NoError(t, err)
	assert.Equal(t, `SELECT CURRENT_TIMESTAMP(6), (NOW() AT TIME ZONE 'UTC') FROM "t" WHERE ("at" < (NOW() - ($1 * INTERVAL '1 DAY')))`, sql)
	assert.Equal(t, []any{int64(1)}, args)
}
