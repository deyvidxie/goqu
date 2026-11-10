package goqu_test

import (
	dbsql "database/sql"
	"net/netip"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/doug-martin/goqu/v9/internal/sb"
	"github.com/doug-martin/goqu/v9/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type (
	selectTestCase struct {
		ds      *goqu.SelectDataset
		clauses exp.SelectClauses
	}
	dsTestActionItem struct {
		Address string `db:"address"`
		Name    string `db:"name"`
	}
	dsUntaggedTestActionItem struct {
		Address  string `db:"address"`
		Name     string `db:"name"`
		Untagged string
	}
	dsSubQueryTestActionItem struct {
		WithAddress bool `db:"withAddress" subquery:"(\"address\" IS NOT NULL)"`
		WithName    bool `db:"withName" subquery:"sub"`
	}
	selectDatasetSuite struct {
		suite.Suite
	}
	dsHiddenColumns struct {
		X int `db:"x"`
	}
	dsUnexportedEmbedRow struct {
		*dsHiddenColumns
		ID int `db:"id"`
	}
)

func (sds *selectDatasetSuite) assertCases(cases ...selectTestCase) {
	for _, s := range cases {
		sds.Equal(s.clauses, s.ds.GetClauses())
	}
}

func (sds *selectDatasetSuite) TestReturnsColumns() {
	ds := goqu.Select(goqu.L("NOW()"))
	sds.True(ds.ReturnsColumns())
}

func (sds *selectDatasetSuite) TestClone() {
	ds := goqu.From("test")
	sds.Equal(ds, ds.Clone())
}

func (sds *selectDatasetSuite) TestExpression() {
	ds := goqu.From("test")
	sds.Equal(ds, ds.Expression())
}

func (sds *selectDatasetSuite) TestDialect() {
	ds := goqu.From("test")
	sds.NotNil(ds.Dialect())
}

func (sds *selectDatasetSuite) TestWithDialect() {
	ds := goqu.From("test")
	md := new(mocks.SQLDialect)
	ds = ds.SetDialect(md)

	dialect := goqu.GetDialect("default")
	dialectDs := ds.WithDialect("default")
	sds.Equal(md, ds.Dialect())
	sds.Equal(dialect, dialectDs.Dialect())
}

func (sds *selectDatasetSuite) TestPrepared() {
	ds := goqu.From("test")
	preparedDs := ds.Prepared(true)
	sds.True(preparedDs.IsPrepared())
	sds.False(ds.IsPrepared())
	// should apply the prepared to any datasets created from the root
	sds.True(preparedDs.Where(goqu.Ex{"a": 1}).IsPrepared())

	defer goqu.SetDefaultPrepared(false)
	goqu.SetDefaultPrepared(true)

	// should be prepared by default
	ds = goqu.From("test")
	sds.True(ds.IsPrepared())
}

func (sds *selectDatasetSuite) TestGetClauses() {
	ds := goqu.From("test")
	ce := exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", goqu.I("test")))
	sds.Equal(ce, ds.GetClauses())
}

func (sds *selectDatasetSuite) TestUpdate() {
	where := goqu.Ex{"a": 1}
	from := goqu.From("cte")
	limit := uint(1)
	order := []exp.OrderedExpression{goqu.C("a").Asc(), goqu.C("b").Desc()}
	ds := goqu.From("test").
		With("test-cte", from).
		Where(where).
		Limit(limit).
		Order(order...)
	ec := exp.NewUpdateClauses().
		SetTable(goqu.C("test")).
		CommonTablesAppend(exp.NewCommonTableExpression(false, "test-cte", from)).
		WhereAppend(ds.GetClauses().Where()).
		SetLimit(limit).
		SetOrder(order...)
	sds.Equal(ec, ds.Update().GetClauses())
}

func (sds *selectDatasetSuite) TestInsert() {
	where := goqu.Ex{"a": 1}
	from := goqu.From("cte")
	limit := uint(1)
	order := []exp.OrderedExpression{goqu.C("a").Asc(), goqu.C("b").Desc()}
	ds := goqu.From("test").
		With("test-cte", from).
		Where(where).
		Limit(limit).
		Order(order...)
	ec := exp.NewInsertClauses().
		SetInto(goqu.C("test")).
		CommonTablesAppend(exp.NewCommonTableExpression(false, "test-cte", from))
	sds.Equal(ec, ds.Insert().GetClauses())
}

func (sds *selectDatasetSuite) TestDelete() {
	where := goqu.Ex{"a": 1}
	from := goqu.From("cte")
	limit := uint(1)
	order := []exp.OrderedExpression{goqu.C("a").Asc(), goqu.C("b").Desc()}
	ds := goqu.From("test").
		With("test-cte", from).
		Where(where).
		Limit(limit).
		Order(order...)
	ec := exp.NewDeleteClauses().
		SetFrom(goqu.C("test")).
		CommonTablesAppend(exp.NewCommonTableExpression(false, "test-cte", from)).
		WhereAppend(ds.GetClauses().Where()).
		SetLimit(limit).
		SetOrder(order...)
	sds.Equal(ec, ds.Delete().GetClauses())
}

func (sds *selectDatasetSuite) TestTruncate() {
	where := goqu.Ex{"a": 1}
	from := goqu.From("cte")
	limit := uint(1)
	order := []exp.OrderedExpression{goqu.C("a").Asc(), goqu.C("b").Desc()}
	ds := goqu.From("test").
		With("test-cte", from).
		Where(where).
		Limit(limit).
		Order(order...)
	ec := exp.NewTruncateClauses().
		SetTable(exp.NewColumnListExpression(nil, "db", "test"))
	sds.Equal(ec, ds.Truncate().GetClauses())
}

func (sds *selectDatasetSuite) TestWith() {
	from := goqu.From("cte")
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.With("test-cte", from),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				CommonTablesAppend(exp.NewCommonTableExpression(false, "test-cte", from)),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestWithRecursive() {
	from := goqu.From("cte")
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.WithRecursive("test-cte", from),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				CommonTablesAppend(exp.NewCommonTableExpression(true, "test-cte", from)),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestSelect() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.Select("a", "b"),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetSelect(exp.NewColumnListExpression(nil, "db", "a", "b")),
		},
		selectTestCase{
			ds: bd.Select("a").Select("b"),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetSelect(exp.NewColumnListExpression(nil, "db", "b")),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestSelect_withTypeOnlyAndArrayStructSources() {
	for _, ds := range []*goqu.SelectDataset{
		goqu.From("t").Select((*dsTestActionItem)(nil)),
		goqu.From("t").Select((*[]dsTestActionItem)(nil)),
		goqu.From("t").Select([2]dsTestActionItem{}),
		goqu.From("t").Select(&[2]*dsTestActionItem{}),
	} {
		sql, _, err := ds.ToSQL()
		sds.NoError(err)
		sds.Equal(`SELECT "address", "name" FROM "t"`, sql)
	}
}

func (sds *selectDatasetSuite) TestSelectDistinct() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.SelectDistinct("a", "b"),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetSelect(exp.NewColumnListExpression(nil, "db", "a", "b")).
				SetDistinct(exp.NewColumnListExpression(nil, "db")),
		},
		selectTestCase{
			ds: bd.SelectDistinct("a").SelectDistinct("b"),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetSelect(exp.NewColumnListExpression(nil, "db", "b")).
				SetDistinct(exp.NewColumnListExpression(nil, "db")),
		},
		selectTestCase{
			ds: bd.Select("a").SelectDistinct("b"),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetSelect(exp.NewColumnListExpression(nil, "db", "b")).
				SetDistinct(exp.NewColumnListExpression(nil, "db")),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestEmptyArguments() {
	bd := goqu.From("test")
	var none []interface{}
	var noOrder []exp.OrderedExpression
	const (
		emptySelect  = "goqu: Select requires at least one column; use ClearSelect for SELECT *"
		emptyGroupBy = "goqu: GroupBy requires at least one column; use ClearGroupBy to drop GROUP BY"
		emptyOrder   = "goqu: Order requires at least one expression; use ClearOrder to drop ORDER BY"
	)
	for name, c := range map[string]struct {
		ds  *goqu.SelectDataset
		err string
	}{
		"Select":     {bd.Select("a").Select(none...), emptySelect},
		"Select nil": {bd.Select(nil), emptySelect},
		"Select no columns": {bd.Select(struct {
			A int `db:"-"`
		}{}), emptySelect},
		"package Select":      {goqu.Select(none...).From("test"), emptySelect},
		"SelectDistinct":      {bd.SelectDistinct(none...), emptySelect},
		"SelectAppend":        {bd.Select("a").SelectAppend(none...), emptySelect},
		"GroupBy":             {bd.GroupBy(none...), emptyGroupBy},
		"GroupBy nil":         {bd.GroupBy(nil), emptyGroupBy},
		"GroupByAppend":       {bd.GroupBy("a").GroupByAppend(none...), emptyGroupBy},
		"Order":               {bd.Order(noOrder...), emptyOrder},
		"Order nil":           {bd.Order(nil), emptyOrder},
		"OrderAppend":         {bd.Order(goqu.C("a").Asc()).OrderAppend(noOrder...), emptyOrder},
		"OrderAppend nil":     {bd.Order(goqu.C("a").Asc()).OrderAppend(nil), emptyOrder},
		"OrderPrepend":        {bd.OrderPrepend(noOrder...), emptyOrder},
		"OrderPrepend nil":    {bd.OrderPrepend(goqu.C("a").Asc(), nil), emptyOrder},
		"Order with Limit":    {bd.Order(noOrder...).Limit(10), emptyOrder},
		"Select then cleared": {bd.Select(none...).ClearSelect(), emptySelect},
	} {
		_, _, err := c.ds.ToSQL()
		sds.EqualError(err, c.err, name)
	}

	for name, ds := range map[string]*goqu.SelectDataset{
		"ClearSelect":  bd.Select("a").ClearSelect(),
		"ClearOrder":   bd.Order(goqu.C("a").Asc()).ClearOrder(),
		"ClearGroupBy": bd.GroupBy("a").ClearGroupBy(),
		"empty Where":  bd.Where(),
	} {
		sql, _, err := ds.ToSQL()
		sds.NoError(err, name)
		sds.Equal(`SELECT * FROM "test"`, sql, name)
	}
}

func (sds *selectDatasetSuite) TestClearSelect() {
	bd := goqu.From("test").Select("a")
	sds.assertCases(
		selectTestCase{
			ds: bd.ClearSelect(),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
		selectTestCase{
			ds: bd,
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetSelect(exp.NewColumnListExpression(nil, "db", "a")),
		},
	)
}

func (sds *selectDatasetSuite) TestSelectAppend() {
	bd := goqu.From("test").Select("a")
	sds.assertCases(
		selectTestCase{
			ds: bd.SelectAppend("b"),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetSelect(exp.NewColumnListExpression(nil, "db", "a", "b")),
		},
		selectTestCase{
			ds: bd,
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetSelect(exp.NewColumnListExpression(nil, "db", "a")),
		},
	)
}

func (sds *selectDatasetSuite) TestDistinctOn() {
	bd := goqu.Dialect("postgres").From("test")
	sds.Equal(bd.Distinct("a", "b"), bd.DistinctOn("a", "b"))

	sql, _, err := bd.DistinctOn("a").ToSQL()
	sds.NoError(err)
	sds.Equal(`SELECT DISTINCT ON ("a") * FROM "test"`, sql)

	var none []interface{}
	for name, ds := range map[string]*goqu.SelectDataset{
		"nil spread":   bd.DistinctOn(none...),
		"empty spread": bd.DistinctOn([]interface{}{}...),
		"nil column":   bd.DistinctOn(nil),
		"empty list":   bd.DistinctOn(exp.NewColumnListExpression(nil, "db")),
	} {
		_, _, err := ds.ToSQL()
		sds.True(goqu.IsInvalidExpression(err), "%s: %v", name, err)
	}
	sds.NoError(bd.Error())
}

func (sds *selectDatasetSuite) TestDistinct() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.Distinct("a", "b"),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetDistinct(exp.NewColumnListExpression(nil, "db", "a", "b")),
		},
		selectTestCase{
			ds: bd.Distinct("a").Distinct("b"),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetDistinct(exp.NewColumnListExpression(nil, "db", "b")),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestFrom() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.From(goqu.T("test2")),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", goqu.T("test2"))),
		},
		selectTestCase{
			ds: bd.From(goqu.From("test")),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", goqu.From("test").As("t1"))),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestFromSelf() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.FromSelf(),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", bd.As("t1"))),
		},
		selectTestCase{
			ds: bd.As("alias").FromSelf(),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", bd.As("alias"))),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestCompoundFromSelf() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds:      bd.CompoundFromSelf(),
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
		selectTestCase{
			ds:      bd.Limit(10).CompoundFromSelf(),
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", bd.Limit(10).As("t1"))),
		},
		selectTestCase{
			ds: bd.Order(goqu.C("a").Asc()).CompoundFromSelf(),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", bd.Order(goqu.C("a").Asc()).As("t1"))),
		},
		selectTestCase{
			ds: bd.As("alias").FromSelf(),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", bd.As("alias"))),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestJoin() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.Join(goqu.T("foo"), goqu.On(goqu.C("a").IsNull())),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				JoinsAppend(
					exp.NewConditionedJoinExpression(exp.InnerJoinType, goqu.T("foo"), goqu.On(goqu.C("a").IsNull())),
				),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestInnerJoin() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.InnerJoin(goqu.T("foo"), goqu.On(goqu.C("a").IsNull())),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				JoinsAppend(
					exp.NewConditionedJoinExpression(exp.InnerJoinType, goqu.T("foo"), goqu.On(goqu.C("a").IsNull())),
				),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestFullOuterJoin() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.FullOuterJoin(goqu.T("foo"), goqu.On(goqu.C("a").IsNull())),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				JoinsAppend(
					exp.NewConditionedJoinExpression(exp.FullOuterJoinType, goqu.T("foo"), goqu.On(goqu.C("a").IsNull())),
				),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestRightOuterJoin() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.RightOuterJoin(goqu.T("foo"), goqu.On(goqu.C("a").IsNull())),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				JoinsAppend(
					exp.NewConditionedJoinExpression(exp.RightOuterJoinType, goqu.T("foo"), goqu.On(goqu.C("a").IsNull())),
				),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestLeftOuterJoin() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.LeftOuterJoin(goqu.T("foo"), goqu.On(goqu.C("a").IsNull())),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				JoinsAppend(
					exp.NewConditionedJoinExpression(exp.LeftOuterJoinType, goqu.T("foo"), goqu.On(goqu.C("a").IsNull())),
				),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestFullJoin() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.FullJoin(goqu.T("foo"), goqu.On(goqu.C("a").IsNull())),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				JoinsAppend(
					exp.NewConditionedJoinExpression(exp.FullJoinType, goqu.T("foo"), goqu.On(goqu.C("a").IsNull())),
				),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestRightJoin() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.RightJoin(goqu.T("foo"), goqu.On(goqu.C("a").IsNull())),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				JoinsAppend(
					exp.NewConditionedJoinExpression(exp.RightJoinType, goqu.T("foo"), goqu.On(goqu.C("a").IsNull())),
				),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestLeftJoin() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.LeftJoin(goqu.T("foo"), goqu.On(goqu.C("a").IsNull())),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				JoinsAppend(
					exp.NewConditionedJoinExpression(exp.LeftJoinType, goqu.T("foo"), goqu.On(goqu.C("a").IsNull())),
				),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestNaturalJoin() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.NaturalJoin(goqu.T("foo")),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				JoinsAppend(
					exp.NewUnConditionedJoinExpression(exp.NaturalJoinType, goqu.T("foo")),
				),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestNaturalLeftJoin() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.NaturalLeftJoin(goqu.T("foo")),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				JoinsAppend(
					exp.NewUnConditionedJoinExpression(exp.NaturalLeftJoinType, goqu.T("foo")),
				),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestNaturalRightJoin() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.NaturalRightJoin(goqu.T("foo")),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				JoinsAppend(
					exp.NewUnConditionedJoinExpression(exp.NaturalRightJoinType, goqu.T("foo")),
				),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestNaturalFullJoin() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.NaturalFullJoin(goqu.T("foo")),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				JoinsAppend(
					exp.NewUnConditionedJoinExpression(exp.NaturalFullJoinType, goqu.T("foo")),
				),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestCrossJoin() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.CrossJoin(goqu.T("foo")),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				JoinsAppend(
					exp.NewUnConditionedJoinExpression(exp.CrossJoinType, goqu.T("foo")),
				),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestCustomJoin() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.CustomJoin(goqu.L("ARRAY JOIN tags").As("tag")),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				JoinsAppend(
					exp.NewUnConditionedJoinExpression(exp.CustomJoinType, goqu.L("ARRAY JOIN tags").As("tag")),
				),
		},
	)
}

func (sds *selectDatasetSuite) TestStraightJoin() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.StraightJoin(goqu.T("foo"), goqu.On(goqu.C("a").IsNull())),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				JoinsAppend(
					exp.NewConditionedJoinExpression(exp.StraightJoinType, goqu.T("foo"), goqu.On(goqu.C("a").IsNull())),
				),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestStraightJoin_unsupportedDialect() {
	_, _, err := goqu.From("test").StraightJoin(goqu.T("foo"), goqu.On(goqu.C("a").IsNull())).ToSQL()
	sds.EqualError(err, "goqu: dialect does not support StraightJoinType [dialect=default]")
}

func (sds *selectDatasetSuite) TestQualify() {
	w := exp.Ex{"a": 1}
	w2 := exp.Ex{"b": 2}
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.Qualify(w),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				QualifyAppend(w),
		},
		selectTestCase{
			ds: bd.Qualify(w).Qualify(w2),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				QualifyAppend(w).
				QualifyAppend(w2),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestQualify_notSupportedByBuiltInDialects() {
	for _, dialect := range []string{"default"} {
		_, _, err := goqu.Dialect(dialect).From("test").Qualify(goqu.C("a").Eq(1)).ToSQL()
		sds.Error(err, dialect)
		sds.Contains(err.Error(), "does not support QUALIFY clause", dialect)
	}
}

func (sds *selectDatasetSuite) TestWhere() {
	w := goqu.Ex{"a": 1}
	w2 := goqu.Ex{"b": "c"}
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.Where(w),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				WhereAppend(w),
		},
		selectTestCase{
			ds: bd.Where(w).Where(w2),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				WhereAppend(w).WhereAppend(w2),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestClearWhere() {
	w := goqu.Ex{"a": 1}
	bd := goqu.From("test").Where(w)
	sds.assertCases(
		selectTestCase{
			ds: bd.ClearWhere(),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")).WhereAppend(w),
		},
	)
}

func (sds *selectDatasetSuite) TestForUpdate() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.ForUpdate(goqu.NoWait),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetLock(exp.NewLock(exp.ForUpdate, goqu.NoWait)),
		},
		selectTestCase{
			ds: bd.ForUpdate(goqu.NoWait, goqu.T("table1")),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetLock(exp.NewLock(exp.ForUpdate, goqu.NoWait, goqu.T("table1"))),
		},
		selectTestCase{
			ds: bd.ForUpdate(goqu.NoWait, goqu.T("table1"), goqu.T("table2")),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetLock(exp.NewLock(exp.ForUpdate, goqu.NoWait, goqu.T("table1"), goqu.T("table2"))),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestForNoKeyUpdate() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.ForNoKeyUpdate(goqu.NoWait),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetLock(exp.NewLock(exp.ForNoKeyUpdate, goqu.NoWait)),
		},
		selectTestCase{
			ds: bd.ForNoKeyUpdate(goqu.NoWait, goqu.T("table1")),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetLock(exp.NewLock(exp.ForNoKeyUpdate, goqu.NoWait, goqu.T("table1"))),
		},
		selectTestCase{
			ds: bd.ForNoKeyUpdate(goqu.NoWait, goqu.T("table1"), goqu.T("table2")),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetLock(exp.NewLock(exp.ForNoKeyUpdate, goqu.NoWait, goqu.T("table1"), goqu.T("table2"))),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestForKeyShare() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.ForKeyShare(goqu.NoWait),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetLock(exp.NewLock(exp.ForKeyShare, goqu.NoWait)),
		},
		selectTestCase{
			ds: bd.ForKeyShare(goqu.NoWait, goqu.T("table1")),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetLock(exp.NewLock(exp.ForKeyShare, goqu.NoWait, goqu.T("table1"))),
		},
		selectTestCase{
			ds: bd.ForKeyShare(goqu.NoWait, goqu.T("table1"), goqu.T("table2")),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetLock(exp.NewLock(exp.ForKeyShare, goqu.NoWait, goqu.T("table1"), goqu.T("table2"))),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestForShare() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.ForShare(goqu.NoWait),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetLock(exp.NewLock(exp.ForShare, goqu.NoWait)),
		},
		selectTestCase{
			ds: bd.ForShare(goqu.NoWait, goqu.T("table1")),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetLock(exp.NewLock(exp.ForShare, goqu.NoWait, goqu.T("table1"))),
		},
		selectTestCase{
			ds: bd.ForShare(goqu.NoWait, goqu.T("table1"), goqu.T("table2")),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetLock(exp.NewLock(exp.ForShare, goqu.NoWait, goqu.T("table1"), goqu.T("table2"))),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestGroupBy() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.GroupBy("a"),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetGroupBy(exp.NewColumnListExpression(nil, "db", "a")),
		},
		selectTestCase{
			ds: bd.GroupBy("a").GroupBy("b"),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetGroupBy(exp.NewColumnListExpression(nil, "db", "b")),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestWindow() {
	w1 := goqu.W("w1").PartitionBy("a").OrderBy("b")
	w2 := goqu.W("w2").PartitionBy("a").OrderBy("b")

	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.Window(w1),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				WindowsAppend(w1),
		},
		selectTestCase{
			ds: bd.Window(w1).Window(w2),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				WindowsAppend(w2),
		},
		selectTestCase{
			ds: bd.Window(w1, w2),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				WindowsAppend(w1, w2),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestWindowAppend() {
	w1 := goqu.W("w1").PartitionBy("a").OrderBy("b")
	w2 := goqu.W("w2").PartitionBy("a").OrderBy("b")

	bd := goqu.From("test").Window(w1)
	sds.assertCases(
		selectTestCase{
			ds: bd.WindowAppend(w2),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				WindowsAppend(w1, w2),
		},
		selectTestCase{
			ds: bd,
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				WindowsAppend(w1),
		},
	)
}

func (sds *selectDatasetSuite) TestClearWindow() {
	w1 := goqu.W("w1").PartitionBy("a").OrderBy("b")

	bd := goqu.From("test").Window(w1)
	sds.assertCases(
		selectTestCase{
			ds:      bd.ClearWindow(),
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
		selectTestCase{
			ds: bd,
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				WindowsAppend(w1),
		},
	)
}

func (sds *selectDatasetSuite) TestEmptyConditionsInStatements() {
	var none []exp.Expression
	for want, ds := range map[string]exp.SQLExpression{
		`SELECT * FROM "t"`:                             goqu.From("t").Where(goqu.And(none...), goqu.Ex{}, goqu.L("")),
		`SELECT * FROM "t" WHERE ("a" = 1)`:             goqu.From("t").Where(goqu.L(""), goqu.C("a").Eq(1)),
		`SELECT * FROM "t" WHERE (1=0)`:                 goqu.From("t").Where(goqu.Or(none...)),
		`SELECT * FROM "t" WHERE (("a" = 1) AND (1=0))`: goqu.From("t").Where(goqu.C("a").Eq(1), goqu.ExOr{}),
		`SELECT * FROM "t" GROUP BY "a"`:                goqu.From("t").GroupBy("a").Having(goqu.L("")),
		`SELECT * FROM "t" GROUP BY "a" HAVING (1=0)`:   goqu.From("t").GroupBy("a").Having(goqu.Or()),
		`DELETE FROM "t" WHERE (1=0)`:                   goqu.Delete("t").Where(goqu.Or(none...)),
		`UPDATE "t" SET "a"=1 WHERE (1=0)`:              goqu.Update("t").Set(goqu.Record{"a": 1}).Where(goqu.ExOr{}),
		`SELECT CASE  WHEN (1=1) THEN 1 END FROM "t"`:   goqu.From("t").Select(goqu.Case().When(goqu.And(none...), 1)),
		`SELECT * FROM "t" WHERE NOT ((1=1))`:           goqu.From("t").Where(goqu.L("NOT (?)", goqu.And(none...))),
		`SELECT * FROM "t" WHERE (("a" = 1) OR (1=1))`:  goqu.From("t").Where(goqu.Or(goqu.C("a").Eq(1), goqu.And(none...))),
		`SELECT * FROM "t" WHERE (("b" = 2) AND (1=0))`: goqu.From("t").Where(goqu.C("b").Eq(2)).Where(goqu.Or()),
		`SELECT * FROM "t" INNER JOIN "u" ON (1=0)`:     goqu.From("t").Join(goqu.T("u"), goqu.On(goqu.Or())),
	} {
		sql, args, err := ds.ToSQL()
		sds.NoError(err, want)
		sds.Empty(args, want)
		sds.Equal(want, sql)
	}
}

func (sds *selectDatasetSuite) TestEmptyJoinConditionIsRequired() {
	_, _, err := goqu.From("t").Join(goqu.T("u"), goqu.On(goqu.L(""))).ToSQL()
	sds.EqualError(err, "goqu: join condition required for conditioned join InnerJoinType")
	sds.True(goqu.IsInvalidExpression(err))
}

func (sds *selectDatasetSuite) TestHaving() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.Having(goqu.C("a").Gt(1)),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				HavingAppend(goqu.C("a").Gt(1)),
		},
		selectTestCase{
			ds: bd.Having(goqu.C("a").Gt(1)).Having(goqu.Ex{"b": "c"}),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				HavingAppend(goqu.C("a").Gt(1)).HavingAppend(goqu.Ex{"b": "c"}),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestOrder() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.Order(goqu.C("a").Asc()),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetOrder(goqu.C("a").Asc()),
		},
		selectTestCase{
			ds: bd.Order(goqu.C("a").Asc()).Order(goqu.C("b").Asc()),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetOrder(goqu.C("b").Asc()),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestOrderAppend() {
	bd := goqu.From("test").Order(goqu.C("a").Asc())
	sds.assertCases(
		selectTestCase{
			ds: bd.OrderAppend(goqu.C("b").Asc()),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetOrder(goqu.C("a").Asc(), goqu.C("b").Asc()),
		},
		selectTestCase{
			ds: bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetOrder(goqu.C("a").Asc()),
		},
	)
}

func (sds *selectDatasetSuite) TestOrderPrepend() {
	bd := goqu.From("test").Order(goqu.C("a").Asc())
	sds.assertCases(
		selectTestCase{
			ds: bd.OrderPrepend(goqu.C("b").Asc()),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetOrder(goqu.C("b").Asc(), goqu.C("a").Asc()),
		},
		selectTestCase{
			ds: bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetOrder(goqu.C("a").Asc()),
		},
	)
}

func (sds *selectDatasetSuite) TestClearOrder() {
	bd := goqu.From("test").Order(goqu.C("a").Asc())
	sds.assertCases(
		selectTestCase{
			ds: bd.ClearOrder(),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
		selectTestCase{
			ds: bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetOrder(goqu.C("a").Asc()),
		},
	)
}

func (sds *selectDatasetSuite) TestLimit() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.Limit(10),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetLimit(uint(10)),
		},
		selectTestCase{
			ds: bd.Limit(0),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetLimit(uint(0)),
		},
		selectTestCase{
			ds: bd.Limit(10).Limit(2),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetLimit(uint(2)),
		},
		selectTestCase{
			ds: bd.Limit(10).Limit(0),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetLimit(uint(0)),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestLimitAll() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.LimitAll(),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetLimit(goqu.L("ALL")),
		},
		selectTestCase{
			ds: bd.Limit(10).LimitAll(),
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetLimit(goqu.L("ALL")),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestClearLimit() {
	bd := goqu.From("test").Limit(10)
	sds.assertCases(
		selectTestCase{
			ds:      bd.ClearLimit(),
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
		selectTestCase{
			ds: bd,
			clauses: exp.NewSelectClauses().
				SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetLimit(uint(10)),
		},
	)
}

func (sds *selectDatasetSuite) TestOffset() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds:      bd.Offset(10),
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")).SetOffset(10),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestClearOffset() {
	bd := goqu.From("test").Offset(10)
	sds.assertCases(
		selectTestCase{
			ds:      bd.ClearOffset(),
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")).SetOffset(10),
		},
	)
}

func (sds *selectDatasetSuite) TestUnion() {
	uds := goqu.From("union_test")
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.Union(uds),
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				CompoundsAppend(exp.NewCompoundExpression(exp.UnionCompoundType, uds)),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestUnionAll() {
	uds := goqu.From("union_test")
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.UnionAll(uds),
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				CompoundsAppend(exp.NewCompoundExpression(exp.UnionAllCompoundType, uds)),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestIntersect() {
	uds := goqu.From("union_test")
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.Intersect(uds),
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				CompoundsAppend(exp.NewCompoundExpression(exp.IntersectCompoundType, uds)),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestIntersectAll() {
	uds := goqu.From("union_test")
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.IntersectAll(uds),
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				CompoundsAppend(exp.NewCompoundExpression(exp.IntersectAllCompoundType, uds)),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestAs() {
	bd := goqu.From("test")
	sds.assertCases(
		selectTestCase{
			ds: bd.As("t"),
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")).
				SetAlias(goqu.T("t")),
		},
		selectTestCase{
			ds:      bd,
			clauses: exp.NewSelectClauses().SetFrom(exp.NewColumnListExpression(nil, "db", "test")),
		},
	)
}

func (sds *selectDatasetSuite) TestToSQL() {
	md := new(mocks.SQLDialect)
	ds := goqu.From("test").SetDialect(md)
	c := ds.GetClauses()
	sqlB := sb.NewSQLBuilder(false)
	md.On("ToSelectSQL", builderMatching(sqlB), c).Return(nil).Once()
	sql, args, err := ds.ToSQL()
	sds.Empty(sql)
	sds.Empty(args)
	sds.Nil(err)
	md.AssertExpectations(sds.T())
}

func (sds *selectDatasetSuite) TestToSQL_prepared() {
	md := new(mocks.SQLDialect)
	ds := goqu.From("test").Prepared(true).SetDialect(md)
	c := ds.GetClauses()
	sqlB := sb.NewSQLBuilder(true)
	md.On("ToSelectSQL", builderMatching(sqlB), c).Return(nil).Once()
	sql, args, err := ds.ToSQL()
	sds.Empty(sql)
	sds.Empty(args)
	sds.Nil(err)
	md.AssertExpectations(sds.T())
}

func (sds *selectDatasetSuite) TestToSQL_ReturnedError() {
	md := new(mocks.SQLDialect)
	ds := goqu.From("test").SetDialect(md)
	c := ds.GetClauses()
	sqlB := sb.NewSQLBuilder(false)
	ee := newTestError("expected error")
	md.On("ToSelectSQL", builderMatching(sqlB), c).Run(func(args mock.Arguments) {
		args.Get(0).(sb.SQLBuilder).SetError(ee)
	}).Once()

	sql, args, err := ds.ToSQL()
	sds.Empty(sql)
	sds.Empty(args)
	sds.Equal(ee, err)
	md.AssertExpectations(sds.T())
}

func (sds *selectDatasetSuite) TestAppendSQL() {
	md := new(mocks.SQLDialect)
	ds := goqu.From("test").SetDialect(md)
	c := ds.GetClauses()
	sqlB := sb.NewSQLBuilder(false)
	md.On("ToSelectSQL", builderMatching(sqlB), c).Return(nil).Once()
	ds.AppendSQL(sqlB)
	sds.NoError(sqlB.Error())
	md.AssertExpectations(sds.T())
}

func (sds *selectDatasetSuite) TestScanStructs() {
	mDB, sqlMock, err := sqlmock.New()
	sds.NoError(err)
	sqlMock.ExpectQuery(`SELECT "address", "name" FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).
			FromCSVString("111 Test Addr,Test1\n211 Test Addr,Test2"))

	sqlMock.ExpectQuery(`SELECT DISTINCT "name" FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).
			FromCSVString("111 Test Addr,Test1\n211 Test Addr,Test2"))
	sqlMock.ExpectQuery(`SELECT "test" FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"test"}).FromCSVString("test1\ntest2"))

	db := goqu.New("mock", mDB)
	var items []dsTestActionItem
	sds.NoError(db.From("items").ScanStructs(&items))
	sds.Equal([]dsTestActionItem{
		{Address: "111 Test Addr", Name: "Test1"},
		{Address: "211 Test Addr", Name: "Test2"},
	}, items)

	items = items[0:0]
	sds.NoError(db.From("items").Select("name").Distinct().ScanStructs(&items))
	sds.Equal([]dsTestActionItem{
		{Address: "111 Test Addr", Name: "Test1"},
		{Address: "211 Test Addr", Name: "Test2"},
	}, items)

	items = items[0:0]
	sds.EqualError(db.From("items").ScanStructs(items),
		"goqu: cannot scan into []goqu_test.dsTestActionItem, a pointer to a slice is required")
	sds.EqualError(db.From("items").ScanStructs(&dsTestActionItem{}),
		"goqu: cannot scan into *goqu_test.dsTestActionItem, a pointer to a slice is required")
	sds.EqualError(db.From("items").Select("test").ScanStructs(&items),
		`goqu: unable to find corresponding field to column "test" returned by query`)

	sds.True(goqu.IsNoDatabase(goqu.From("items").ScanStructs(items)))
}

func (sds *selectDatasetSuite) TestScanStructs_WithPreparedStatements() {
	mDB, sqlMock, err := sqlmock.New()
	sds.NoError(err)
	sqlMock.ExpectQuery(
		`SELECT "address", "name" FROM "items" WHERE \(\("address" = \?\) AND \("name" IN \(\?, \?, \?\)\)\)`,
	).
		WithArgs("111 Test Addr", "Bob", "Sally", "Billy").
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).
			FromCSVString("111 Test Addr,Test1\n211 Test Addr,Test2"))

	sqlMock.ExpectQuery(
		`SELECT "test" FROM "items" WHERE \(\("address" = \?\) AND \("name" IN \(\?, \?, \?\)\)\)`,
	).
		WithArgs("111 Test Addr", "Bob", "Sally", "Billy").
		WillReturnRows(sqlmock.NewRows([]string{"test"}).FromCSVString("test1\ntest2"))

	db := goqu.New("mock", mDB)
	var items []dsTestActionItem
	sds.NoError(db.From("items").Prepared(true).Where(goqu.Ex{
		"name":    []string{"Bob", "Sally", "Billy"},
		"address": "111 Test Addr",
	}).ScanStructs(&items))
	sds.Equal(items, []dsTestActionItem{
		{Address: "111 Test Addr", Name: "Test1"},
		{Address: "211 Test Addr", Name: "Test2"},
	})

	items = items[0:0]
	sds.EqualError(db.From("items").ScanStructs(items),
		"goqu: cannot scan into []goqu_test.dsTestActionItem, a pointer to a slice is required")
	sds.EqualError(db.From("items").ScanStructs(&dsTestActionItem{}),
		"goqu: cannot scan into *goqu_test.dsTestActionItem, a pointer to a slice is required")
	unmappedErr := db.From("items").
		Prepared(true).
		Select("test").
		Where(goqu.Ex{"name": []string{"Bob", "Sally", "Billy"}, "address": "111 Test Addr"}).
		ScanStructs(&items)
	sds.EqualError(unmappedErr, `goqu: unable to find corresponding field to column "test" returned by query`)
	sds.True(goqu.IsColumnMismatch(unmappedErr))
	sds.False(goqu.IsUnsupportedScanType(unmappedErr))
}

func (sds *selectDatasetSuite) TestStructScans_rejectsTargetsThatAreNotRows() {
	mDB, _, err := sqlmock.New()
	sds.Require().NoError(err)
	db := goqu.New("mock", mDB)
	var nullStrings []dbsql.NullString
	var times []time.Time
	var names []string
	var nullString dbsql.NullString
	var addr netip.Addr
	var user *dsTestActionItem
	var embedRows []dsUnexportedEmbedRow
	var count int
	for name, scan := range map[string]func() error{
		"[]sql.NullString":       func() error { return db.From("items").ScanStructs(&nullStrings) },
		"[]time.Time":            func() error { return db.From("items").ScanStructs(&times) },
		"[]string":               func() error { return db.From("items").ScanStructs(&names) },
		"[]string value":         func() error { return db.From("items").ScanStructs([]string{}) },
		"*sql.NullString":        func() error { _, err := db.From("items").ScanStruct(&nullString); return err },
		"*netip.Addr":            func() error { _, err := db.From("items").ScanStruct(&addr); return err },
		"**T":                    func() error { _, err := db.From("items").ScanStruct(&user); return err },
		"struct value":           func() error { _, err := db.From("items").ScanStruct(dsTestActionItem{}); return err },
		"auto-selected embed []": func() error { return db.From("items").ScanStructs(&embedRows) },
		"*int ScanStructs":       func() error { return db.From("items").ScanStructs(&count) },
		"nil ScanStructs":        func() error { return db.From("items").ScanStructs(nil) },
	} {
		var scanErr error
		sds.NotPanics(func() { scanErr = scan() }, name)
		sds.True(goqu.IsUnsupportedScanType(scanErr), "%s: %v", name, scanErr)
	}
}

func (sds *selectDatasetSuite) TestStructScans_rejectedTargetLeavesTheDatasetUsable() {
	mDB, sqlMock, err := sqlmock.New()
	sds.Require().NoError(err)
	ds := goqu.New("mock", mDB).From("items")
	var nullStrings []dbsql.NullString
	sds.True(goqu.IsUnsupportedScanType(ds.ScanStructs(&nullStrings)))

	sql, _, err := ds.ToSQL()
	sds.NoError(err)
	sds.Equal(`SELECT * FROM "items"`, sql)
	sqlMock.ExpectQuery(`SELECT "address", "name" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).AddRow("111 Test Addr", "Test1"))
	var items []dsTestActionItem
	sds.NoError(ds.ScanStructs(&items))
	sds.Len(items, 1)
}

func (sds *selectDatasetSuite) TestStructScans_acceptsCallerFilledUnexportedEmbeds() {
	mDB, sqlMock, err := sqlmock.New()
	sds.Require().NoError(err)
	db := goqu.New("mock", mDB)

	sqlMock.ExpectQuery(`SELECT "id", "x" FROM "items" LIMIT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "x"}).AddRow(1, 2))
	row := dsUnexportedEmbedRow{dsHiddenColumns: &dsHiddenColumns{}}
	found, err := db.From("items").ScanStruct(&row)
	sds.Require().NoError(err)
	sds.True(found)
	sds.Equal(dsUnexportedEmbedRow{dsHiddenColumns: &dsHiddenColumns{X: 2}, ID: 1}, row)

	sqlMock.ExpectQuery(`SELECT "id" FROM "items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	var rows []dsUnexportedEmbedRow
	sds.Require().NoError(db.From("items").Select("id").ScanStructs(&rows))
	sds.Equal([]dsUnexportedEmbedRow{{ID: 3}}, rows)
}

func (sds *selectDatasetSuite) TestScanStruct() {
	mDB, sqlMock, err := sqlmock.New()
	sds.NoError(err)
	sqlMock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).FromCSVString("111 Test Addr,Test1"))

	sqlMock.ExpectQuery(`SELECT DISTINCT "name" FROM "items" LIMIT 1`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).FromCSVString("111 Test Addr,Test1"))

	sqlMock.ExpectQuery(`SELECT "test" FROM "items" LIMIT 1`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"test"}).FromCSVString("test1\ntest2"))

	db := goqu.New("mock", mDB)
	var item dsTestActionItem
	found, err := db.From("items").ScanStruct(&item)
	sds.NoError(err)
	sds.True(found)
	sds.Equal("111 Test Addr", item.Address)
	sds.Equal("Test1", item.Name)

	item = dsTestActionItem{}
	found, err = db.From("items").Select("name").Distinct().ScanStruct(&item)
	sds.NoError(err)
	sds.True(found)
	sds.Equal("111 Test Addr", item.Address)
	sds.Equal("Test1", item.Name)

	_, err = db.From("items").ScanStruct(item)
	sds.EqualError(err, "goqu: cannot scan into goqu_test.dsTestActionItem, a pointer to a struct is required")
	_, err = db.From("items").ScanStruct([]dsTestActionItem{})
	sds.EqualError(err, "goqu: cannot scan into []goqu_test.dsTestActionItem, a pointer to a struct is required")
	_, err = db.From("items").Select("test").ScanStruct(&item)
	sds.EqualError(err, `goqu: unable to find corresponding field to column "test" returned by query`)
	sds.True(goqu.IsColumnMismatch(err))
	sds.False(goqu.IsUnsupportedScanType(err))

	_, err = goqu.From("items").ScanStruct(item)
	sds.True(goqu.IsNoDatabase(err))
}

func (sds *selectDatasetSuite) TestScanStruct_WithPreparedStatements() {
	mDB, sqlMock, err := sqlmock.New()
	sds.NoError(err)
	sqlMock.ExpectQuery(
		`SELECT "address", "name" FROM "items" WHERE \(\("address" = \?\) AND \("name" IN \(\?, \?, \?\)\)\) LIMIT \?`,
	).
		WithArgs("111 Test Addr", "Bob", "Sally", "Billy", 1).
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).FromCSVString("111 Test Addr,Test1"))

	sqlMock.ExpectQuery(`SELECT "test" FROM "items" WHERE \(\("address" = \?\) AND \("name" IN \(\?, \?, \?\)\)\) LIMIT \?`).
		WithArgs("111 Test Addr", "Bob", "Sally", "Billy", 1).
		WillReturnRows(sqlmock.NewRows([]string{"test"}).FromCSVString("test1\ntest2"))

	db := goqu.New("mock", mDB)
	var item dsTestActionItem
	found, err := db.From("items").Prepared(true).Where(goqu.Ex{
		"name":    []string{"Bob", "Sally", "Billy"},
		"address": "111 Test Addr",
	}).ScanStruct(&item)
	sds.NoError(err)
	sds.True(found)
	sds.Equal("111 Test Addr", item.Address)
	sds.Equal("Test1", item.Name)

	_, err = db.From("items").ScanStruct(item)
	sds.EqualError(err, "goqu: cannot scan into goqu_test.dsTestActionItem, a pointer to a struct is required")
	_, err = db.From("items").ScanStruct([]dsTestActionItem{})
	sds.EqualError(err, "goqu: cannot scan into []goqu_test.dsTestActionItem, a pointer to a struct is required")
	_, err = db.From("items").
		Prepared(true).
		Select("test").
		Where(goqu.Ex{"name": []string{"Bob", "Sally", "Billy"}, "address": "111 Test Addr"}).
		ScanStruct(&item)
	sds.EqualError(err, `goqu: unable to find corresponding field to column "test" returned by query`)
}

func (sds *selectDatasetSuite) TestScanStructSubqueries() {
	mDB, sqlMock, err := sqlmock.New()
	sds.NoError(err)
	sqlMock.ExpectQuery(`SELECT \("address" IS NOT NULL\) AS "withAddress", \("name" IS NOT NULL\) AS "withName" FROM "items" LIMIT 1`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"withAddress", "withName"}).FromCSVString("1,1"))

	sqlMock.ExpectQuery(`SELECT "test" FROM "items" LIMIT 1`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"test"}).FromCSVString("test1\ntest2"))

	db := goqu.New("mock", mDB)
	var item dsSubQueryTestActionItem
	found, err := db.From("items").SubQueries(map[string]exp.Aliaseable{"sub": goqu.L("(\"name\" IS NOT NULL)")}).ScanStruct(&item)
	sds.NoError(err)
	sds.True(found)
	sds.Equal(true, item.WithAddress)
	sds.Equal(true, item.WithName)

	_, err = db.From("items").ScanStruct(item)
	sds.EqualError(err, "goqu: cannot scan into goqu_test.dsSubQueryTestActionItem, a pointer to a struct is required")
	_, err = db.From("items").ScanStruct([]dsTestActionItem{})
	sds.EqualError(err, "goqu: cannot scan into []goqu_test.dsTestActionItem, a pointer to a struct is required")
	_, err = db.From("items").Select("test").ScanStruct(&item)
	sds.EqualError(err, `goqu: unable to find corresponding field to column "test" returned by query`)
	sds.True(goqu.IsColumnMismatch(err))
	sds.False(goqu.IsUnsupportedScanType(err))

	_, err = goqu.From("items").ScanStruct(item)
	sds.True(goqu.IsNoDatabase(err))
}

func (sds *selectDatasetSuite) TestScanStructUntagged() {
	defer goqu.SetIgnoreUntaggedFields(false)

	mDB, sqlMock, err := sqlmock.New()
	sds.NoError(err)
	sqlMock.ExpectQuery(`SELECT "address", "name", "untagged" FROM "items" LIMIT 1`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"address", "name", "untagged"}).FromCSVString("111 Test Addr,Test1,Test2"))

	sqlMock.ExpectQuery(`SELECT "address", "name" FROM "items" LIMIT 1`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"address", "name"}).FromCSVString("111 Test Addr,Test1"))

	db := goqu.New("mock", mDB)
	var item dsUntaggedTestActionItem

	found, err := db.From("items").ScanStruct(&item)
	sds.NoError(err)
	sds.True(found)
	sds.Equal("111 Test Addr", item.Address)
	sds.Equal("Test1", item.Name)
	sds.Equal("Test2", item.Untagged)

	// Ignore untagged fields, which will suppress the "untagged" column
	goqu.SetIgnoreUntaggedFields(true)

	item = dsUntaggedTestActionItem{}
	found, err = db.From("items").ScanStruct(&item)
	sds.NoError(err)
	sds.True(found)
	sds.Equal("111 Test Addr", item.Address)
	sds.Equal("Test1", item.Name)
	sds.Equal("", item.Untagged)
}

func (sds *selectDatasetSuite) TestScanVals() {
	mDB, sqlMock, err := sqlmock.New()
	sds.NoError(err)
	sqlMock.ExpectQuery(`SELECT "id" FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2\n3\n4\n5"))

	db := goqu.New("mock", mDB)
	var ids []uint32
	sds.NoError(db.From("items").Select("id").ScanVals(&ids))
	sds.Equal(ids, []uint32{1, 2, 3, 4, 5})

	sds.EqualError(db.From("items").ScanVals([]uint32{}),
		"goqu: cannot scan into []uint32, a pointer to a slice is required")
	sds.EqualError(db.From("items").ScanVals(dsTestActionItem{}),
		"goqu: cannot scan into goqu_test.dsTestActionItem, a pointer to a slice is required")

	err = goqu.From("items").ScanVals(&ids)
	sds.True(goqu.IsNoDatabase(err))
}

func (sds *selectDatasetSuite) TestScanVals_WithPreparedStatment() {
	mDB, sqlMock, err := sqlmock.New()
	sds.NoError(err)
	sqlMock.ExpectQuery(
		`SELECT "id" FROM "items" WHERE \(\("address" = \?\) AND \("name" IN \(\?, \?, \?\)\)\)`,
	).
		WithArgs("111 Test Addr", "Bob", "Sally", "Billy").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("1\n2\n3\n4\n5"))

	db := goqu.New("mock", mDB)
	var ids []uint32
	sds.NoError(db.From("items").
		Prepared(true).
		Select("id").
		Where(goqu.Ex{"name": []string{"Bob", "Sally", "Billy"}, "address": "111 Test Addr"}).
		ScanVals(&ids))
	sds.Equal([]uint32{1, 2, 3, 4, 5}, ids)

	sds.EqualError(db.From("items").ScanVals([]uint32{}),
		"goqu: cannot scan into []uint32, a pointer to a slice is required")

	sds.EqualError(db.From("items").ScanVals(dsTestActionItem{}),
		"goqu: cannot scan into goqu_test.dsTestActionItem, a pointer to a slice is required")
}

func (sds *selectDatasetSuite) TestScanVal() {
	mDB, sqlMock, err := sqlmock.New()
	sds.NoError(err)
	sqlMock.ExpectQuery(`SELECT "id" FROM "items" LIMIT 1`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("10"))

	db := goqu.New("mock", mDB)
	var id int64
	found, err := db.From("items").Select("id").ScanVal(&id)
	sds.NoError(err)
	sds.Equal(id, int64(10))
	sds.True(found)

	found, err = db.From("items").ScanVal([]int64{})
	sds.False(found)
	sds.EqualError(err, "goqu: cannot scan into []int64, a pointer is required")
	found, err = db.From("items").ScanVal(10)
	sds.False(found)
	sds.EqualError(err, "goqu: cannot scan into int, a pointer is required")

	_, err = goqu.From("items").ScanVal(&id)
	sds.True(goqu.IsNoDatabase(err))
}

func (sds *selectDatasetSuite) TestScanVal_WithPreparedStatement() {
	mDB, sqlMock, err := sqlmock.New()
	sds.NoError(err)
	sqlMock.ExpectQuery(
		`SELECT "id" FROM "items" WHERE \(\("address" = \?\) AND \("name" IN \(\?, \?, \?\)\)\) LIMIT ?`,
	).
		WithArgs("111 Test Addr", "Bob", "Sally", "Billy", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).FromCSVString("10"))

	db := goqu.New("mock", mDB)
	var id int64
	found, err := db.From("items").
		Prepared(true).
		Select("id").
		Where(goqu.Ex{"name": []string{"Bob", "Sally", "Billy"}, "address": "111 Test Addr"}).
		ScanVal(&id)
	sds.NoError(err)
	sds.Equal(int64(10), id)
	sds.True(found)

	found, err = db.From("items").ScanVal([]int64{})
	sds.False(found)
	sds.EqualError(err, "goqu: cannot scan into []int64, a pointer is required")
	found, err = db.From("items").ScanVal(10)
	sds.False(found)
	sds.EqualError(err, "goqu: cannot scan into int, a pointer is required")
}

func (sds *selectDatasetSuite) TestCount() {
	mDB, sqlMock, err := sqlmock.New()
	sds.NoError(err)
	sqlMock.ExpectQuery(`SELECT COUNT\(\*\) AS "count" FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"count"}).FromCSVString("10"))

	db := goqu.New("mock", mDB)
	count, err := db.From("items").Count()
	sds.NoError(err)
	sds.Equal(count, int64(10))
}

func (sds *selectDatasetSuite) TestCount_WithPreparedStatement() {
	mDB, sqlMock, err := sqlmock.New()
	sds.NoError(err)
	sqlMock.ExpectQuery(
		`SELECT COUNT\(\*\) AS "count" FROM "items" WHERE \(\("address" = \?\) AND \("name" IN \(\?, \?, \?\)\)\)`,
	).
		WithArgs("111 Test Addr", "Bob", "Sally", "Billy", 1).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).FromCSVString("10"))

	ds := goqu.New("mock", mDB)
	count, err := ds.From("items").
		Prepared(true).
		Where(goqu.Ex{"name": []string{"Bob", "Sally", "Billy"}, "address": "111 Test Addr"}).
		Count()
	sds.NoError(err)
	sds.Equal(int64(10), count)
}

func (sds *selectDatasetSuite) TestExists() {
	mDB, sqlMock, err := sqlmock.New()
	sds.NoError(err)
	sqlMock.ExpectQuery(`SELECT CASE WHEN EXISTS \(SELECT \* FROM "items"\) THEN 1 ELSE 0 END LIMIT 1`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).FromCSVString("1"))

	db := goqu.New("mock", mDB)
	exists, err := db.From("items").Exists()
	sds.NoError(err)
	sds.True(exists)
}

func (sds *selectDatasetSuite) TestExists_WithPreparedStatement() {
	mDB, sqlMock, err := sqlmock.New()
	sds.NoError(err)
	sqlMock.ExpectQuery(
		`SELECT CASE WHEN EXISTS \(SELECT \* FROM "items" WHERE \("address" = \?\)\) THEN 1 ELSE 0 END LIMIT \?`,
	).
		WithArgs("111 Test Addr", 1).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).FromCSVString("true"))

	ds := goqu.New("mock", mDB)
	exists, err := ds.From("items").
		Prepared(true).
		Where(goqu.Ex{"address": "111 Test Addr"}).
		Exists()
	sds.NoError(err)
	sds.True(exists)
}

func (sds *selectDatasetSuite) TestPluck() {
	mDB, sqlMock, err := sqlmock.New()
	sds.NoError(err)
	sqlMock.ExpectQuery(`SELECT "name" FROM "items"`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"name"}).FromCSVString("test1\ntest2\ntest3\ntest4\ntest5"))

	db := goqu.New("mock", mDB)
	var names []string
	sds.NoError(db.From("items").Pluck(&names, "name"))
	sds.Equal([]string{"test1", "test2", "test3", "test4", "test5"}, names)
}

func (sds *selectDatasetSuite) TestPluck_WithPreparedStatement() {
	mDB, sqlMock, err := sqlmock.New()
	sds.NoError(err)
	sqlMock.ExpectQuery(
		`SELECT "name" FROM "items" WHERE \(\("address" = \?\) AND \("name" IN \(\?, \?, \?\)\)\)`,
	).
		WithArgs("111 Test Addr", "Bob", "Sally", "Billy").
		WillReturnRows(sqlmock.NewRows([]string{"name"}).FromCSVString("Bob\nSally\nBilly"))

	db := goqu.New("mock", mDB)
	var names []string
	sds.NoError(db.From("items").
		Prepared(true).
		Where(goqu.Ex{"name": []string{"Bob", "Sally", "Billy"}, "address": "111 Test Addr"}).
		Pluck(&names, "name"))
	sds.Equal([]string{"Bob", "Sally", "Billy"}, names)
}

func (sds *selectDatasetSuite) TestSetError() {
	err1 := newTestError("error #1")
	err2 := newTestError("error #2")
	err3 := newTestError("error #3")

	md := new(mocks.SQLDialect)
	ds := goqu.From("test").SetDialect(md)
	ds = ds.SetError(err1)
	sds.Equal(err1, ds.Error())
	sql, args, err := ds.ToSQL()
	sds.Empty(sql)
	sds.Empty(args)
	sds.Equal(err1, err)

	// Repeated SetError calls on Dataset should not overwrite the original error
	ds = ds.SetError(err2)
	sds.Equal(err1, ds.Error())
	sql, args, err = ds.ToSQL()
	sds.Empty(sql)
	sds.Empty(args)
	sds.Equal(err1, err)

	// Builder functions should not lose the error
	ds = ds.ClearWindow()
	sds.Equal(err1, ds.Error())
	sql, args, err = ds.ToSQL()
	sds.Empty(sql)
	sds.Empty(args)
	sds.Equal(err1, err)

	// Deeper errors inside SQL generation should still return original error
	c := ds.GetClauses()
	sqlB := sb.NewSQLBuilder(false)
	md.On("ToInsertSQL", builderMatching(sqlB), c).Run(func(args mock.Arguments) {
		args.Get(0).(sb.SQLBuilder).SetError(err3)
	}).Once()

	sql, args, err = ds.ToSQL()
	sds.Empty(sql)
	sds.Empty(args)
	sds.Equal(err1, err)
}

func (sds *selectDatasetSuite) TestLimitZero() {
	for _, c := range []struct {
		ds   *goqu.SelectDataset
		sql  string
		args []interface{}
	}{
		{ds: goqu.From("t").Limit(0), sql: `SELECT * FROM "t" LIMIT 0`},
		{ds: goqu.From("t").Prepared(true).Limit(0), sql: `SELECT * FROM "t" LIMIT ?`, args: []interface{}{int64(0)}},
		{ds: goqu.From("t").Order(goqu.C("a").Asc()).Limit(0).Offset(2), sql: `SELECT * FROM "t" ORDER BY "a" ASC LIMIT 0 OFFSET 2`},
		{ds: goqu.Dialect("postgres").From("t").Limit(0), sql: `SELECT * FROM "t" LIMIT 0`},
	} {
		sql, args, err := c.ds.ToSQL()
		sds.NoError(err, c.sql)
		sds.Equal(c.sql, sql)
		if c.args != nil {
			sds.Equal(c.args, args, c.sql)
		} else {
			sds.Empty(args, c.sql)
		}
	}
}

func TestSelectDataset(t *testing.T) {
	suite.Run(t, new(selectDatasetSuite))
}
