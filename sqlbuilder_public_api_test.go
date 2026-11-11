package goqu_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exec"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/sqlbuilder"
	"github.com/doug-martin/goqu/v9/sqlgen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type literalSubquery struct{ sql string }

func (l literalSubquery) Clone() exp.Expression             { return l }
func (l literalSubquery) Expression() exp.Expression        { return l }
func (l literalSubquery) AppendSQL(b sqlbuilder.SQLBuilder) { b.WriteStrings(l.sql) }
func (l literalSubquery) GetAs() exp.IdentifierExpression   { return nil }
func (l literalSubquery) ReturnsColumns() bool              { return true }

type commentingDialect struct{ goqu.SQLDialect }

func (d commentingDialect) ToSelectSQL(b sqlbuilder.SQLBuilder, clauses exp.SelectClauses) {
	b.WriteStrings("/* custom */ ")
	d.SQLDialect.ToSelectSQL(b, clauses)
}

func TestSQLBuilderAPIsNeedOnlyPublicPackages(t *testing.T) {
	sql, _, err := goqu.From("t").Where(goqu.C("id").In(literalSubquery{sql: "SELECT 1"})).ToSQL()
	require.NoError(t, err)
	assert.Equal(t, `SELECT * FROM "t" WHERE ("id" IN (SELECT 1))`, sql)

	base, err := goqu.GetDialect("default")
	require.NoError(t, err)
	sql, _, err = goqu.From("t").SetDialect(commentingDialect{base}).ToSQL()
	require.NoError(t, err)
	assert.Equal(t, `/* custom */ SELECT * FROM "t"`, sql)

	b := sqlbuilder.New(true)
	goqu.From("t").Where(goqu.C("a").Eq(1)).AppendSQL(b)
	sql, args, err := b.ToSQL()
	require.NoError(t, err)
	assert.Equal(t, `SELECT * FROM "t" WHERE ("a" = ?)`, sql)
	assert.Equal(t, []interface{}{int64(1)}, args)

	b = sqlbuilder.New(false)
	sqlgen.NewExpressionSQLGenerator("default", "db", sqlgen.DefaultDialectOptions()).Generate(b, goqu.C("a").Eq(1))
	sql, _, err = b.ToSQL()
	require.NoError(t, err)
	assert.Equal(t, `("a" = 1)`, sql)

	errored := sqlbuilder.New(false)
	goqu.Dialect("not-registered").From("t").AppendSQL(errored)
	_, err = exec.NewQueryFactory("db", nil).FromSQLBuilder(errored).Exec()
	assert.True(t, goqu.IsUnknownDialect(err))
}
