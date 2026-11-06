package postgres_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/sqlgen"
	"github.com/stretchr/testify/assert"
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
	assert.True(t, sqlgen.IsInvalidExpression(err))
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
		{update.Order(goqu.C("a").Asc()), "goqu: dialect does not support ORDER BY on UPDATE [dialect=postgres]"},
		{del.Limit(1), "goqu: dialect does not support LIMIT on DELETE [dialect=postgres]"},
		{del.Order(goqu.C("a").Asc()), "goqu: dialect does not support ORDER BY on DELETE [dialect=postgres]"},
		{selectToUpdate, "goqu: dialect does not support ORDER BY on UPDATE [dialect=postgres]"},
	} {
		_, _, err := c.sql.ToSQL()
		assert.EqualError(t, err, c.err)
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

func TestEmptyInList(t *testing.T) {
	ds := goqu.Dialect("postgres").From("test").Where(goqu.C("a").In([]int{}), goqu.C("b").NotIn([]int{}))
	for _, prepared := range []bool{false, true} {
		sql, args, err := ds.Prepared(prepared).ToSQL()
		assert.NoError(t, err)
		assert.Equal(t, "SELECT * FROM \"test\" WHERE ((1=0) AND (1=1))", sql)
		assert.Empty(t, args)
	}
}
