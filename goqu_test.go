package goqu_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/doug-martin/goqu/v9"
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
	dws.EqualError(err, `goqu: dialect "nope" is not registered; blank-import its dialect package or call RegisterDialect`)
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
	dws.EqualError(err, `goqu: dialect "nope" is not registered; blank-import its dialect package or call RegisterDialect`)
	dws.True(goqu.IsUnknownDialect(err))
	dws.Equal(goqu.DialectWrapper{}, dw)
}

func TestDialectWrapper(t *testing.T) {
	suite.Run(t, new(dialectWrapperSuite))
}
