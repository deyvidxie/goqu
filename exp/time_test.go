package exp_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/stretchr/testify/assert"
)

func TestNewTimeFunctionExpressions(t *testing.T) {
	now := exp.NewNowExpression(6)
	assert.Equal(t, exp.NowFunction, now.Function())
	assert.Equal(t, []int{6}, now.FSP())
	assert.Equal(t, now, now.Expression())
	assert.Equal(t, now, now.Clone())

	fsp := []int{3}
	shared := exp.NewUTCTimestampExpression(fsp...)
	fsp[0] = 9
	assert.Equal(t, []int{3}, shared.FSP())

	utc := exp.NewUTCTimestampExpression()
	assert.Equal(t, exp.UTCTimestampFunction, utc.Function())
	assert.Empty(t, utc.FSP())

	interval := exp.NewIntervalExpression(1, exp.SecondInterval)
	assert.Equal(t, 1, interval.Count())
	assert.Equal(t, exp.SecondInterval, interval.Unit())
	assert.Equal(t, interval, interval.Expression())
	assert.Equal(t, interval, interval.Clone())
}

func TestTimeFunctionExpressionOperators(t *testing.T) {
	e := exp.NewNowExpression()
	assert.Equal(t, exp.NewAliasExpression(e, "a"), e.As("a"))
	assert.Equal(t, exp.NewBooleanExpression(exp.EqOp, e, 1), e.Eq(1))
	assert.Equal(t, exp.NewBooleanExpression(exp.NeqOp, e, 1), e.Neq(1))
	assert.Equal(t, exp.NewBooleanExpression(exp.GtOp, e, 1), e.Gt(1))
	assert.Equal(t, exp.NewBooleanExpression(exp.GteOp, e, 1), e.Gte(1))
	assert.Equal(t, exp.NewBooleanExpression(exp.LtOp, e, 1), e.Lt(1))
	assert.Equal(t, exp.NewBooleanExpression(exp.LteOp, e, 1), e.Lte(1))
	assert.Equal(t, exp.NewArithmeticExpression(exp.ArithmeticAddOp, e, 1), e.Add(1))
	assert.Equal(t, exp.NewArithmeticExpression(exp.ArithmeticSubOp, e, 1), e.Sub(1))
	assert.Equal(t, exp.NewArithmeticExpression(exp.ArithmeticMulOp, e, 1), e.Mul(1))
	assert.Equal(t, exp.NewArithmeticExpression(exp.ArithmeticDivOp, e, 1), e.Div(1))
	assert.Equal(t, exp.NewRangeExpression(exp.BetweenOp, e, exp.NewRangeVal(1, 2)), e.Between(exp.NewRangeVal(1, 2)))
	assert.Equal(t, exp.NewRangeExpression(exp.NotBetweenOp, e, exp.NewRangeVal(1, 2)), e.NotBetween(exp.NewRangeVal(1, 2)))
	assert.Equal(t, exp.NewBooleanExpression(exp.InOp, e, []any{1, 2}), e.In(1, 2))
	assert.Equal(t, exp.NewBooleanExpression(exp.NotInOp, e, []any{1, 2}), e.NotIn(1, 2))
	assert.Equal(t, exp.NewBooleanExpression(exp.IsOp, e, nil), e.Is(nil))
	assert.Equal(t, exp.NewBooleanExpression(exp.IsNotOp, e, nil), e.IsNot(nil))
	assert.Equal(t, exp.NewBooleanExpression(exp.IsOp, e, nil), e.IsNull())
	assert.Equal(t, exp.NewBooleanExpression(exp.IsNotOp, e, nil), e.IsNotNull())
	assert.Equal(t, exp.NewBooleanExpression(exp.IsOp, e, true), e.IsTrue())
	assert.Equal(t, exp.NewBooleanExpression(exp.IsNotOp, e, true), e.IsNotTrue())
	assert.Equal(t, exp.NewBooleanExpression(exp.IsOp, e, false), e.IsFalse())
	assert.Equal(t, exp.NewBooleanExpression(exp.IsNotOp, e, false), e.IsNotFalse())
	assert.Equal(t, exp.NewOrderedExpression(e, exp.AscDir, exp.NoNullsSortType), e.Asc())
	assert.Equal(t, exp.NewOrderedExpression(e, exp.DescSortDir, exp.NoNullsSortType), e.Desc())
}
