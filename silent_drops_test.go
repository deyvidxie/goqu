package goqu_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/stretchr/testify/assert"
)

func TestInsertColsWithRowsIsAnError(t *testing.T) {
	const errColsWithRows = "goqu: Cols cannot be combined with Rows; Rows take their columns from the records"
	for name, ds := range map[string]*goqu.InsertDataset{
		"Cols then Rows": goqu.Insert("t").Cols("a").Rows(goqu.Record{"a": 1, "b": 2}),
		"Rows then Cols": goqu.Insert("t").Rows(goqu.Record{"a": 1}).Cols("a"),
	} {
		_, _, err := ds.ToSQL()
		assert.EqualError(t, err, errColsWithRows, name)
		assert.True(t, goqu.IsInvalidExpression(err), name)
	}
}

func TestSelectConversionsRejectClausesTheyCannotCarry(t *testing.T) {
	base := goqu.From("t").Where(goqu.C("a").Eq(1))
	joined := base.Join(goqu.T("u"), goqu.On(goqu.I("t.id").Eq(goqu.I("u.id"))))
	for name, c := range map[string]struct {
		ds             interface{ ToSQL() (string, []any, error) }
		method, clause string
	}{
		"Truncate with WHERE":      {base.Truncate(), "Truncate", "WHERE"},
		"Truncate with LIMIT":      {goqu.From("t").Limit(5).Truncate(), "Truncate", "LIMIT"},
		"Truncate with ORDER BY":   {goqu.From("t").Order(goqu.C("a").Asc()).Truncate(), "Truncate", "ORDER BY"},
		"Truncate with WITH":       {goqu.From("t").With("c", goqu.From("u")).Truncate(), "Truncate", "WITH"},
		"Update with JOIN":         {joined.Update().Set(goqu.Record{"a": 2}), "Update", "JOIN"},
		"Delete with JOIN":         {joined.Delete(), "Delete", "JOIN"},
		"Delete with OFFSET":       {base.Offset(10).Delete(), "Delete", "OFFSET"},
		"Delete with second table": {goqu.From("t", "u").Delete(), "Delete", "second FROM table"},
		"Update with GROUP BY":     {base.GroupBy("a").Update().Set(goqu.Record{"a": 2}), "Update", "GROUP BY"},
		"Delete with HAVING":       {base.Having(goqu.C("a").Gt(1)).Delete(), "Delete", "HAVING"},
		"Delete with QUALIFY":      {base.Qualify(goqu.C("a").Gt(1)).Delete(), "Delete", "QUALIFY"},
		"Delete with DISTINCT":     {base.Distinct().Delete(), "Delete", "DISTINCT"},
		"Update with DISTINCT ON":  {base.DistinctOn("a").Update().Set(goqu.Record{"a": 2}), "Update", "DISTINCT"},
		"Delete with WINDOW":       {base.Window(goqu.W("w")).Delete(), "Delete", "WINDOW"},
		"Delete with UNION":        {base.Union(goqu.From("u")).Delete(), "Delete", "compound (UNION/INTERSECT)"},
		"Delete with lock":         {base.ForUpdate(goqu.Wait).Delete(), "Delete", "locking"},
	} {
		_, _, err := c.ds.ToSQL()
		assert.EqualError(t, err, "goqu: SelectDataset."+c.method+" cannot carry the select's "+c.clause+" clause", name)
		assert.True(t, goqu.IsInvalidExpression(err), name)
	}
	assertSQLCases(t,
		sqlCase{ds: goqu.From("t").Truncate(), sql: `TRUNCATE "t"`},
		sqlCase{ds: base.Delete(), sql: `DELETE FROM "t" WHERE ("a" = 1)`},
	)
}

func TestNilListEntriesAreErrors(t *testing.T) {
	const errNil = "goqu: column and condition lists must not contain nil"
	for name, ds := range map[string]interface{ ToSQL() (string, []any, error) }{
		"Select":    goqu.From("t").Select("a", nil),
		"GroupBy":   goqu.From("t").GroupBy(nil),
		"Cols":      goqu.Insert("t").Cols("a", nil).Vals([]interface{}{1, 2}),
		"Returning": goqu.Delete("t").Returning("a", nil),
		"Where":     goqu.From("t").Where(nil),
		"Or":        goqu.From("t").Where(goqu.Or(goqu.C("a").Eq(1), nil)),
		"And":       goqu.From("t").Where(goqu.And(nil)),
		"Having":    goqu.From("t").GroupBy("a").Having(nil),
		"On":        goqu.From("t").Join(goqu.T("u"), goqu.On(nil)),
	} {
		_, _, err := ds.ToSQL()
		assert.EqualError(t, err, errNil, name)
		assert.True(t, goqu.IsInvalidExpression(err), name)
	}
}
