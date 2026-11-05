package exp_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/stretchr/testify/assert"
)

func TestNewExcludedExpression(t *testing.T) {
	cases := []struct {
		name string
		col  interface{}
		want string
	}{
		{name: "column string", col: "a", want: "a"},
		{name: "qualified string", col: "t.a", want: "a"},
		{name: "schema qualified string", col: "s.t.a", want: "a"},
		{name: "column identifier", col: exp.NewIdentifierExpression("", "", "a"), want: "a"},
		{name: "qualified identifier", col: exp.NewIdentifierExpression("s", "t", "a"), want: "a"},
		{name: "identifier without column", col: exp.NewIdentifierExpression("", "t", nil), want: ""},
		{name: "non string column", col: exp.NewIdentifierExpression("", "", exp.Star()), want: ""},
		{name: "unsupported type", col: 1, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := exp.NewExcludedExpression(tc.col)
			assert.Equal(t, tc.want, e.Col())
			assert.Equal(t, e, e.Expression())
			assert.Equal(t, e, e.Clone())
		})
	}
}

func TestExcludedExpressionOperators(t *testing.T) {
	e := exp.NewExcludedExpression("a")
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
	assert.Equal(t, exp.NewBooleanExpression(exp.IsOp, e, nil), e.Is(nil))
	assert.Equal(t, exp.NewBooleanExpression(exp.IsNotOp, e, nil), e.IsNot(nil))
	assert.Equal(t, exp.NewBooleanExpression(exp.IsOp, e, nil), e.IsNull())
	assert.Equal(t, exp.NewBooleanExpression(exp.IsNotOp, e, nil), e.IsNotNull())
	assert.Equal(t, exp.NewBooleanExpression(exp.IsOp, e, true), e.IsTrue())
	assert.Equal(t, exp.NewBooleanExpression(exp.IsNotOp, e, true), e.IsNotTrue())
	assert.Equal(t, exp.NewBooleanExpression(exp.IsOp, e, false), e.IsFalse())
	assert.Equal(t, exp.NewBooleanExpression(exp.IsNotOp, e, false), e.IsNotFalse())
	assert.Equal(t, exp.NewBooleanExpression(exp.InOp, e, []interface{}{1, 2}), e.In(1, 2))
	assert.Equal(t, exp.NewBooleanExpression(exp.NotInOp, e, []interface{}{1, 2}), e.NotIn(1, 2))
	assert.Equal(t, exp.NewRangeExpression(exp.BetweenOp, e, exp.NewRangeVal(1, 2)), e.Between(exp.NewRangeVal(1, 2)))
	assert.Equal(t, exp.NewRangeExpression(exp.NotBetweenOp, e, exp.NewRangeVal(1, 2)), e.NotBetween(exp.NewRangeVal(1, 2)))
	assert.Equal(t, exp.NewBooleanExpression(exp.LikeOp, e, "x"), e.Like("x"))
	assert.Equal(t, exp.NewBooleanExpression(exp.NotLikeOp, e, "x"), e.NotLike("x"))
	assert.Equal(t, exp.NewBooleanExpression(exp.ILikeOp, e, "x"), e.ILike("x"))
	assert.Equal(t, exp.NewBooleanExpression(exp.NotILikeOp, e, "x"), e.NotILike("x"))
	assert.Equal(t, exp.NewBooleanExpression(exp.RegexpLikeOp, e, "x"), e.RegexpLike("x"))
	assert.Equal(t, exp.NewBooleanExpression(exp.RegexpNotLikeOp, e, "x"), e.RegexpNotLike("x"))
	assert.Equal(t, exp.NewBooleanExpression(exp.RegexpILikeOp, e, "x"), e.RegexpILike("x"))
	assert.Equal(t, exp.NewBooleanExpression(exp.RegexpNotILikeOp, e, "x"), e.RegexpNotILike("x"))
	assert.Equal(t, exp.NewBitwiseExpression(exp.BitwiseInversionOp, nil, e), e.BitwiseInversion())
	assert.Equal(t, exp.NewBitwiseExpression(exp.BitwiseOrOp, e, 1), e.BitwiseOr(1))
	assert.Equal(t, exp.NewBitwiseExpression(exp.BitwiseAndOp, e, 1), e.BitwiseAnd(1))
	assert.Equal(t, exp.NewBitwiseExpression(exp.BitwiseXorOp, e, 1), e.BitwiseXor(1))
	assert.Equal(t, exp.NewBitwiseExpression(exp.BitwiseLeftShiftOp, e, 1), e.BitwiseLeftShift(1))
	assert.Equal(t, exp.NewBitwiseExpression(exp.BitwiseRightShiftOp, e, 1), e.BitwiseRightShift(1))
}
