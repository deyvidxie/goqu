package goqu_test

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/sb"
	"github.com/doug-martin/goqu/v9/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type (
	deleteTestCase struct {
		ds      *goqu.DeleteDataset
		clauses exp.DeleteClauses
	}
	deleteDatasetSuite struct {
		suite.Suite
	}
)

func (dds *deleteDatasetSuite) assertCases(cases ...deleteTestCase) {
	for _, s := range cases {
		dds.Equal(s.clauses, s.ds.GetClauses())
	}
}

func (dds *deleteDatasetSuite) SetupSuite() {
	noReturn := goqu.DefaultDialectOptions()
	noReturn.SupportsReturn = false
	goqu.RegisterDialect("no-return", "db", noReturn)

	limitOnDelete := goqu.DefaultDialectOptions()
	limitOnDelete.SupportsLimitOnDelete = true
	goqu.RegisterDialect("limit-on-delete", "db", limitOnDelete)

	orderOnDelete := goqu.DefaultDialectOptions()
	orderOnDelete.SupportsOrderByOnDelete = true
	goqu.RegisterDialect("order-on-delete", "db", orderOnDelete)
}

func (dds *deleteDatasetSuite) TearDownSuite() {
	goqu.DeregisterDialect("no-return")
	goqu.DeregisterDialect("limit-on-delete")
	goqu.DeregisterDialect("order-on-delete")
}

func (dds *deleteDatasetSuite) TestDelete() {
	ds := goqu.Delete("test")
	dds.IsType(&goqu.DeleteDataset{}, ds)
	dds.Implements((*exp.Expression)(nil), ds)
	dds.Implements((*exp.AppendableExpression)(nil), ds)
}

func (dds *deleteDatasetSuite) TestClone() {
	ds := goqu.Delete("test")
	dds.Equal(ds.Clone(), ds)
}

func (dds *deleteDatasetSuite) TestExpression() {
	ds := goqu.Delete("test")
	dds.Equal(ds.Expression(), ds)
}

func (dds *deleteDatasetSuite) TestDialect() {
	ds := goqu.Delete("test")
	dds.NotNil(ds.Dialect())
}

func (dds *deleteDatasetSuite) TestWithDialect() {
	ds := goqu.Delete("test")
	md := new(mocks.SQLDialect)
	ds = ds.SetDialect(md)

	dialect := goqu.GetDialect("default")
	dialectDs := ds.WithDialect("default")
	dds.Equal(md, ds.Dialect())
	dds.Equal(dialect, dialectDs.Dialect())
}

func (dds *deleteDatasetSuite) TestPrepared() {
	ds := goqu.Delete("test")
	preparedDs := ds.Prepared(true)
	dds.True(preparedDs.IsPrepared())
	dds.False(ds.IsPrepared())
	// should apply the prepared to any datasets created from the root
	dds.True(preparedDs.Where(goqu.Ex{"a": 1}).IsPrepared())

	defer goqu.SetDefaultPrepared(false)
	goqu.SetDefaultPrepared(true)

	// should be prepared by default
	ds = goqu.Delete("test")
	dds.True(ds.IsPrepared())
}

func (dds *deleteDatasetSuite) TestGetClauses() {
	ds := goqu.Delete("test")
	ce := exp.NewDeleteClauses().SetFrom(goqu.I("test"))
	dds.Equal(ce, ds.GetClauses())
}

func (dds *deleteDatasetSuite) TestWith() {
	from := goqu.From("cte")
	bd := goqu.Delete("items")
	dds.assertCases(
		deleteTestCase{
			ds: bd.With("test-cte", from),
			clauses: exp.NewDeleteClauses().SetFrom(goqu.C("items")).
				CommonTablesAppend(exp.NewCommonTableExpression(false, "test-cte", from)),
		},
		deleteTestCase{
			ds:      bd,
			clauses: exp.NewDeleteClauses().SetFrom(goqu.C("items")),
		},
	)
}

func (dds *deleteDatasetSuite) TestWithRecursive() {
	from := goqu.From("cte")
	bd := goqu.Delete("items")
	dds.assertCases(
		deleteTestCase{
			ds: bd.WithRecursive("test-cte", from),
			clauses: exp.NewDeleteClauses().SetFrom(goqu.C("items")).
				CommonTablesAppend(exp.NewCommonTableExpression(true, "test-cte", from)),
		},
		deleteTestCase{
			ds:      bd,
			clauses: exp.NewDeleteClauses().SetFrom(goqu.C("items")),
		},
	)
}

func (dds *deleteDatasetSuite) TestFrom_withIdentifier() {
	bd := goqu.Delete("items")
	dds.assertCases(
		deleteTestCase{
			ds:      bd.From("items2"),
			clauses: exp.NewDeleteClauses().SetFrom(goqu.C("items2")),
		},
		deleteTestCase{
			ds:      bd.From(goqu.C("items2")),
			clauses: exp.NewDeleteClauses().SetFrom(goqu.C("items2")),
		},
		deleteTestCase{
			ds:      bd.From(goqu.T("items2")),
			clauses: exp.NewDeleteClauses().SetFrom(goqu.T("items2")),
		},
		deleteTestCase{
			ds:      bd.From("schema.table"),
			clauses: exp.NewDeleteClauses().SetFrom(goqu.I("schema.table")),
		},
		deleteTestCase{
			ds:      bd,
			clauses: exp.NewDeleteClauses().SetFrom(goqu.C("items")),
		},
	)

	dds.PanicsWithError("goqu: unsupported DeleteDataset#From argument bool, a string or identifier expression is required", func() {
		goqu.Delete("test").From(true)
	})
}

func (dds *deleteDatasetSuite) TestWhere() {
	bd := goqu.Delete("items")
	dds.assertCases(
		deleteTestCase{
			ds: bd.Where(goqu.Ex{"a": 1}),
			clauses: exp.NewDeleteClauses().
				SetFrom(goqu.C("items")).
				WhereAppend(goqu.Ex{"a": 1}),
		},
		deleteTestCase{
			ds: bd.Where(goqu.Ex{"a": 1}).Where(goqu.C("b").Eq("c")),
			clauses: exp.NewDeleteClauses().
				SetFrom(goqu.C("items")).
				WhereAppend(goqu.Ex{"a": 1}).
				WhereAppend(goqu.C("b").Eq("c")),
		},
		deleteTestCase{
			ds:      bd,
			clauses: exp.NewDeleteClauses().SetFrom(goqu.C("items")),
		},
	)
}

func (dds *deleteDatasetSuite) TestClearWhere() {
	bd := goqu.Delete("items").Where(goqu.Ex{"a": 1})
	dds.assertCases(
		deleteTestCase{
			ds: bd.ClearWhere(),
			clauses: exp.NewDeleteClauses().
				SetFrom(goqu.C("items")),
		},
		deleteTestCase{
			ds: bd,
			clauses: exp.NewDeleteClauses().
				SetFrom(goqu.C("items")).
				WhereAppend(goqu.Ex{"a": 1}),
		},
	)
}

func (dds *deleteDatasetSuite) TestOrder() {
	bd := goqu.Delete("items")
	dds.assertCases(
		deleteTestCase{
			ds: bd.Order(goqu.C("a").Asc()),
			clauses: exp.NewDeleteClauses().
				SetFrom(goqu.C("items")).
				SetOrder(goqu.C("a").Asc()),
		},
		deleteTestCase{
			ds: bd.Order(goqu.C("a").Asc()).Order(goqu.C("b").Desc()),
			clauses: exp.NewDeleteClauses().
				SetFrom(goqu.C("items")).
				SetOrder(goqu.C("b").Desc()),
		},
		deleteTestCase{
			ds: bd.Order(goqu.C("a").Asc(), goqu.C("b").Desc()),
			clauses: exp.NewDeleteClauses().
				SetFrom(goqu.C("items")).
				SetOrder(goqu.C("a").Asc(), goqu.C("b").Desc()),
		},
		deleteTestCase{
			ds:      bd,
			clauses: exp.NewDeleteClauses().SetFrom(goqu.C("items")),
		},
	)
}

func (dds *deleteDatasetSuite) TestOrderAppend() {
	bd := goqu.Delete("items").Order(goqu.C("a").Asc())
	dds.assertCases(
		deleteTestCase{
			ds: bd.OrderAppend(goqu.C("b").Desc()),
			clauses: exp.NewDeleteClauses().
				SetFrom(goqu.C("items")).
				SetOrder(goqu.C("a").Asc(), goqu.C("b").Desc()),
		},
		deleteTestCase{
			ds: bd,
			clauses: exp.NewDeleteClauses().
				SetFrom(goqu.C("items")).
				SetOrder(goqu.C("a").Asc()),
		},
	)
}

func (dds *deleteDatasetSuite) TestOrderPrepend() {
	bd := goqu.Delete("items").Order(goqu.C("a").Asc())
	dds.assertCases(
		deleteTestCase{
			ds: bd.OrderPrepend(goqu.C("b").Desc()),
			clauses: exp.NewDeleteClauses().
				SetFrom(goqu.C("items")).
				SetOrder(goqu.C("b").Desc(), goqu.C("a").Asc()),
		},
		deleteTestCase{
			ds: bd,
			clauses: exp.NewDeleteClauses().
				SetFrom(goqu.C("items")).
				SetOrder(goqu.C("a").Asc()),
		},
	)
}

func (dds *deleteDatasetSuite) TestClearOrder() {
	bd := goqu.Delete("items").Order(goqu.C("a").Asc())
	dds.assertCases(
		deleteTestCase{
			ds:      bd.ClearOrder(),
			clauses: exp.NewDeleteClauses().SetFrom(goqu.C("items")),
		},
		deleteTestCase{
			ds: bd,
			clauses: exp.NewDeleteClauses().
				SetFrom(goqu.C("items")).
				SetOrder(goqu.C("a").Asc()),
		},
	)
}

func (dds *deleteDatasetSuite) TestLimit() {
	bd := goqu.Delete("test")
	dds.assertCases(
		deleteTestCase{
			ds: bd.Limit(10),
			clauses: exp.NewDeleteClauses().
				SetFrom(goqu.C("test")).
				SetLimit(uint(10)),
		},
		deleteTestCase{
			ds:      bd.Limit(0),
			clauses: exp.NewDeleteClauses().SetFrom(goqu.C("test")).SetLimit(uint(0)),
		},
		deleteTestCase{
			ds: bd.Limit(10).Limit(2),
			clauses: exp.NewDeleteClauses().
				SetFrom(goqu.C("test")).
				SetLimit(uint(2)),
		},
		deleteTestCase{
			ds:      bd.Limit(10).Limit(0),
			clauses: exp.NewDeleteClauses().SetFrom(goqu.C("test")).SetLimit(uint(0)),
		},
		deleteTestCase{
			ds:      bd,
			clauses: exp.NewDeleteClauses().SetFrom(goqu.C("test")),
		},
	)
}

func (dds *deleteDatasetSuite) TestLimitAll() {
	bd := goqu.Delete("test")
	dds.assertCases(
		deleteTestCase{
			ds: bd.LimitAll(),
			clauses: exp.NewDeleteClauses().
				SetFrom(goqu.C("test")).
				SetLimit(goqu.L("ALL")),
		},
		deleteTestCase{
			ds: bd.Limit(10).LimitAll(),
			clauses: exp.NewDeleteClauses().
				SetFrom(goqu.C("test")).
				SetLimit(goqu.L("ALL")),
		},
		deleteTestCase{
			ds:      bd,
			clauses: exp.NewDeleteClauses().SetFrom(goqu.C("test")),
		},
	)
}

func (dds *deleteDatasetSuite) TestClearLimit() {
	bd := goqu.Delete("test").Limit(10)
	dds.assertCases(
		deleteTestCase{
			ds:      bd.ClearLimit(),
			clauses: exp.NewDeleteClauses().SetFrom(goqu.C("test")),
		},
		deleteTestCase{
			ds:      bd,
			clauses: exp.NewDeleteClauses().SetFrom(goqu.C("test")).SetLimit(uint(10)),
		},
	)
}

func (dds *deleteDatasetSuite) TestReturning() {
	bd := goqu.Delete("items")
	dds.assertCases(
		deleteTestCase{
			ds: bd.Returning("a"),
			clauses: exp.NewDeleteClauses().
				SetFrom(goqu.C("items")).
				SetReturning(exp.NewColumnListExpression(nil, "db", "a")),
		},
		deleteTestCase{
			ds: bd.Returning(),
			clauses: exp.NewDeleteClauses().
				SetFrom(goqu.C("items")).
				SetReturning(exp.NewColumnListExpression(nil, "db")),
		},
		deleteTestCase{
			ds: bd.Returning(nil),
			clauses: exp.NewDeleteClauses().
				SetFrom(goqu.C("items")).
				SetReturning(exp.NewColumnListExpression(nil, "db")),
		},
		deleteTestCase{
			ds: bd.Returning("a").Returning("b"),
			clauses: exp.NewDeleteClauses().
				SetFrom(goqu.C("items")).
				SetReturning(exp.NewColumnListExpression(nil, "db", "b")),
		},
		deleteTestCase{
			ds:      bd,
			clauses: exp.NewDeleteClauses().SetFrom(goqu.C("items")),
		},
	)
}

func (dds *deleteDatasetSuite) TestReturnsColumns() {
	ds := goqu.Delete("test")
	dds.False(ds.ReturnsColumns())
	dds.True(ds.Returning("foo", "bar").ReturnsColumns())
}

func (dds *deleteDatasetSuite) TestToSQL() {
	md := new(mocks.SQLDialect)
	ds := goqu.Delete("test").SetDialect(md)
	c := ds.GetClauses()
	sqlB := sb.NewSQLBuilder(false)
	md.On("ToDeleteSQL", builderMatching(sqlB), c).Return(nil).Once()

	sql, args, err := ds.ToSQL()
	dds.Empty(sql)
	dds.Empty(args)
	dds.Nil(err)
	md.AssertExpectations(dds.T())
}

func (dds *deleteDatasetSuite) TestToSQL_Prepared() {
	md := new(mocks.SQLDialect)
	ds := goqu.Delete("test").Prepared(true).SetDialect(md)
	c := ds.GetClauses()
	sqlB := sb.NewSQLBuilder(true)
	md.On("ToDeleteSQL", builderMatching(sqlB), c).Return(nil).Once()

	sql, args, err := ds.ToSQL()
	dds.Empty(sql)
	dds.Empty(args)
	dds.Nil(err)
	md.AssertExpectations(dds.T())
}

func (dds *deleteDatasetSuite) TestToSQL_WithError() {
	md := new(mocks.SQLDialect)
	ds := goqu.Delete("test").SetDialect(md)
	c := ds.GetClauses()
	ee := newTestError("expected error")
	sqlB := sb.NewSQLBuilder(false)
	md.On("ToDeleteSQL", builderMatching(sqlB), c).Run(func(args mock.Arguments) {
		args.Get(0).(sb.SQLBuilder).SetError(ee)
	}).Once()

	sql, args, err := ds.ToSQL()
	dds.Empty(sql)
	dds.Empty(args)
	dds.Equal(ee, err)
	md.AssertExpectations(dds.T())
}

func (dds *deleteDatasetSuite) TestExecutor() {
	mDB, _, err := sqlmock.New()
	dds.NoError(err)

	ds := goqu.New("mock", mDB).Delete("items").Where(goqu.Ex{"id": goqu.Op{"gt": 10}})

	dsql, args, err := ds.Executor().ToSQL()
	dds.NoError(err)
	dds.Empty(args)
	dds.Equal(`DELETE FROM "items" WHERE ("id" > 10)`, dsql)

	dsql, args, err = ds.Prepared(true).Executor().ToSQL()
	dds.NoError(err)
	dds.Equal([]interface{}{int64(10)}, args)
	dds.Equal(`DELETE FROM "items" WHERE ("id" > ?)`, dsql)

	defer goqu.SetDefaultPrepared(false)
	goqu.SetDefaultPrepared(true)

	dsql, args, err = ds.Executor().ToSQL()
	dds.NoError(err)
	dds.Equal([]interface{}{int64(10)}, args)
	dds.Equal(`DELETE FROM "items" WHERE ("id" > ?)`, dsql)
}

func (dds *deleteDatasetSuite) TestSetError() {
	err1 := newTestError("error #1")
	err2 := newTestError("error #2")
	err3 := newTestError("error #3")

	md := new(mocks.SQLDialect)
	ds := goqu.Delete("test").SetDialect(md)
	ds = ds.SetError(err1)
	dds.Equal(err1, ds.Error())
	sql, args, err := ds.ToSQL()
	dds.Empty(sql)
	dds.Empty(args)
	dds.Equal(err1, err)

	// Repeated SetError calls on Dataset should not overwrite the original error
	ds = ds.SetError(err2)
	dds.Equal(err1, ds.Error())
	sql, args, err = ds.ToSQL()
	dds.Empty(sql)
	dds.Empty(args)
	dds.Equal(err1, err)

	// Builder functions should not lose the error
	ds = ds.ClearLimit()
	dds.Equal(err1, ds.Error())
	sql, args, err = ds.ToSQL()
	dds.Empty(sql)
	dds.Empty(args)
	dds.Equal(err1, err)

	// Deeper errors inside SQL generation should still return original error
	c := ds.GetClauses()
	sqlB := sb.NewSQLBuilder(false)
	md.On("ToDeleteSQL", builderMatching(sqlB), c).Run(func(args mock.Arguments) {
		args.Get(0).(sb.SQLBuilder).SetError(err3)
	}).Once()

	sql, args, err = ds.ToSQL()
	dds.Empty(sql)
	dds.Empty(args)
	dds.Equal(err1, err)
}

func (dds *deleteDatasetSuite) TestToSQL_postgresRejectsLimitZero() {
	_, _, err := goqu.Dialect("postgres").Delete("test").Limit(0).ToSQL()
	dds.EqualError(err, "goqu: dialect does not support LIMIT on DELETE [dialect=postgres]")
}

func (dds *deleteDatasetSuite) TestEmptyOrder() {
	var noOrder []exp.OrderedExpression
	bd := goqu.Delete("test")
	const (
		emptyOrder = "goqu: Order requires at least one expression; use ClearOrder to drop ORDER BY"
		nilOrder   = "goqu: Order expressions must not be nil"
	)
	for name, c := range map[string]struct {
		ds interface {
			ToSQL() (string, []interface{}, error)
		}
		err string
	}{
		"Order":            {bd.Order(noOrder...), emptyOrder},
		"Order nil":        {bd.Order(nil), nilOrder},
		"OrderAppend":      {bd.Order(goqu.C("a").Asc()).OrderAppend(noOrder...), "goqu: OrderAppend requires at least one expression"},
		"OrderAppend nil":  {bd.Order(goqu.C("a").Asc()).OrderAppend(nil), nilOrder},
		"OrderPrepend":     {bd.OrderPrepend(noOrder...), "goqu: OrderPrepend requires at least one expression"},
		"OrderPrepend nil": {bd.OrderPrepend(goqu.C("a").Asc(), nil), nilOrder},
		"from select":      {goqu.Dialect("mysql").From("test").Order(noOrder...).Limit(10).Delete(), emptyOrder},
	} {
		_, _, err := c.ds.ToSQL()
		dds.EqualError(err, c.err, name)
		dds.True(goqu.IsInvalidExpression(err), name)
	}
}

func (dds *deleteDatasetSuite) TestToSQL_emptyWhere() {
	var none []exp.Expression
	for name, d := range map[string]interface {
		ToSQL() (string, []interface{}, error)
	}{
		"no conditions":        goqu.Delete("test").Where(),
		"empty Ex":             goqu.Delete("test").Where(goqu.Ex{}),
		"empty And":            goqu.Delete("test").Where(goqu.And()),
		"empty literal":        goqu.Delete("test").Where(goqu.L("")),
		"Or of empty Ex":       goqu.Delete("test").Where(goqu.Or(goqu.Ex{})),
		"Or with empty And":    goqu.Delete("test").Where(goqu.Or(goqu.C("a").Eq(1), goqu.And())),
		"And of empty groups":  goqu.Delete("test").Where(goqu.And(goqu.And(), goqu.Ex{})),
		"from select":          goqu.From("test").Where(none...).Delete(),
		"from select empty Ex": goqu.From("test").Where(goqu.Ex{}).Delete(),
	} {
		_, _, err := d.ToSQL()
		dds.EqualError(err, "goqu: WHERE matches every row when generating delete sql; omit Where or call ClearWhere to delete every row", name)
		dds.True(goqu.IsInvalidExpression(err), name)
	}

	for name, c := range map[string]struct {
		ds interface {
			ToSQL() (string, []interface{}, error)
		}
		sql string
	}{
		"no Where":                 {goqu.Delete("test"), `DELETE FROM "test"`},
		"cleared":                  {goqu.Delete("test").Where().ClearWhere(), `DELETE FROM "test"`},
		"empty then real":          {goqu.Delete("test").Where(none...).Where(goqu.C("a").Eq(1)), `DELETE FROM "test" WHERE ("a" = 1)`},
		"real then empty":          {goqu.Delete("test").Where(goqu.C("a").Eq(1)).Where(none...), `DELETE FROM "test" WHERE ("a" = 1)`},
		"empty Or matches nothing": {goqu.Delete("test").Where(goqu.Or()), `DELETE FROM "test" WHERE (1=0)`},
	} {
		sql, _, err := c.ds.ToSQL()
		dds.NoError(err, name)
		dds.Equal(c.sql, sql, name)
	}
}

func (dds *deleteDatasetSuite) TestToSQL_defaultDialectRejectsLimitDropsOrder() {
	for _, limit := range []uint{0, 1} {
		_, _, err := goqu.Delete("test").Limit(limit).ToSQL()
		dds.EqualError(err, "goqu: dialect does not support LIMIT on DELETE [dialect=default]")
	}
	sql, _, err := goqu.Delete("test").LimitAll().Order(goqu.C("a").Asc()).ToSQL()
	dds.NoError(err)
	dds.Equal(`DELETE FROM "test"`, sql)
}

func TestDeleteDataset(t *testing.T) {
	suite.Run(t, new(deleteDatasetSuite))
}
