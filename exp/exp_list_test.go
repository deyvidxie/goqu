package exp_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/stretchr/testify/assert"
)

func TestNewExpressionList_copiesInput(t *testing.T) {
	a := exp.NewIdentifierExpression("", "", "a").Eq(1)
	in := []exp.Expression{a}
	el := exp.NewExpressionList(exp.AndType, in...)
	in[0] = exp.NewIdentifierExpression("", "", "b").Eq(2)
	assert.Equal(t, a, el.Expressions()[0])
}
