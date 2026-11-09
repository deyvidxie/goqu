package exp

import (
	"fmt"
	"reflect"

	"github.com/doug-martin/goqu/v9/internal/util"
)

type columnList struct {
	columns []Expression
}

func NewColumnListExpression(subquerys map[string]Aliaseable, tagName string, vals ...interface{}) ColumnListExpression {
	cols := []Expression{}
	for _, val := range vals {
		switch t := val.(type) {
		case nil:
		case string:
			cols = append(cols, ParseIdentifier(t))
		case ColumnListExpression:
			cols = append(cols, t.Columns()...)
		case Expression:
			cols = append(cols, t)
		default:
			structType, valKind := util.GetTypeInfo(val)

			if valKind == reflect.Struct {
				if problem := util.CheckColumnSource(structType, tagName); problem.Issue != util.ColumnMapOK {
					cols = append(cols, newInvalidExpression(errColumnSource(structType, problem, tagName)))
					continue
				}
				cm := util.StructColumnMap(structType, tagName)
				structCols, subqueryKeys := cm.Cols()
				for _, col := range structCols {
					var sc Expression

					if v, found := subqueryKeys[col]; found {
						if q, f := subquerys[v]; f {
							sc = q.As(col)
						} else {
							sc = NewLiteralExpression(v).As(col)
						}
					} else {
						i := ParseIdentifier(col)
						sc = i
						if i.IsQualified() {
							sc = i.As(NewIdentifierExpression("", "", col))
						}
					}
					cols = append(cols, sc)
				}
			} else {
				panic(fmt.Sprintf("Cannot create expression from  %+v", val))
			}
		}
	}
	return columnList{columns: cols}
}

func NewOrderedColumnList(vals ...OrderedExpression) ColumnListExpression {
	exps := make([]interface{}, 0, len(vals))
	for _, col := range vals {
		exps = append(exps, col.Expression())
	}
	return NewColumnListExpression(nil, "db", exps...)
}

func (cl columnList) Clone() Expression {
	newExps := make([]Expression, 0, len(cl.columns))
	for _, exp := range cl.columns {
		newExps = append(newExps, exp.Clone())
	}
	return columnList{columns: newExps}
}

func (cl columnList) Expression() Expression {
	return cl
}

func (cl columnList) IsEmpty() bool {
	return len(cl.columns) == 0
}

func (cl columnList) Columns() []Expression {
	return cl.columns
}

func (cl columnList) Append(cols ...Expression) ColumnListExpression {
	ret := columnList{}
	exps := ret.columns
	exps = append(exps, cl.columns...)
	exps = append(exps, cols...)
	ret.columns = exps
	return ret
}
