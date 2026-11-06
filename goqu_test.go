package goqu_test

import (
	"fmt"
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

func (dws *dialectWrapperSuite) TestLookupDialect() {
	for _, name := range []string{"test", "TEST", "default"} {
		dw, err := goqu.LookupDialect(name)
		dws.Require().NoError(err, name)
		dws.Equal(goqu.Dialect(name), dw, name)
	}

	dw, err := goqu.LookupDialectWithTagName("test", "json")
	dws.Require().NoError(err)
	dws.Equal(goqu.DialectWithTagName("test", "json"), dw)
}

func (dws *dialectWrapperSuite) TestLookupDialect_unregistered() {
	const msg = `goqu: dialect "nope" is not registered; blank-import its dialect package or call RegisterDialect`

	dw, err := goqu.LookupDialect("nope")
	dws.EqualError(err, msg)
	dws.Equal(goqu.DialectWrapper{}, dw)
	dws.True(goqu.IsUnknownDialect(err))
	dws.True(goqu.IsUnknownDialect(fmt.Errorf("opening: %w", err)))
	var target goqu.UnknownDialectError
	dws.ErrorAs(err, &target)

	_, err = goqu.LookupDialectWithTagName("nope", "json")
	dws.EqualError(err, msg)
}

func (dws *dialectWrapperSuite) TestLookupDialect_deregistered() {
	goqu.RegisterDialect("gone", "db", goqu.DefaultDialectOptions())
	goqu.DeregisterDialect("gone")

	_, err := goqu.LookupDialect("gone")
	dws.True(goqu.IsUnknownDialect(err))
}

func (dws *dialectWrapperSuite) TestIsUnknownDialect() {
	dws.False(goqu.IsUnknownDialect(nil))
	dws.False(goqu.IsUnknownDialect(goqu.ErrQueryFactoryNotFoundError))
}

func TestDialectWrapper(t *testing.T) {
	suite.Run(t, new(dialectWrapperSuite))
}
