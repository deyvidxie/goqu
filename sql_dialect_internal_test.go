package goqu

import (
	"sync"
	"testing"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/sb"
	"github.com/doug-martin/goqu/v9/sqlgen"
	"github.com/doug-martin/goqu/v9/sqlgen/mocks"
	"github.com/stretchr/testify/suite"
)

type dialectTestSuite struct {
	suite.Suite
}

func (dts *dialectTestSuite) TestDialect() {
	opts := DefaultDialectOptions()
	sm := new(mocks.SelectSQLGenerator)
	d := sqlDialect{dialect: "test", dialectOptions: opts, selectGen: sm}

	dts.Equal("test", d.Dialect())
}

func (dts *dialectTestSuite) TestToSelectSQL() {
	opts := DefaultDialectOptions()
	sm := new(mocks.SelectSQLGenerator)
	d := sqlDialect{dialect: "test", dialectOptions: opts, selectGen: sm}

	b := sb.NewSQLBuilder(true)
	sc := exp.NewSelectClauses()
	sm.On("Generate", b, sc).Return(nil).Once()

	d.ToSelectSQL(b, sc)
	sm.AssertExpectations(dts.T())
}

func (dts *dialectTestSuite) TestToUpdateSQL() {
	opts := DefaultDialectOptions()
	um := new(mocks.UpdateSQLGenerator)
	d := sqlDialect{dialect: "test", dialectOptions: opts, updateGen: um}

	b := sb.NewSQLBuilder(true)
	uc := exp.NewUpdateClauses()
	um.On("Generate", b, uc).Return(nil).Once()

	d.ToUpdateSQL(b, uc)
	um.AssertExpectations(dts.T())
}

func (dts *dialectTestSuite) TestToInsertSQL() {
	opts := DefaultDialectOptions()
	im := new(mocks.InsertSQLGenerator)
	d := sqlDialect{dialect: "test", dialectOptions: opts, insertGen: im}

	b := sb.NewSQLBuilder(true)
	ic := exp.NewInsertClauses()
	im.On("Generate", b, ic).Return(nil).Once()

	d.ToInsertSQL(b, ic)
	im.AssertExpectations(dts.T())
}

func (dts *dialectTestSuite) TestToDeleteSQL() {
	opts := DefaultDialectOptions()
	dm := new(mocks.DeleteSQLGenerator)
	d := sqlDialect{dialect: "test", dialectOptions: opts, deleteGen: dm}

	b := sb.NewSQLBuilder(true)
	dc := exp.NewDeleteClauses()
	dm.On("Generate", b, dc).Return(nil).Once()

	d.ToDeleteSQL(b, dc)
	dm.AssertExpectations(dts.T())
}

func (dts *dialectTestSuite) TestToTruncateSQL() {
	opts := DefaultDialectOptions()
	tm := new(mocks.TruncateSQLGenerator)
	d := sqlDialect{dialect: "test", dialectOptions: opts, truncateGen: tm}

	b := sb.NewSQLBuilder(true)
	tc := exp.NewTruncateClauses()
	tm.On("Generate", b, tc).Return(nil).Once()

	d.ToTruncateSQL(b, tc)
	tm.AssertExpectations(dts.T())
}

func (dts *dialectTestSuite) TestToExpressionSQL() {
	opts := DefaultDialectOptions()
	d := sqlDialect{dialect: "test", dialectOptions: opts, expressionGen: sqlgen.NewExpressionSQLGenerator("test", "db", opts)}

	b := sb.NewSQLBuilder(true)
	d.toExpressionSQL(b, exp.NewIdentifierExpression("", "", "a").Eq(1))
	sql, args, err := b.ToSQL()
	dts.Require().NoError(err)
	dts.Equal(`("a" = ?)`, sql)
	dts.Equal([]interface{}{int64(1)}, args)
}

func (dts *dialectTestSuite) TestGetDialect() {
	RegisterDialect("get-test", "db", DefaultDialectOptions())
	defer DeregisterDialect("get-test")

	d, err := GetDialect("GET-TEST")
	dts.Require().NoError(err)
	dts.Equal("get-test", d.Dialect())

	d, err = GetDialect("not-registered")
	dts.Nil(d)
	dts.True(IsUnknownDialect(err))
}

func (dts *dialectTestSuite) TestGetDialectWithTag() {
	RegisterDialect("tag-test", "db", DefaultDialectOptions())
	defer DeregisterDialect("tag-test")

	get := func(name, tag string) SQLDialect {
		d, err := GetDialectWithTag(name, tag)
		dts.Require().NoError(err)
		return d
	}
	registered, err := GetDialect("tag-test")
	dts.Require().NoError(err)
	dbTagged := get("tag-test", "db")
	dts.Same(registered, dbTagged)
	dts.Same(dbTagged, get("tag-test", ""))

	d, err := GetDialectWithTag("not-registered", "json")
	dts.Nil(d)
	dts.True(IsUnknownDialect(err))

	jsonTagged := get("tag-test", "json")
	dts.NotSame(dbTagged, jsonTagged)
	dts.Same(jsonTagged, get("tag-test", "json"))
	dts.Equal("tag-test", jsonTagged.Dialect())

	RegisterDialect("tag-test", "db", DefaultDialectOptions())
	dts.NotSame(jsonTagged, get("tag-test", "json"))
}

func (dts *dialectTestSuite) TestGetDialectWithTag_concurrentRegister() {
	defer DeregisterDialect("race-test")
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			RegisterDialect("race-test", "db", DefaultDialectOptions())
		}()
		go func() {
			defer wg.Done()
			_, _ = GetDialectWithTag("race-test", "json")
			_, _ = GetDialect("race-test")
			registeredTagName("race-test")
		}()
	}
	wg.Wait()
}

func TestSQLDialect(t *testing.T) {
	suite.Run(t, new(dialectTestSuite))
}
