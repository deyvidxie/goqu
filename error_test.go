package goqu_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exec"
	"github.com/doug-martin/goqu/v9/internal/errors"
	"github.com/stretchr/testify/assert"
)

func TestErrorKinds(t *testing.T) {
	is := map[string]func(error) bool{
		"InvalidExpression":   goqu.IsInvalidExpression,
		"Unsupported":         goqu.IsUnsupported,
		"UnsupportedScanType": goqu.IsUnsupportedScanType,
		"ColumnMismatch":      goqu.IsColumnMismatch,
		"UnknownDialect":      goqu.IsUnknownDialect,
		"NoDatabase":          goqu.IsNoDatabase,
	}
	kinds := map[string]error{
		"InvalidExpression":   errors.NewInvalidExpressionError("bad %s", "x"),
		"Unsupported":         errors.NewUnsupportedError("bad %s", "x"),
		"UnsupportedScanType": errors.NewUnsupportedScanTypeError("bad %s", "x"),
		"ColumnMismatch":      errors.NewColumnMismatchError("bad %s", "x"),
		"UnknownDialect":      errors.NewUnknownDialectError("bad %s", "x"),
		"NoDatabase":          errors.NewNoDatabaseError("bad %s", "x"),
	}
	for kind, err := range kinds {
		assert.EqualError(t, err, "goqu: bad x", kind)
		for other, isOther := range is {
			assert.Equal(t, kind == other, isOther(err), "%s is %s", kind, other)
			assert.Equal(t, kind == other, isOther(fmt.Errorf("wrapped: %w", err)), "wrapped %s is %s", kind, other)
		}
	}
	for name, isKind := range is {
		assert.False(t, isKind(nil), name)
		assert.False(t, isKind(newTestError("x")), name)
	}

	var invalidExpression goqu.InvalidExpressionError
	assert.ErrorAs(t, kinds["InvalidExpression"], &invalidExpression)
	var unsupported goqu.UnsupportedError
	assert.ErrorAs(t, kinds["Unsupported"], &unsupported)
	var unsupportedScanType goqu.UnsupportedScanTypeError
	assert.ErrorAs(t, kinds["UnsupportedScanType"], &unsupportedScanType)
	var columnMismatch goqu.ColumnMismatchError
	assert.ErrorAs(t, kinds["ColumnMismatch"], &columnMismatch)
	var unknownDialect goqu.UnknownDialectError
	assert.ErrorAs(t, kinds["UnknownDialect"], &unknownDialect)
	var noDatabase goqu.NoDatabaseError
	assert.ErrorAs(t, kinds["NoDatabase"], &noDatabase)
}

func TestInvalidExpressionErrorWithCause(t *testing.T) {
	cause := newTestError("cause")
	err := errors.NewInvalidExpressionErrorWithCause(cause, "bad %s", "x")
	assert.EqualError(t, err, "goqu: bad x: cause")
	assert.True(t, goqu.IsInvalidExpression(err))
	assert.ErrorIs(t, err, cause)
	assert.EqualError(t, errors.NewInvalidExpressionErrorWithCause(nil, "bad %s", "x"), "goqu: bad x")
}

func TestLookupDialectUnknown(t *testing.T) {
	_, err := goqu.LookupDialect("nope")
	assert.EqualError(t, err, `goqu: dialect "nope" is not registered; blank-import its dialect package or call RegisterDialect`)
	assert.True(t, goqu.IsUnknownDialect(err))
}

func TestBuilderInputErrorsAreInvalidExpressions(t *testing.T) {
	type row struct{ A int }
	toSQL := func(ds interface{ ToSQL() (string, []any, error) }) error {
		_, _, err := ds.ToSQL()
		return err
	}
	const (
		tableType = "goqu: unsupported table type bool, a string or expression is required"
		badFrom   = "goqu: unsupported DeleteDataset#From argument bool, a string or identifier expression is required"
	)
	for _, c := range []struct {
		name string
		want string
		err  error
	}{
		{"delete From argument", badFrom, panicErr(func() { goqu.Delete("t").From(true) })},
		{"insert Into table", tableType, panicErr(func() { goqu.Insert("t").Into(true) })},
		{"update Table", tableType, panicErr(func() { goqu.Update("t").Table(true) })},
		{
			"rows of different lengths", "goqu: rows with different value length, expected 1 got 2",
			toSQL(goqu.Insert("t").Rows(goqu.Record{"a": 1}, goqu.Record{"a": 1, "b": 2})),
		},
		{
			"rows with different keys", `goqu: rows with different keys, expected ["a"] got ["b"]`,
			toSQL(goqu.Insert("t").Rows(goqu.Record{"a": 1}, goqu.Record{"b": 1})),
		},
		{
			"rows of different types", "goqu: rows must all be the same type, expected exp.Record got goqu_test.row",
			toSQL(goqu.Insert("t").Rows(goqu.Record{"a": 1}, row{A: 1})),
		},
		{
			"struct rows of different types", "goqu: rows must all be the same type, expected goqu_test.row got exp.Record",
			toSQL(goqu.Insert("t").Rows(row{A: 1}, goqu.Record{"a": 1})),
		},
		{
			"unsupported row type", "goqu: unsupported insert row type int, a map, goqu.Record or struct is required",
			toSQL(goqu.Insert("t").Rows(1)),
		},
		{
			"unsupported update value", "goqu: unsupported update value type int, a map, goqu.Record or struct is required",
			toSQL(goqu.Update("t").Set(1)),
		},
		{
			"unknown Op key", `goqu: unsupported Op key "bogus"`,
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
		assert.True(t, goqu.IsInvalidExpression(c.err), "%s: %v", c.name, c.err)
	}
}

func panicErr(f func()) (err error) {
	defer func() { err, _ = recover().(error) }()
	f()
	return nil
}

func TestNoDatabaseMessage(t *testing.T) {
	_, err := goqu.From("t").Count()
	assert.EqualError(t, err, "goqu: no database to execute the query, create the dataset with goqu.Database#From")
	assert.True(t, goqu.IsNoDatabase(err))
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
		assert.True(t, goqu.IsNoDatabase(err), name)
	}
}
