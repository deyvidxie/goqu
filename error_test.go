package goqu_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exec"
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

func TestBuilderInputErrorsAreInvalidExpressions(t *testing.T) {
	type row struct{ A int }
	toSQL := func(ds interface{ ToSQL() (string, []any, error) }) error {
		_, _, err := ds.ToSQL()
		return err
	}
	const tableType = "goqu: unsupported table type, a string or identifier expression is required"
	for _, c := range []struct {
		name string
		want string
		err  error
	}{
		{
			"ErrBadFromArgument", "goqu: unsupported DeleteDataset#From argument, a string or identifier expression is required",
			goqu.ErrBadFromArgument,
		},
		{"ErrUnsupportedIntoType", tableType, goqu.ErrUnsupportedIntoType},
		{"ErrUnsupportedUpdateTableType", tableType, goqu.ErrUnsupportedUpdateTableType},
		{
			"rows of different lengths", "goqu: rows with different value length expected 1 got 2",
			toSQL(goqu.Insert("t").Rows(goqu.Record{"a": 1}, goqu.Record{"a": 1, "b": 2})),
		},
		{
			"rows with different keys", `goqu: rows with different keys expected ["a"] got ["b"]`,
			toSQL(goqu.Insert("t").Rows(goqu.Record{"a": 1}, goqu.Record{"b": 1})),
		},
		{
			"rows of different types", "goqu: rows must be all the same type expected exp.Record got goqu_test.row",
			toSQL(goqu.Insert("t").Rows(goqu.Record{"a": 1}, row{A: 1})),
		},
		{
			"struct rows of different types", "goqu: rows must be all the same type expected goqu_test.row got exp.Record",
			toSQL(goqu.Insert("t").Rows(row{A: 1}, goqu.Record{"a": 1})),
		},
		{
			"unsupported row type", "goqu: unsupported insert must be map, goqu.Record, or struct type got: int",
			toSQL(goqu.Insert("t").Rows(1)),
		},
		{"unsupported update value", "goqu: unsupported update interface type int", toSQL(goqu.Update("t").Set(1))},
		{
			"unknown Op key", "goqu: unsupported expression type bogus",
			toSQL(goqu.From("t").Where(goqu.Ex{"a": goqu.Op{"bogus": 1}})),
		},
		{"nil pointer row", "goqu: insert rows must not be nil, got *goqu_test.row", toSQL(goqu.Insert("t").Rows((*row)(nil)))},
		{"nil row", "goqu: insert rows must not be nil, got <nil>", toSQL(goqu.Insert("t").Rows(row{A: 1}, nil))},
		{
			"nil select dataset row", "goqu: insert rows must not be nil, got *goqu.SelectDataset",
			toSQL(goqu.Insert("t").Rows((*goqu.SelectDataset)(nil))),
		},
		{
			"nil FromQuery source", "goqu: insert source must not be nil, got *goqu.SelectDataset",
			toSQL(goqu.Insert("t").FromQuery((*goqu.SelectDataset)(nil))),
		},
		{"nil pointer update", "goqu: update values must not be nil, got *goqu_test.row", toSQL(goqu.Update("t").Set((*row)(nil)))},
	} {
		assert.EqualError(t, c.err, c.want, c.name)
		assert.True(t, sqlgen.IsInvalidExpression(c.err), "%s: %v", c.name, c.err)
	}
}

func TestIsNoDatabase(t *testing.T) {
	assert.True(t, goqu.IsNoDatabase(fmt.Errorf("x: %w", goqu.ErrQueryFactoryNotFoundError)))
	_, dialectErr := goqu.LookupDialect("nope")
	assert.False(t, goqu.IsNoDatabase(dialectErr))
	assert.False(t, goqu.IsNoDatabase(sqlgen.NewInvalidExpressionError("x")))
	assert.False(t, goqu.IsNoDatabase(nil))
}

func TestQueryFactoryNotFoundIsNoDatabase(t *testing.T) {
	_, err := goqu.From("t").Count()
	assert.EqualError(t, err, "goqu: unable to execute query did you use goqu.Database#From to create the dataset")
	assert.ErrorIs(t, err, goqu.ErrQueryFactoryNotFoundError)
	assert.True(t, goqu.IsNoDatabase(err))
	var target goqu.NoDatabaseError
	assert.ErrorAs(t, err, &target)
	assert.False(t, sqlgen.IsInvalidExpression(err))
}

func TestExecutorWithoutDatabaseFailsWithNoDatabase(t *testing.T) {
	for name, executor := range map[string]func() (any, error){
		"select":   func() (any, error) { return goqu.From("t").Executor().Exec() },
		"insert":   func() (any, error) { return goqu.Insert("t").Rows(goqu.Record{"a": 1}).Executor().Exec() },
		"update":   func() (any, error) { return goqu.Update("t").Set(goqu.Record{"a": 1}).Executor().Exec() },
		"delete":   func() (any, error) { return goqu.Delete("t").Executor().Exec() },
		"truncate": func() (any, error) { return goqu.Truncate("t").Executor().Exec() },
	} {
		_, err := executor()
		assert.ErrorIs(t, err, goqu.ErrQueryFactoryNotFoundError, name)
		assert.True(t, goqu.IsNoDatabase(err), name)
	}
}

type SkippedScalar int

func TestExecutorWithoutDatabaseKeepsTheDatasetTag(t *testing.T) {
	type row struct {
		A              int `goqu:"a"`
		*SkippedScalar `goqu:"-"`
	}
	dw := goqu.DialectWithTagName("default", "goqu")
	for name, executor := range map[string]exec.QueryExecutor{
		"select":   dw.From("t").Executor(),
		"insert":   dw.Insert("t").Rows(goqu.Record{"a": 1}).Executor(),
		"update":   dw.Update("t").Set(goqu.Record{"a": 1}).Executor(),
		"delete":   dw.Delete("t").Executor(),
		"truncate": dw.Truncate("t").Executor(),
	} {
		_, err := executor.ScanAllStructs[row](context.Background())
		assert.ErrorIs(t, err, goqu.ErrQueryFactoryNotFoundError, name)
	}
}
