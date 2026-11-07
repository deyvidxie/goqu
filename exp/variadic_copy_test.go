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

type idList []int

func TestInCopiesASliceArgument(t *testing.T) {
	ids := []int{1}
	named := idList{1}
	a := exp.NewIdentifierExpression("", "", "a")
	in, notIn, inNamed := a.In(ids), a.NotIn(ids), a.In(named)

	ids[0], named[0] = 2, 2

	assert.Equal(t, []int{1}, in.RHS())
	assert.Equal(t, []int{1}, notIn.RHS())
	assert.Equal(t, idList{1}, inNamed.RHS())
}

func TestClauseSettersCopyTheCallersSlice(t *testing.T) {
	window := func(name string) exp.WindowExpression {
		return exp.NewWindowExpression("db", exp.ParseIdentifier(name), nil, nil, nil)
	}
	ws := []exp.WindowExpression{window("w1")}
	rows := []any{1}
	sc := exp.NewSelectClauses().SetWindows(ws)
	ic := exp.NewInsertClauses().SetRows(rows)

	ws[0], rows[0] = window("w2"), 2

	assert.Equal(t, []exp.WindowExpression{window("w1")}, sc.Windows())
	assert.Equal(t, []any{1}, ic.Rows())
}

func TestSiblingAppendsDoNotShareBackingArrays(t *testing.T) {
	cte := func(name string) exp.CommonTableExpression {
		return exp.NewCommonTableExpression(false, name, exp.NewLiteralExpression("x"))
	}
	compound := func(sql string) exp.CompoundExpression {
		return exp.NewCompoundExpression(exp.UnionCompoundType, newTestAppendableExpression(sql, []any{}))
	}
	window := func(name string) exp.WindowExpression {
		return exp.NewWindowExpression("db", exp.ParseIdentifier(name), nil, nil, nil)
	}
	withSpareCap := func(sc exp.SelectClauses) exp.SelectClauses {
		return sc.CommonTablesAppend(cte("c0")).CommonTablesAppend(cte("c1")).CommonTablesAppend(cte("c2")).
			CompoundsAppend(compound("s0")).CompoundsAppend(compound("s1")).CompoundsAppend(compound("s2")).
			WindowsAppend(window("w0")).WindowsAppend(window("w1")).WindowsAppend(window("w2"))
	}
	base := withSpareCap(exp.NewSelectClauses())
	x := base.CommonTablesAppend(cte("x")).CompoundsAppend(compound("x")).WindowsAppend(window("x"))
	base.CommonTablesAppend(cte("y")).CompoundsAppend(compound("y")).WindowsAppend(window("y"))
	assert.Equal(t, cte("x"), x.CommonTables()[3])
	assert.Equal(t, compound("x"), x.Compounds()[3])
	assert.Equal(t, window("x"), x.Windows()[3])

	for name, clauses := range map[string]func() func(exp.CommonTableExpression) []exp.CommonTableExpression{
		"insert": func() func(exp.CommonTableExpression) []exp.CommonTableExpression {
			b := exp.NewInsertClauses().CommonTablesAppend(cte("c0")).CommonTablesAppend(cte("c1")).CommonTablesAppend(cte("c2"))
			return func(c exp.CommonTableExpression) []exp.CommonTableExpression {
				return b.CommonTablesAppend(c).CommonTables()
			}
		},
		"update": func() func(exp.CommonTableExpression) []exp.CommonTableExpression {
			b := exp.NewUpdateClauses().CommonTablesAppend(cte("c0")).CommonTablesAppend(cte("c1")).CommonTablesAppend(cte("c2"))
			return func(c exp.CommonTableExpression) []exp.CommonTableExpression {
				return b.CommonTablesAppend(c).CommonTables()
			}
		},
		"delete": func() func(exp.CommonTableExpression) []exp.CommonTableExpression {
			b := exp.NewDeleteClauses().CommonTablesAppend(cte("c0")).CommonTablesAppend(cte("c1")).CommonTablesAppend(cte("c2"))
			return func(c exp.CommonTableExpression) []exp.CommonTableExpression {
				return b.CommonTablesAppend(c).CommonTables()
			}
		},
	} {
		appendTo := clauses()
		ctes := appendTo(cte("x"))
		appendTo(cte("y"))
		assert.Equal(t, cte("x"), ctes[3], name)
	}

	caseBase := exp.NewCaseExpression().When(1, "a").When(2, "b").When(3, "c")
	caseX := caseBase.When(4, "x")
	caseBase.When(4, "y")
	assert.Equal(t, exp.NewCaseWhen(4, "x"), caseX.GetWhens()[3])
}

func TestSQLFunctionCloneDoesNotShareArgs(t *testing.T) {
	fn := exp.NewSQLFunctionExpression("f", 1)
	clone := fn.Clone().(exp.SQLFunctionExpression)
	fn.Args()[0] = 2
	assert.Equal(t, []any{1}, clone.Args())
}

func TestValsCopyEachRow(t *testing.T) {
	row := []any{1}
	set := exp.NewInsertClauses().SetVals([][]any{row})
	appended := exp.NewInsertClauses().ValsAppend([][]any{row})

	row[0] = 2

	assert.Equal(t, [][]any{{1}}, set.Vals())
	assert.Equal(t, [][]any{{1}}, appended.Vals())
}
