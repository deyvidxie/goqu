package goqu_test

import (
	"fmt"
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/sqlgen"
	"github.com/stretchr/testify/assert"
)

func TestIsUnknownDialect(t *testing.T) {
	_, err := goqu.LookupDialect("nope")
	assert.EqualError(t, err, `goqu: dialect "nope" is not registered; blank-import its dialect package or call RegisterDialect`)
	assert.True(t, goqu.IsUnknownDialect(err))
	assert.True(t, goqu.IsUnknownDialect(fmt.Errorf("opening: %w", err)))
	assert.False(t, goqu.IsUnknownDialect(goqu.ErrQueryFactoryNotFoundError))
	assert.False(t, goqu.IsUnknownDialect(nil))
	var target goqu.UnknownDialectError
	assert.ErrorAs(t, err, &target)
}

func TestDatasetErrorsAreInvalidExpressions(t *testing.T) {
	type row struct{ A int }
	toSQL := func(ds interface{ ToSQL() (string, []any, error) }) error {
		_, _, err := ds.ToSQL()
		return err
	}
	for _, c := range []struct {
		name string
		want string
		err  error
	}{
		{"ErrBadFromArgument", "unsupported DeleteDataset#From argument", goqu.ErrBadFromArgument},
		{"ErrUnsupportedIntoType", "unsupported table type", goqu.ErrUnsupportedIntoType},
		{"ErrUnsupportedUpdateTableType", "unsupported table type", goqu.ErrUnsupportedUpdateTableType},
		{"ErrQueryFactoryNotFoundError", "unable to execute query", goqu.ErrQueryFactoryNotFoundError},
		{
			"rows of different lengths", "rows with different value length expected 1 got 2",
			toSQL(goqu.Insert("t").Rows(goqu.Record{"a": 1}, goqu.Record{"a": 1, "b": 2})),
		},
		{
			"rows with different keys", "rows with different keys",
			toSQL(goqu.Insert("t").Rows(goqu.Record{"a": 1}, goqu.Record{"b": 1})),
		},
		{
			"rows of different types", "rows must be all the same type",
			toSQL(goqu.Insert("t").Rows(goqu.Record{"a": 1}, row{A: 1})),
		},
		{
			"struct rows of different types", "rows must be all the same type",
			toSQL(goqu.Insert("t").Rows(row{A: 1}, goqu.Record{"a": 1})),
		},
		{"unsupported row type", "unsupported insert must be map", toSQL(goqu.Insert("t").Rows(1))},
		{"unsupported update value", "unsupported update interface type int", toSQL(goqu.Update("t").Set(1))},
		{
			"unknown Op key", "unsupported expression type bogus",
			toSQL(goqu.From("t").Where(goqu.Ex{"a": goqu.Op{"bogus": 1}})),
		},
	} {
		assert.ErrorContains(t, c.err, c.want, c.name)
		assert.True(t, sqlgen.IsInvalidExpression(c.err), "%s: %v", c.name, c.err)
	}
}
