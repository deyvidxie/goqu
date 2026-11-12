package exp_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/stretchr/testify/suite"
)

type sqlFunctionExpressionSuite struct {
	suite.Suite
	fn exp.SQLFunctionExpression
}

func TestSQLFunctionExpressionSuite(t *testing.T) {
	suite.Run(t, &sqlFunctionExpressionSuite{
		fn: exp.NewSQLFunctionExpression("COUNT", exp.Star()),
	})
}

func (sfes *sqlFunctionExpressionSuite) TestClone() {
	sfes.Equal(sfes.fn, sfes.fn.Clone())
}

func (sfes *sqlFunctionExpressionSuite) TestExpression() {
	sfes.Equal(sfes.fn, sfes.fn.Expression())
}

func (sfes *sqlFunctionExpressionSuite) TestArgs() {
	sfes.Equal([]interface{}{exp.Star()}, sfes.fn.Args())
}

func (sfes *sqlFunctionExpressionSuite) TestName() {
	sfes.Equal("COUNT", sfes.fn.Name())
}

func (sfes *sqlFunctionExpressionSuite) TestOrderBy() {
	ts := exp.NewIdentifierExpression("", "", "ts")
	ordered := sfes.fn.OrderBy(ts.Desc(), ts.Asc())

	sfes.False(sfes.fn.HasOrder())
	sfes.Nil(sfes.fn.OrderCols())
	sfes.True(ordered.HasOrder())
	sfes.Equal(exp.NewColumnListExpression(nil, "", ts.Desc(), ts.Asc()), ordered.OrderCols())
	sfes.Equal(ordered, ordered.Clone())
	sfes.Equal(sfes.fn.Args(), ordered.Args())

	for name, fn := range map[string]exp.SQLFunctionExpression{
		"no expressions": sfes.fn.OrderBy(),
		"nil expression": sfes.fn.OrderBy(ts.Asc(), nil),
	} {
		cols := fn.OrderCols().Columns()
		invalid, ok := cols[len(cols)-1].(exp.InvalidExpression)
		sfes.Require().True(ok, name)
		sfes.Error(invalid.Err(), name)
	}
	sfes.EqualError(sfes.fn.OrderBy().OrderCols().Columns()[0].(exp.InvalidExpression).Err(), "goqu: OrderBy requires at least one expression")
}

func (sfes *sqlFunctionExpressionSuite) TestWithinGroup() {
	ts := exp.NewIdentifierExpression("", "", "ts")
	grouped := sfes.fn.WithinGroup(ts.Desc(), ts.Asc())

	sfes.False(sfes.fn.HasWithinGroup())
	sfes.Nil(sfes.fn.WithinGroupCols())
	sfes.True(grouped.HasWithinGroup())
	sfes.False(grouped.HasOrder())
	sfes.Equal(exp.NewOrderedColumnList(ts.Desc(), ts.Asc()), grouped.WithinGroupCols())
	sfes.Equal(grouped, grouped.Clone())
	sfes.Equal(sfes.fn.Args(), grouped.Args())
	sfes.EqualError(sfes.fn.WithinGroup().WithinGroupCols().Columns()[0].(exp.InvalidExpression).Err(),
		"goqu: WithinGroup requires at least one expression")
	sfes.EqualError(sfes.fn.WithinGroup(nil).WithinGroupCols().Columns()[0].(exp.InvalidExpression).Err(),
		"goqu: Order expressions must not be nil")
}

func (sfes *sqlFunctionExpressionSuite) TestAllOthers() {
	fn := sfes.fn

	rv := exp.NewRangeVal(1, 2)
	pattern := "func like%"
	inVals := []interface{}{1, 2}
	testCases := []struct {
		Ex       exp.Expression
		Expected exp.Expression
	}{
		{Ex: fn.As("a"), Expected: exp.NewAliasExpression(fn, "a")},
		{Ex: fn.Eq(1), Expected: exp.NewBooleanExpression(exp.EqOp, fn, 1)},
		{Ex: fn.Neq(1), Expected: exp.NewBooleanExpression(exp.NeqOp, fn, 1)},
		{Ex: fn.Gt(1), Expected: exp.NewBooleanExpression(exp.GtOp, fn, 1)},
		{Ex: fn.Gte(1), Expected: exp.NewBooleanExpression(exp.GteOp, fn, 1)},
		{Ex: fn.Lt(1), Expected: exp.NewBooleanExpression(exp.LtOp, fn, 1)},
		{Ex: fn.Lte(1), Expected: exp.NewBooleanExpression(exp.LteOp, fn, 1)},
		{Ex: fn.Between(rv), Expected: exp.NewRangeExpression(exp.BetweenOp, fn, rv)},
		{Ex: fn.NotBetween(rv), Expected: exp.NewRangeExpression(exp.NotBetweenOp, fn, rv)},
		{Ex: fn.Like(pattern), Expected: exp.NewBooleanExpression(exp.LikeOp, fn, pattern)},
		{Ex: fn.NotLike(pattern), Expected: exp.NewBooleanExpression(exp.NotLikeOp, fn, pattern)},
		{Ex: fn.ILike(pattern), Expected: exp.NewBooleanExpression(exp.ILikeOp, fn, pattern)},
		{Ex: fn.NotILike(pattern), Expected: exp.NewBooleanExpression(exp.NotILikeOp, fn, pattern)},
		{Ex: fn.RegexpLike(pattern), Expected: exp.NewBooleanExpression(exp.RegexpLikeOp, fn, pattern)},
		{Ex: fn.RegexpNotLike(pattern), Expected: exp.NewBooleanExpression(exp.RegexpNotLikeOp, fn, pattern)},
		{Ex: fn.RegexpILike(pattern), Expected: exp.NewBooleanExpression(exp.RegexpILikeOp, fn, pattern)},
		{Ex: fn.RegexpNotILike(pattern), Expected: exp.NewBooleanExpression(exp.RegexpNotILikeOp, fn, pattern)},
		{Ex: fn.In(inVals), Expected: exp.NewBooleanExpression(exp.InOp, fn, inVals)},
		{Ex: fn.NotIn(inVals), Expected: exp.NewBooleanExpression(exp.NotInOp, fn, inVals)},
		{Ex: fn.Is(true), Expected: exp.NewBooleanExpression(exp.IsOp, fn, true)},
		{Ex: fn.IsNot(true), Expected: exp.NewBooleanExpression(exp.IsNotOp, fn, true)},
		{Ex: fn.IsNull(), Expected: exp.NewBooleanExpression(exp.IsOp, fn, nil)},
		{Ex: fn.IsNotNull(), Expected: exp.NewBooleanExpression(exp.IsNotOp, fn, nil)},
		{Ex: fn.IsTrue(), Expected: exp.NewBooleanExpression(exp.IsOp, fn, true)},
		{Ex: fn.IsNotTrue(), Expected: exp.NewBooleanExpression(exp.IsNotOp, fn, true)},
		{Ex: fn.IsFalse(), Expected: exp.NewBooleanExpression(exp.IsOp, fn, false)},
		{Ex: fn.IsNotFalse(), Expected: exp.NewBooleanExpression(exp.IsNotOp, fn, false)},
		{Ex: fn.Desc(), Expected: exp.NewOrderedExpression(fn, exp.DescSortDir, exp.NoNullsSortType)},
		{Ex: fn.Asc(), Expected: exp.NewOrderedExpression(fn, exp.AscDir, exp.NoNullsSortType)},
		{Ex: fn.Add(1), Expected: exp.NewArithmeticExpression(exp.ArithmeticAddOp, fn, 1)},
		{Ex: fn.Sub(1), Expected: exp.NewArithmeticExpression(exp.ArithmeticSubOp, fn, 1)},
		{Ex: fn.Mul(1), Expected: exp.NewArithmeticExpression(exp.ArithmeticMulOp, fn, 1)},
		{Ex: fn.Div(1), Expected: exp.NewArithmeticExpression(exp.ArithmeticDivOp, fn, 1)},
	}

	for _, tc := range testCases {
		sfes.Equal(tc.Expected, tc.Ex)
	}
}
