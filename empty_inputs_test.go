package goqu_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/stretchr/testify/assert"
)

func TestEmptyClauseInputsAreErrors(t *testing.T) {
	var noTables []interface{}
	var noWindows []exp.WindowExpression
	var noLockTables []exp.IdentifierExpression
	for name, c := range map[string]struct {
		ds  interface{ ToSQL() (string, []any, error) }
		err string
	}{
		"select From":     {goqu.From("t").From(noTables...), "goqu: From requires at least one table; use ClearFrom to drop it"},
		"select From nil": {goqu.From("t").From(nil), "goqu: From requires at least one table; use ClearFrom to drop it"},
		"update From": {
			goqu.Update("t").Set(goqu.Record{"a": 1}).From(noTables...),
			"goqu: From requires at least one table; use ClearFrom to drop it",
		},
		"Truncate": {goqu.Truncate(noTables...), "goqu: Truncate requires at least one table"},
		"Table":    {goqu.Truncate("t").Table(noTables...), "goqu: Truncate requires at least one table"},
		"Window": {
			goqu.From("t").Window(noWindows...),
			"goqu: Window requires at least one window; use ClearWindow to drop it",
		},
		"ForUpdateOf": {
			goqu.From("t").ForUpdateOf(goqu.Wait, noLockTables...),
			"goqu: ForUpdateOf requires at least one table; use ForUpdate without Of to lock every table",
		},
		"ForNoKeyUpdateOf": {
			goqu.From("t").ForNoKeyUpdateOf(goqu.Wait),
			"goqu: ForNoKeyUpdateOf requires at least one table; use ForNoKeyUpdate without Of to lock every table",
		},
		"ForShareOf": {
			goqu.From("t").ForShareOf(goqu.Wait),
			"goqu: ForShareOf requires at least one table; use ForShare without Of to lock every table",
		},
		"ForKeyShareOf": {
			goqu.From("t").ForKeyShareOf(goqu.Wait),
			"goqu: ForKeyShareOf requires at least one table; use ForKeyShare without Of to lock every table",
		},
		"insert Returning": {
			goqu.Insert("t").Returning(),
			"goqu: Returning requires at least one column; use ClearReturning to drop it",
		},
		"select With":          {goqu.From("t").With("", goqu.From("u")), "goqu: With requires a name"},
		"select WithRecursive": {goqu.From("t").WithRecursive(" ", goqu.From("u")), "goqu: With requires a name"},
		"insert With":          {goqu.Insert("t").With("", goqu.From("u")).Rows(goqu.Record{"a": 1}), "goqu: With requires a name"},
		"update With":          {goqu.Update("t").With("", goqu.From("u")).Set(goqu.Record{"a": 1}), "goqu: With requires a name"},
		"delete WithRecursive": {goqu.Delete("t").WithRecursive("", goqu.From("u")), "goqu: With requires a name"},
	} {
		_, _, err := c.ds.ToSQL()
		assert.EqualError(t, err, c.err, name)
		assert.True(t, goqu.IsInvalidExpression(err), name)
	}
}

func TestClearMethodsDropTheirClause(t *testing.T) {
	sub := goqu.From("s")
	assertSQLCases(t,
		sqlCase{ds: goqu.From("t").ForUpdate(goqu.Wait), sql: `SELECT * FROM "t" FOR UPDATE `},
		sqlCase{ds: goqu.From("t").ForUpdate(goqu.Wait).ClearLock(), sql: `SELECT * FROM "t"`},
		sqlCase{ds: goqu.From("t").Join(goqu.T("u"), goqu.On(goqu.I("t.id").Eq(goqu.I("u.id")))).ClearJoins(), sql: `SELECT * FROM "t"`},
		sqlCase{ds: goqu.From("t").With("c", sub).ClearWith(), sql: `SELECT * FROM "t"`},
		sqlCase{ds: goqu.Insert("t").With("c", sub).Rows(goqu.Record{"a": 1}).ClearWith(), sql: `INSERT INTO "t" ("a") VALUES (1)`},
		sqlCase{ds: goqu.Update("t").With("c", sub).Set(goqu.Record{"a": 1}).ClearWith(), sql: `UPDATE "t" SET "a"=1`},
		sqlCase{ds: goqu.Delete("t").With("c", sub).ClearWith(), sql: `DELETE FROM "t"`},
		sqlCase{ds: goqu.Update("t").Set(goqu.Record{"a": 1}).From("u").ClearFrom(), sql: `UPDATE "t" SET "a"=1`},
		sqlCase{ds: goqu.Update("t").Set(goqu.Record{"a": 1}).Returning("a").ClearReturning(), sql: `UPDATE "t" SET "a"=1`},
		sqlCase{ds: goqu.Delete("t").Returning("a").ClearReturning(), sql: `DELETE FROM "t"`},
		sqlCase{ds: goqu.From("t").Select("a").Distinct().ClearSelect(), sql: `SELECT DISTINCT * FROM "t"`},
		sqlCase{ds: goqu.From("t").DistinctOn("a").ClearDistinct(), sql: `SELECT * FROM "t"`},
		sqlCase{ds: goqu.From("t").Window(goqu.W("w")).ClearWindow(), sql: `SELECT * FROM "t"`},
	)
	d := goqu.Dialect("mysql8")
	assertSQLCases(t, sqlCase{ds: d.Insert("t").Rows(goqu.Record{"a": 1}).As("new").ClearAlias(), sql: "INSERT INTO `t` (`a`) VALUES (1)"})
}
