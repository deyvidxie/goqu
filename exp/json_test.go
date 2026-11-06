package exp_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/stretchr/testify/assert"
)

func TestNewJSONExpressions(t *testing.T) {
	col := exp.NewIdentifierExpression("", "t", "doc")
	for _, tc := range []struct {
		name      string
		e         exp.JSONExpression
		function  exp.JSONFunction
		col       interface{}
		candidate interface{}
		paths     []string
	}{
		{"extract string column", exp.NewJSONExtractExpression("t.doc", "$.a"), exp.JSONExtractFunction, col, nil, []string{"$.a"}},
		{"value expression column", exp.NewJSONValueExpression(col, "$.a"), exp.JSONValueFunction, col, nil, []string{"$.a"}},
		{"contains without path", exp.NewJSONContainsExpression("t.doc", 1), exp.JSONContainsFunction, col, 1, nil},
		{"contains with path", exp.NewJSONContainsExpression("t.doc", "x", "$.a"), exp.JSONContainsFunction, col, "x", []string{"$.a"}},
		{"non string column", exp.NewJSONExtractExpression(1, "$"), exp.JSONExtractFunction, 1, nil, []string{"$"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.function, tc.e.Function())
			assert.Equal(t, tc.col, tc.e.Col())
			assert.Equal(t, tc.candidate, tc.e.Candidate())
			assert.Equal(t, tc.paths, tc.e.Paths())
			assert.Equal(t, tc.e, tc.e.Expression())
			assert.Equal(t, tc.e, tc.e.Clone())
		})
	}
}

func TestJSONExpressionOperators(t *testing.T) {
	e := exp.NewJSONValueExpression("doc", "$.a")
	assert.Equal(t, exp.NewAliasExpression(e, "a"), e.As("a"))
	assert.Equal(t, exp.NewBooleanExpression(exp.EqOp, e, 1), e.Eq(1))
	assert.Equal(t, exp.NewBooleanExpression(exp.NeqOp, e, 1), e.Neq(1))
	assert.Equal(t, exp.NewBooleanExpression(exp.GtOp, e, 1), e.Gt(1))
	assert.Equal(t, exp.NewBooleanExpression(exp.GteOp, e, 1), e.Gte(1))
	assert.Equal(t, exp.NewBooleanExpression(exp.LtOp, e, 1), e.Lt(1))
	assert.Equal(t, exp.NewBooleanExpression(exp.LteOp, e, 1), e.Lte(1))
	assert.Equal(t, exp.NewBooleanExpression(exp.IsOp, e, nil), e.Is(nil))
	assert.Equal(t, exp.NewBooleanExpression(exp.IsNotOp, e, nil), e.IsNot(nil))
	assert.Equal(t, exp.NewBooleanExpression(exp.IsOp, e, nil), e.IsNull())
	assert.Equal(t, exp.NewBooleanExpression(exp.IsNotOp, e, nil), e.IsNotNull())
	assert.Equal(t, exp.NewBooleanExpression(exp.IsOp, e, true), e.IsTrue())
	assert.Equal(t, exp.NewBooleanExpression(exp.IsNotOp, e, true), e.IsNotTrue())
	assert.Equal(t, exp.NewBooleanExpression(exp.IsOp, e, false), e.IsFalse())
	assert.Equal(t, exp.NewBooleanExpression(exp.IsNotOp, e, false), e.IsNotFalse())
	assert.Equal(t, exp.NewBooleanExpression(exp.InOp, e, []any{1, 2}), e.In(1, 2))
	assert.Equal(t, exp.NewBooleanExpression(exp.NotInOp, e, []any{1, 2}), e.NotIn(1, 2))
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
	assert.Equal(t, exp.NewOrderedExpression(e, exp.AscDir, exp.NoNullsSortType), e.Asc())
	assert.Equal(t, exp.NewOrderedExpression(e, exp.DescSortDir, exp.NoNullsSortType), e.Desc())
}
