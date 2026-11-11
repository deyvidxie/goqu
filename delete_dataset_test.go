package goqu_test

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/sb"
	"github.com/doug-martin/goqu/v9/mocks"
	"github.com/doug-martin/goqu/v9/sqlbuilder"
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

	dialect, err := goqu.GetDialect("default")
	dds.Require().NoError(err)
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

	_, _, err := goqu.Delete("test").From(true).ToSQL()
	dds.EqualError(err, "goqu: unsupported DeleteDataset#From argument bool, a string or identifier expression is required")
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
		args.Get(0).(sqlbuilder.SQLBuilder).SetError(ee)
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

	ds := goqu.New("default", mDB).Delete("items").Where(goqu.Ex{"id": goqu.Op{"gt": 10}})

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
		args.Get(0).(sqlbuilder.SQLBuilder).SetError(err3)
	}).Once()

	sql, args, err = ds.ToSQL()
	dds.Empty(sql)
	dds.Empty(args)
	dds.Equal(err1, err)
}

func (dds *deleteDatasetSuite) TestEmptyOrder() {
	const (
		emptyOrderAppend  = "goqu: OrderAppend requires at least one expression"
		emptyOrderPrepend = "goqu: OrderPrepend requires at least one expression"
	)
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
		"Order":                       {bd.Order(noOrder...), emptyOrder},
		"Order nil":                   {bd.Order(nil), nilOrder},
		"OrderAppend":                 {bd.Order(goqu.C("a").Asc()).OrderAppend(noOrder...), emptyOrderAppend},
		"OrderAppend nil":             {bd.Order(goqu.C("a").Asc()).OrderAppend(nil), "goqu: OrderAppend expressions must not be nil"},
		"OrderPrepend":                {bd.OrderPrepend(noOrder...), emptyOrderPrepend},
		"OrderPrepend nil":            {bd.OrderPrepend(goqu.C("a").Asc(), nil), "goqu: OrderPrepend expressions must not be nil"},
		"OrderAppend then ClearOrder": {bd.Order(goqu.C("a").Asc()).OrderAppend(noOrder...).ClearOrder(), emptyOrderAppend},
		"OrderPrepend then Order":     {bd.OrderPrepend(noOrder...).Order(goqu.C("a").Asc()), emptyOrderPrepend},
		"from select":                 {goqu.Dialect("mysql").From("test").Order(noOrder...).Limit(10).Delete(), emptyOrder},
	} {
		_, _, err := c.ds.ToSQL()
		dds.EqualError(err, c.err, name)
		dds.True(goqu.IsInvalidExpression(err), name)
	}
}

func (dds *deleteDatasetSuite) TestEmptyWhere() {
	var none []exp.Expression
	const emptyWhere = "goqu: Where requires at least one expression; use ClearWhere to drop WHERE"
	bd := goqu.Delete("test")
	for name, c := range map[string]struct {
		ds interface {
			ToSQL() (string, []interface{}, error)
		}
		err string
	}{
		"no conditions": {bd.Where(), emptyWhere},
		"empty spread":  {bd.Where(none...), emptyWhere},
		"empty Ex":      {bd.Where(goqu.Ex{}), `goqu: empty AND condition`},
	} {
		_, _, err := c.ds.ToSQL()
		dds.EqualError(err, c.err, name)
		dds.True(goqu.IsInvalidExpression(err), name)
	}

	sql, _, err := bd.Where(goqu.C("a").Eq(1)).ClearWhere().ToSQL()
	dds.NoError(err)
	dds.Equal(`DELETE FROM "test"`, sql)
}

func (dds *deleteDatasetSuite) TestToSQL_defaultDialectRejectsLimitAndOrder() {
	bd := goqu.Delete("test")
	for name, c := range map[string]struct {
		ds  *goqu.DeleteDataset
		err string
	}{
		"Limit(0)": {bd.Limit(0), "goqu: dialect does not support LIMIT on DELETE [dialect=default]"},
		"Limit(1)": {bd.Limit(1), "goqu: dialect does not support LIMIT on DELETE [dialect=default]"},
		"Order":    {bd.Order(goqu.C("a").Asc()), "goqu: dialect does not support ORDER BY on DELETE [dialect=default]"},
	} {
		_, _, err := c.ds.ToSQL()
		dds.EqualError(err, c.err, name)
		dds.True(goqu.IsUnsupported(err), name)
	}
	sql, _, err := bd.LimitAll().ToSQL()
	dds.NoError(err)
	dds.Equal(`DELETE FROM "test"`, sql)
}

func TestDeleteDataset(t *testing.T) {
	suite.Run(t, new(deleteDatasetSuite))
}
