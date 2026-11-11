package goqu_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/mysql"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
	_ "github.com/doug-martin/goqu/v9/dialect/sqlserver"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/stretchr/testify/suite"
)

type (
	dialectWrapperSuite struct {
		suite.Suite
	}
)

func (dws *dialectWrapperSuite) SetupSuite() {
	testDialect := goqu.DefaultDialectOptions()
	// override to some value to ensure correct dialect is set
	goqu.RegisterDialect("test", "db", testDialect)
}

func (dws *dialectWrapperSuite) TearDownSuite() {
	goqu.DeregisterDialect("test")
}

func (dws *dialectWrapperSuite) TestFrom() {
	dw := goqu.Dialect("test")
	dws.Equal(goqu.From("table").WithDialect("test"), dw.From("table"))
}

func (dws *dialectWrapperSuite) TestSelect() {
	dw := goqu.Dialect("test")
	dws.Equal(goqu.Select("col").WithDialect("test"), dw.Select("col"))
}

func (dws *dialectWrapperSuite) TestInsert() {
	dw := goqu.Dialect("test")
	dws.Equal(goqu.Insert("table").WithDialect("test"), dw.Insert("table"))
}

func (dws *dialectWrapperSuite) TestDelete() {
	dw := goqu.Dialect("test")
	dws.Equal(goqu.Delete("table").WithDialect("test"), dw.Delete("table"))
}

func (dws *dialectWrapperSuite) TestTruncate() {
	dw := goqu.Dialect("test")
	dws.Equal(goqu.Truncate("table").WithDialect("test"), dw.Truncate("table"))
}

func (dws *dialectWrapperSuite) TestDB() {
	mDB, _, err := sqlmock.New()
	dws.Require().NoError(err)
	dw := goqu.Dialect("test")
	dws.Equal(goqu.New("test", mDB), dw.DB(mDB))
}

func (dws *dialectWrapperSuite) TestTx() {
	mDB, mock, err := sqlmock.New()
	dws.Require().NoError(err)
	defer mDB.Close()
	mock.ExpectBegin()
	tx, err := mDB.BeginTx(context.Background(), nil)
	dws.Require().NoError(err)
	dws.Equal(goqu.NewTx("test", tx), goqu.Dialect("test").Tx(tx))
	dws.Equal(goqu.NewTxWithTagName("test", "json", tx), goqu.DialectWithTagName("test", "json").Tx(tx))
}

func (dws *dialectWrapperSuite) TestLookupDialect() {
	for _, name := range []string{"test", "TEST", "default"} {
		dw, err := goqu.LookupDialect(name)
		dws.Require().NoError(err, name)
		dws.Equal(goqu.Dialect(name), dw, name)
	}
}

func (dws *dialectWrapperSuite) TestLookupDialect_unregistered() {
	dw, err := goqu.LookupDialect("nope")
	dws.True(goqu.IsUnknownDialect(err))
	dws.Equal(goqu.DialectWrapper{}, dw)
}

func (dws *dialectWrapperSuite) TestLookupDialectWithTagName() {
	dw, err := goqu.LookupDialectWithTagName("test", "json")
	dws.Require().NoError(err)
	dws.Equal(goqu.DialectWithTagName("test", "json"), dw)
}

func (dws *dialectWrapperSuite) TestLookupDialectWithTagName_unregistered() {
	dw, err := goqu.LookupDialectWithTagName("nope", "json")
	dws.True(goqu.IsUnknownDialect(err))
	dws.Equal(goqu.DialectWrapper{}, dw)
}

func (dws *dialectWrapperSuite) TestExpressionSQL() {
	for _, c := range []struct {
		dialect string
		expr    exp.Expression
		sql     string
	}{
		{"mysql", goqu.C("a").Eq(1), "(`a` = 1)"},
		{"mysql", goqu.I("t.a"), "`t`.`a`"},
		{"mysql", goqu.Func("COALESCE", goqu.C("a"), "x"), "COALESCE(`a`, 'x')"},
		{"postgres", goqu.C("a").Eq(1), `("a" = 1)`},
		{"postgres", goqu.I("t.a"), `"t"."a"`},
		{"postgres", goqu.Func("COALESCE", goqu.C("a"), "x"), `COALESCE("a", 'x')`},
	} {
		sql, args, err := goqu.Dialect(c.dialect).ExpressionSQL(c.expr)
		dws.Require().NoError(err, c.sql)
		dws.Equal(c.sql, sql)
		dws.Empty(args, c.sql)
	}
}

func (dws *dialectWrapperSuite) TestPackageExpressionSQL() {
	sql, args, err := goqu.ExpressionSQL(goqu.C("a").Eq(1))
	dws.NoError(err)
	dws.Equal(`("a" = 1)`, sql)
	dws.Empty(args)

	sql, args, err = goqu.PreparedExpressionSQL(goqu.C("a").Eq(1))
	dws.NoError(err)
	dws.Equal(`("a" = ?)`, sql)
	dws.Equal([]interface{}{int64(1)}, args)
}

func (dws *dialectWrapperSuite) TestPackageExpressionSQL_followsRegisteredDefault() {
	opts := goqu.DefaultDialectOptions()
	opts.QuoteRune = '`'
	goqu.RegisterDialect("default", "db", opts)
	defer goqu.RegisterDialect("default", "db", goqu.DefaultDialectOptions())

	sql, _, err := goqu.ExpressionSQL(goqu.C("a").Eq(1))
	dws.NoError(err)
	dws.Equal("(`a` = 1)", sql)
}

func (dws *dialectWrapperSuite) TestPreparedExpressionSQL() {
	for _, c := range []struct {
		dialect string
		expr    exp.Expression
		sql     string
		args    []interface{}
	}{
		{"mysql", goqu.C("a").Eq(1), "(`a` = ?)", []interface{}{int64(1)}},
		{"mysql", goqu.I("t.a"), "`t`.`a`", []interface{}{}},
		{"mysql", goqu.Func("COALESCE", goqu.C("a"), "x"), "COALESCE(`a`, ?)", []interface{}{"x"}},
		{"postgres", goqu.C("a").Eq(1), `("a" = $1)`, []interface{}{int64(1)}},
		{"postgres", goqu.I("t.a"), `"t"."a"`, []interface{}{}},
		{"postgres", goqu.Func("COALESCE", goqu.C("a"), "x"), `COALESCE("a", $1)`, []interface{}{"x"}},
	} {
		sql, args, err := goqu.Dialect(c.dialect).PreparedExpressionSQL(c.expr)
		dws.Require().NoError(err, c.sql)
		dws.Equal(c.sql, sql)
		dws.Equal(c.args, args, c.sql)
	}
}

func (dws *dialectWrapperSuite) TestExpressionSQL_unsupportedOperator() {
	dw := goqu.Dialect("sqlserver")
	for _, render := range []func(exp.Expression) (string, []interface{}, error){dw.ExpressionSQL, dw.PreparedExpressionSQL} {
		sql, args, err := render(goqu.C("a").BitwiseLeftShift(1))
		dws.EqualError(err, "goqu: dialect does not support bitwise operator \"Left Shift\" [dialect=sqlserver]")
		dws.True(goqu.IsUnsupported(err))
		dws.Empty(sql)
		dws.Empty(args)
	}
}

func (dws *dialectWrapperSuite) TestExpressionSQL_errorNamesDialect() {
	_, _, err := goqu.DialectWithTagName("sqlserver", "json").ExpressionSQL(goqu.C("a").IsTrue())
	dws.EqualError(err, "goqu: dialect does not support the boolean data type [dialect=sqlserver]")
}

func (dws *dialectWrapperSuite) TestExpressionSQL_ignoresDefaultPrepared() {
	defer goqu.SetDefaultPrepared(false)
	goqu.SetDefaultPrepared(true)

	sql, args, err := goqu.Dialect("mysql").ExpressionSQL(goqu.C("a").Eq(1))
	dws.Require().NoError(err)
	dws.Equal("(`a` = 1)", sql)
	dws.Empty(args)
}

func (dws *dialectWrapperSuite) TestExpressionSQL_unregisteredDialect() {
	sql, _, err := goqu.Dialect("nope").ExpressionSQL(goqu.C("a").Eq(1))
	dws.Require().NoError(err)
	dws.Equal(`("a" = 1)`, sql)
}

func (dws *dialectWrapperSuite) TestPreparedExpressionSQL_tagName() {
	sql, args, err := goqu.DialectWithTagName("postgres", "json").PreparedExpressionSQL(goqu.C("a").Eq(1))
	dws.Require().NoError(err)
	dws.Equal(`("a" = $1)`, sql)
	dws.Equal([]interface{}{int64(1)}, args)
}

func TestDialectWrapper(t *testing.T) {
	suite.Run(t, new(dialectWrapperSuite))
}
