package exp_test

import (
	"testing"

	"github.com/doug-martin/goqu/v9/exp"
	"github.com/stretchr/testify/assert"
)

func TestConstructorsCopyVariadicArgs(t *testing.T) {
	paths := []string{"$.a"}
	of := []exp.IdentifierExpression{exp.ParseIdentifier("t")}
	args := []any{1}
	contains := exp.NewJSONContainsExpression("doc", 1, paths...)
	lock := exp.NewLock(exp.ForUpdate, exp.Wait, of...)
	fn := exp.NewSQLFunctionExpression("f", args...)
	lit := exp.NewLiteralExpression("?", args...)
	in := exp.NewIdentifierExpression("", "", "a").In(args...)

	paths[0], of[0], args[0] = "$.b", exp.ParseIdentifier("u"), 2

	assert.Equal(t, []string{"$.a"}, contains.Paths())
	assert.Equal(t, []exp.IdentifierExpression{exp.ParseIdentifier("t")}, lock.Of())
	assert.Equal(t, []any{1}, fn.Args())
	assert.Equal(t, []any{1}, lit.Args())
	assert.Equal(t, []any{1}, in.RHS())
}
